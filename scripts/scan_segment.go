package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	SCAN_RANGE = "142.250.72.0/24"
	TEST_HOST  = "www.youtube.com"
	TIMEOUT    = 2 * time.Second
	THREADS    = 50
)

func main() {
	_, ipNet, _ := net.ParseCIDR(SCAN_RANGE)
	ips := enqueueIPs(ipNet)

	fmt.Printf("Scanning range %s for %s...\n", SCAN_RANGE, TEST_HOST)
	fmt.Printf("%-20s | %-15s | %s\n", "IP Address", "Latency", "Status")
	fmt.Println("------------------------------------------------------------")

	var wg sync.WaitGroup
	sem := make(chan struct{}, THREADS)

	for _, ip := range ips {
		wg.Add(1)
		go func(targetIP string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			start := time.Now()
			dialer := &net.Dialer{Timeout: TIMEOUT}
			conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(targetIP, "443"), &tls.Config{
				ServerName:         TEST_HOST,
				InsecureSkipVerify: true,
			})

			if err == nil {
				conn.Close()
				fmt.Printf("%-20s | %-15s | ✅ REACHABLE\n", targetIP, time.Since(start))
			}
		}(ip)
	}

	wg.Wait()
}

func enqueueIPs(ipNet *net.IPNet) []string {
	var ips []string
	for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); inc(ip) {
		ips = append(ips, ip.String())
	}
	// remove network and broadcast
	if len(ips) > 2 {
		return ips[1 : len(ips)-1]
	}
	return ips
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
