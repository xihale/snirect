package core

import (
	"context"
	"strings"
	"sync"

	"log/slog"

	"github.com/xihale/snirect/logger"
)

// This file bridges snirect-core's slog logger into the Android UI. Before this
// bridge, every log line from core/tun (upstream dial failures, TLS handshake
// errors) and core/dns (resolver failures) went to stdout only — the Android
// log page saw nothing, so the symptom "logs look fine but no traffic" was
// actually "logs were invisible, so the real error was never shown".
//
// installMobileSink registers a slog.Handler that formats each record
// ("[upstream] dial failed host=example.com error=...") and forwards it through
// the existing logChan → OnStatusChanged path, so core errors appear in the
// same log page as the "CORE: Starting..." lines.

var installOnce sync.Once

// installMobileSink wires snirect-core's logger into the Android log page. It
// is idempotent (sync.Once) and safe to call on every StartEngine. The sink
// respects the mobile log level (SetLogLevel) because logf() drops records
// below currentLogLevel before they reach logChan.
func installMobileSink() {
	installOnce.Do(func() {
		logger.InstallSink(&mobileHandler{})
	})
}

// mobileHandler is a slog.Handler that formats core records and routes them
// through the existing LogInfo/LogWarn/LogError/LogDebug funnel. We split on
// level so the mobile UI's [ERROR]/[WARN] highlighting (which keys off the
// prefix) still works.
type mobileHandler struct{}

func (h *mobileHandler) Enabled(_ context.Context, lvl slog.Level) bool {
	// Map the slog level onto the mobile LogLevel enum (Debug=0..Error=3) and
	// let logf() re-check; this just skips formatting cost for dropped levels.
	// The level read goes through levelEnabled's lock (audit L3 data race).
	var mobileLevel LogLevel
	switch {
	case lvl >= slog.LevelError:
		mobileLevel = LevelError
	case lvl >= slog.LevelWarn:
		mobileLevel = LevelWarn
	case lvl >= slog.LevelInfo:
		mobileLevel = LevelInfo
	default:
		mobileLevel = LevelDebug
	}
	return levelEnabled(mobileLevel)
}

func (h *mobileHandler) Handle(_ context.Context, r slog.Record) error {
	// Pull side + host out of the attrs for a readable prefix, matching the
	// console handler's "[side] host message" style.
	var side, host, process string
	var attrs []string
	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "side":
			side = a.Value.String()
		case "host", "target":
			if host == "" {
				host = a.Value.String()
			} else {
				attrs = append(attrs, a.Key+"="+a.Value.String())
			}
		case "process":
			process = a.Value.String()
		default:
			attrs = append(attrs, a.Key+"="+a.Value.String())
		}
		return true
	})

	var b strings.Builder
	if side != "" {
		b.WriteByte('[')
		b.WriteString(side)
		b.WriteString("] ")
	}
	if process != "" {
		b.WriteByte('[')
		b.WriteString(process)
		b.WriteString("] ")
	}
	b.WriteString(r.Message)
	if host != "" {
		b.WriteString(" host=")
		b.WriteString(host)
	}
	for _, a := range attrs {
		b.WriteByte(' ')
		b.WriteString(a)
	}
	msg := b.String()

	// Route by level so the Kotlin side's prefix-based coloring works.
	switch {
	case r.Level >= slog.LevelError:
		logf(LevelError, "%s", msg)
	case r.Level >= slog.LevelWarn:
		logf(LevelWarn, "%s", msg)
	case r.Level >= slog.LevelInfo:
		logf(LevelInfo, "%s", msg)
	default:
		logf(LevelDebug, "%s", msg)
	}
	return nil
}

func (h *mobileHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *mobileHandler) WithGroup(name string) slog.Handler       { return h }
