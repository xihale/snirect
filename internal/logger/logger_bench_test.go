package logger

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

func BenchmarkLogger_DisabledDebug(b *testing.B) {
	SetLevel("INFO")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Upstream().Debug("tls handshake", "host", "www.google.com", "alpn", "h2")
	}
}

func BenchmarkLogger_EnabledInfo(b *testing.B) {
	ch := &consoleHandler{w: io.Discard}
	mh := &MultiHandler{
		console: ch,
		level:   logLevel,
	}
	l := slog.New(mh).With("side", string(SideUpstream))
	ctx := context.Background()
	SetLevel("INFO")

	b.Run("consoleHandler", func(b *testing.B) {
		rec := slog.NewRecord(time.Unix(1700000000, 0), slog.LevelInfo, "connected upstream", 0)
		rec.AddAttrs(
			slog.String("side", "upstream"),
			slog.String("host", "www.google.com"),
			slog.String("ip", "64.233.160.142"),
			slog.Int("port", 443),
			slog.Duration("rtt", 12*time.Millisecond),
		)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := ch.Handle(ctx, rec); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("MultiHandler_WithSide", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info("connected upstream", "host", "www.google.com", "ip", "64.233.160.142", "port", 443)
		}
	})
}
