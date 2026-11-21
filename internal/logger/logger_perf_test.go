package logger

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

type recordingSink struct {
	minLevel slog.Level
	calls    atomic.Int64
	lastSide atomic.Value
}

func (s *recordingSink) Enabled(_ context.Context, level slog.Level) bool {
	return level >= s.minLevel
}

func (s *recordingSink) Handle(_ context.Context, r slog.Record) error {
	s.calls.Add(1)
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "side" {
			s.lastSide.Store(a.Value.String())
		}
		return true
	})
	return nil
}

func (s *recordingSink) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Keep side tracking simple for test: store side if provided on WithAttrs.
	clone := &recordingSink{minLevel: s.minLevel}
	for _, a := range attrs {
		if a.Key == "side" {
			clone.lastSide.Store(a.Value.String())
		}
	}
	return &wrappedRecordingSink{parent: s, side: clone.lastSide}
}

func (s *recordingSink) WithGroup(string) slog.Handler { return s }

type wrappedRecordingSink struct {
	parent *recordingSink
	side   atomic.Value
}

func (w *wrappedRecordingSink) Enabled(ctx context.Context, level slog.Level) bool {
	return w.parent.Enabled(ctx, level)
}

func (w *wrappedRecordingSink) Handle(_ context.Context, r slog.Record) error {
	w.parent.calls.Add(1)
	if v := w.side.Load(); v != nil {
		w.parent.lastSide.Store(v)
	}
	return nil
}

func (w *wrappedRecordingSink) WithAttrs([]slog.Attr) slog.Handler { return w }
func (w *wrappedRecordingSink) WithGroup(string) slog.Handler      { return w }

func TestConsoleHandler_Formatting(t *testing.T) {
	var out bytes.Buffer
	ch := &consoleHandler{w: &out, wMu: &sync.Mutex{}}

	rec := slog.NewRecord(time.Date(2026, 11, 21, 9, 8, 7, 0, time.UTC), slog.LevelInfo, "connected", 0)
	rec.AddAttrs(
		slog.String("side", "upstream"),
		slog.String("process", "curl"),
		slog.String("target", "example.com"),
		slog.String("ip", "1.2.3.4"),
		slog.Int("port", 443),
		slog.Bool("tls", true),
		slog.Duration("rtt", 15*time.Millisecond),
		slog.Group("meta", slog.Uint64("id", 7), slog.Float64("ratio", 1.5)),
	)

	h := ch.WithGroup("conn")
	if err := h.Handle(context.Background(), rec); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	got := ansiRE.ReplaceAllString(out.String(), "")
	want := "[09:08:07] [INFO] [upstream] [curl] example.com connected conn.ip=\"1.2.3.4\" conn.port=443 conn.tls=true conn.rtt=15ms conn.meta.id=7 conn.meta.ratio=1.5\n"
	if got != want {
		t.Fatalf("formatted output:\n got: %q\nwant: %q", got, want)
	}
}

func TestMultiHandler_SinkEnabledGating(t *testing.T) {
	sink := &recordingSink{minLevel: slog.LevelWarn}
	mh := &MultiHandler{
		console: &consoleHandler{w: &bytes.Buffer{}, wMu: &sync.Mutex{}},
		level:   logLevel,
		sinks:   []slog.Handler{sink},
	}
	SetLevel("INFO")
	t.Cleanup(func() { SetLevel("INFO") })

	l := slog.New(mh).With("side", string(SideClient))
	l.Info("ignored by sink")
	if got := sink.calls.Load(); got != 0 {
		t.Fatalf("sink Handle called %d times on Info when sink minLevel is Warn", got)
	}

	l.Warn("delivered to sink")
	if got := sink.calls.Load(); got != 1 {
		t.Fatalf("sink Handle called %d times on Warn, want 1", got)
	}
	if side, _ := sink.lastSide.Load().(string); side != "client" {
		t.Fatalf("sink lastSide = %q, want %q", side, "client")
	}
}

func TestLogger_ConcurrentSetLevelInstallSinkAndSideCalls(t *testing.T) {
	// Redirect console output inside snapshot to bytes.Buffer/Discard? Instead keep
	// SetOutput("") on cleanup and test at INFO/WARN so stdout stays quiet.
	t.Cleanup(func() {
		SetLevel("INFO")
		_ = updateLogger("")
	})

	// Replace console writer with discard during concurrency test so we don't spam stdout.
	mu.Lock()
	snap := currentSnapshot.Load()
	mh := &MultiHandler{
		console: &consoleHandler{w: io.Discard, wMu: &sync.Mutex{}},
		level:   snap.handler.level,
	}
	currentSnapshot.Store(buildSnapshot(mh))
	mu.Unlock()

	sink := &recordingSink{minLevel: slog.LevelError}
	if !InstallSink(sink) {
		t.Fatal("InstallSink returned false")
	}

	const goroutines = 10
	const iters = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				switch i % 4 {
				case 0:
					SetLevel("INFO")
				case 1:
					SetLevel("WARN")
				case 2:
					InstallSink(&recordingSink{minLevel: slog.LevelError})
				case 3:
					SetLevel("DEBUG")
				}
				Client().Debug("dbg", "host", "a.com")
				Upstream().Info("info", "host", "b.com", "port", 443)
				DNS().Warn("warn", "target", "c.com")
				System().Debug("sys")
				Control().Debug("ctrl")
				For(Side("custom")).Debug("custom")
				_ = Default()
			}
		}(g)
	}
	wg.Wait()
}
