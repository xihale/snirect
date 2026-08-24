package proxy

import (
	"context"
	"net"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// dialOutbound must behave identically to the historical code path when no
// dialer is injected (desktop). This guards the Step-1 refactor against
// regressing the desktop proxy: nil outboundDialer must dial the configured
// target on the loopback listener within the timeout.
func TestDialOutbound_Default(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		accepted <- c
	}()

	s := &ProxyServer{} // outboundDialer == nil -> default behavior
	conn, err := s.dialOutbound(context.Background(), ln.Addr().String(), 5*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	srv := <-accepted
	if srv == nil {
		t.Fatal("server side did not accept")
	}
	defer srv.Close()
}

// TestDialOutbound_Injected proves the injected dialer (and its Control hook,
// which on Android carries VpnService.protect) is actually used for the dial.
// A sentinel Control callback flips a counter; if the injected dialer were
// ignored, the counter stays 0.
func TestDialOutbound_Injected(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		accepted <- c
	}()

	var controls int32
	injected := &net.Dialer{
		Timeout: 5 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			atomic.AddInt32(&controls, 1)
			return nil
		},
	}

	s := &ProxyServer{outboundDialer: injected}
	conn, err := s.dialOutbound(context.Background(), ln.Addr().String(), 0)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	srv := <-accepted
	if srv == nil {
		t.Fatal("server side did not accept")
	}
	defer srv.Close()

	if atomic.LoadInt32(&controls) == 0 {
		t.Fatal("injected dialer Control hook was not invoked; dialOutbound ignored outboundDialer")
	}
}
