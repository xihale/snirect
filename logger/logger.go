package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/xihale/snirect/term/color"
)

var (
	currentLogger *slog.Logger
	logLevel      *slog.LevelVar
	mu            sync.RWMutex
	logFile       *os.File
)

// Side names the part of the system a log line concerns. It is rendered as
// `[side]` in the console so a reader can tell at a glance whether a failure is
// on the client side (the browser talking to us), the upstream side (us talking
// to the remote site), DNS, a system operation (cert/install/proxy), or control
// (daemon lifecycle).
type Side string

const (
	SideClient   Side = "client"
	SideUpstream Side = "upstream"
	SideDNS      Side = "dns"
	SideSystem   Side = "system"
	SideControl  Side = "control"
)

func init() {
	logLevel = &slog.LevelVar{}
	logLevel.Set(slog.LevelInfo)
	if err := updateLogger(""); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
	}
}

func updateLogger(path string) error {
	consoleWriter := &consoleHandler{
		w: os.Stdout,
	}

	var newFile *os.File
	if path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("create log directory: %w", err)
		}

		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("open log file: %w", err)
		}
		newFile = f
	}

	handler := &MultiHandler{
		console: consoleWriter,
		level:   logLevel,
	}
	if newFile != nil {
		handler.file = slog.NewTextHandler(newFile, &slog.HandlerOptions{
			Level: logLevel,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey {
					return slog.String(a.Key, a.Value.Time().Format("2006/01/02 15:04:05"))
				}
				return a
			},
		})
	}

	newLogger := slog.New(handler)

	mu.Lock()
	oldFile := logFile
	currentLogger = newLogger
	logFile = newFile
	mu.Unlock()

	if oldFile != nil && oldFile != newFile {
		_ = oldFile.Close()
	}

	if newFile != nil {
		absPath, _ := filepath.Abs(path)
		newLogger.Info("Logging to file", "path", absPath)
	}

	return nil
}

// MultiHandler dispatches to console (custom, human-readable) and file
// (standard text, complete). The file handler never drops a record, so the
// full log is always available on disk.
//
// sinks are optional extra handlers installed by embedders (e.g. mobile-core
// forwards records to the Android UI via InstallSink). They run after the file
// handler and in parallel with the console handler.
type MultiHandler struct {
	console *consoleHandler
	file    slog.Handler
	level   slog.Leveler
	sinksMu sync.RWMutex
	sinks   []slog.Handler
}

func (h *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
	// File gets every record, unchanged.
	if h.file != nil {
		if err := h.file.Handle(ctx, r); err != nil {
			return err
		}
	}
	// Sinks (e.g. mobile UI forwarder) get every record too, after the file.
	h.sinksMu.RLock()
	sinks := h.sinks
	h.sinksMu.RUnlock()
	for _, s := range sinks {
		if err := s.Handle(ctx, r.Clone()); err != nil {
			return err
		}
	}
	return h.console.Handle(ctx, r)
}

func (h *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newH := &MultiHandler{
		console: h.console.WithAttrs(attrs).(*consoleHandler),
		level:   h.level,
	}
	if h.file != nil {
		newH.file = h.file.WithAttrs(attrs)
	}
	// Sinks see attrs too so a per-side logger (WithAttrs("side", ...)) still
	// tags records delivered to mobile.
	newH.sinksMu.Lock()
	newH.sinks = make([]slog.Handler, len(h.sinks))
	for i, s := range h.sinks {
		newH.sinks[i] = s.WithAttrs(attrs)
	}
	newH.sinksMu.Unlock()
	return newH
}

func (h *MultiHandler) WithGroup(name string) slog.Handler {
	newH := &MultiHandler{
		console: h.console.WithGroup(name).(*consoleHandler),
		level:   h.level,
	}
	if h.file != nil {
		newH.file = h.file.WithGroup(name)
	}
	newH.sinksMu.Lock()
	newH.sinks = make([]slog.Handler, len(h.sinks))
	for i, s := range h.sinks {
		newH.sinks[i] = s.WithGroup(name)
	}
	newH.sinksMu.Unlock()
	return newH
}

// InstallSink attaches an extra handler to the live logger's MultiHandler. It
// is how mobile-core bridges core/proxy + core/dns records into the Android UI:
// register a handler that formats each record and forwards it to LogInfo.
// Returns false if the active handler is not a MultiHandler.
//
// Must be called after the package init (which sets up the MultiHandler), i.e.
// from an exported function or explicit Init — early package init order is not
// guaranteed across modules.
func InstallSink(sink slog.Handler) bool {
	mu.Lock()
	defer mu.Unlock()
	if currentLogger == nil {
		return false
	}
	mh, ok := currentLogger.Handler().(*MultiHandler)
	if !ok {
		return false
	}
	mh.sinksMu.Lock()
	mh.sinks = append(mh.sinks, sink)
	mh.sinksMu.Unlock()
	return true
}

// consoleHandler formats logs for human readability:
//
//	[time] [LEVEL] [side] host  message  attr=value …
//
// where [side] and host are pulled out of the record attrs when present, so the
// reader can immediately see which site and which side of the connection a line
// concerns.
type consoleHandler struct {
	w      io.Writer
	attrs  []slog.Attr
	groups []string
}

func (h *consoleHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= logLevel.Level()
}

func (h *consoleHandler) Handle(ctx context.Context, r slog.Record) error {
	// Collect attrs (handler-held + record-held).
	all := append([]slog.Attr{}, h.attrs...)
	r.Attrs(func(a slog.Attr) bool {
		all = append(all, a)
		return true
	})

	// Pull side, process, and host out of the attr set for prefix rendering.
	// Build `rest` as a fresh slice (not an alias) so the dropped attrs don't
	// leak through. `process` is only present in TUN mode (per-connection
	// attribution); rendering it between side and host keeps a TUN log line
	// scannable: [client] [telegram] api.telegram.org ...
	var side string
	var process string
	var host string
	rest := make([]slog.Attr, 0, len(all))
	for _, a := range all {
		switch a.Key {
		case "side":
			side = a.Value.String()
			continue
		case "process":
			process = a.Value.String()
			continue
		case "host":
			host = a.Value.String()
			continue
		case "target":
			// Some DNS lines use "target" instead of "host".
			if host == "" {
				host = a.Value.String()
				continue
			}
		}
		rest = append(rest, a)
	}

	level := r.Level.String()
	msg := r.Message
	timeStr := r.Time.Format("15:04:05")

	var levelColor, reset, timeColor string
	reset = color.If(color.Reset)
	timeColor = color.If(color.Gray)
	switch r.Level {
	case slog.LevelDebug:
		levelColor = color.If(color.Gray)
	case slog.LevelInfo:
		levelColor = color.If(color.Green)
	case slog.LevelWarn:
		levelColor = color.If(color.Yellow)
	case slog.LevelError:
		levelColor = color.If(color.Red)
	}

	attrText := formatAttrs(h.groups, rest)
	if attrText != "" {
		msg = msg + " " + attrText
	}

	// Build the prefix: `[side] [process] host`. Each segment is optional — a
	// control/lifecycle line often has none; a TUN-mode client line has all
	// three. Render only the present ones so we never print placeholders.
	prefix := ""
	if side != "" {
		prefix += fmt.Sprintf(" [%s%s%s]", color.If(color.Cyan), side, reset)
	}
	if process != "" {
		prefix += fmt.Sprintf(" [%s%s%s]", color.If(color.Gray), process, reset)
	}
	if host != "" {
		prefix += fmt.Sprintf(" %s%s%s", color.If(color.Blue), host, reset)
	}

	_, err := fmt.Fprintf(h.w, "%s[%s]%s [%s%s%s]%s %s\n",
		timeColor, timeStr, reset,
		levelColor, level, reset,
		prefix,
		msg)
	return err
}

func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &clone
}

func (h *consoleHandler) WithGroup(name string) slog.Handler {
	clone := *h
	clone.groups = append(append([]string{}, h.groups...), name)
	return &clone
}

// Write implements io.Writer to support standard logger compatibility if needed
func (h *consoleHandler) Write(p []byte) (n int, err error) {
	return h.w.Write(p)
}

func formatAttrs(groups []string, attrs []slog.Attr) string {
	if len(attrs) == 0 {
		return ""
	}

	parts := make([]string, 0, len(attrs))
	for _, attr := range attrs {
		parts = append(parts, formatAttr(groups, attr))
	}
	return strings.Join(parts, " ")
}

func formatAttr(groups []string, attr slog.Attr) string {
	if attr.Equal(slog.Attr{}) {
		return ""
	}

	resolved := attr.Value.Resolve()
	key := attr.Key
	if len(groups) > 0 {
		key = strings.Join(append(append([]string{}, groups...), key), ".")
	}

	switch resolved.Kind() {
	case slog.KindGroup:
		groupParts := make([]string, 0, len(resolved.Group()))
		for _, child := range resolved.Group() {
			groupParts = append(groupParts, formatAttr(append(groups, key), child))
		}
		return strings.Join(groupParts, " ")
	case slog.KindString:
		return fmt.Sprintf("%s=%q", key, resolved.String())
	case slog.KindDuration:
		return fmt.Sprintf("%s=%s", key, resolved.Duration())
	case slog.KindTime:
		return fmt.Sprintf("%s=%s", key, resolved.Time().Format(time.RFC3339))
	case slog.KindBool:
		return fmt.Sprintf("%s=%t", key, resolved.Bool())
	case slog.KindInt64:
		return fmt.Sprintf("%s=%d", key, resolved.Int64())
	case slog.KindUint64:
		return fmt.Sprintf("%s=%d", key, resolved.Uint64())
	case slog.KindFloat64:
		return fmt.Sprintf("%s=%g", key, resolved.Float64())
	case slog.KindAny:
		return fmt.Sprintf("%s=%v", key, resolved.Any())
	default:
		return fmt.Sprintf("%s=%v", key, resolved.Any())
	}
}

// Default returns the current logger. Re-read on every call so a runtime
// SetOutput reconfiguration is picked up — callers that capture
// logger.Default() at package init would otherwise pin a stale handler.
func Default() *slog.Logger {
	mu.RLock()
	logger := currentLogger
	mu.RUnlock()
	if logger != nil {
		return logger
	}
	return slog.Default()
}

func With(args ...any) *slog.Logger {
	return Default().With(args...)
}

// Per-side accessors resolve Default() on each call (see Default's note), then
// attach the `side` attr so the console handler renders `[side]`.

// Client logs failures on the client side (the browser/app talking to us), e.g.
// the client TLS handshake, leaf-cert signing.
func Client() *slog.Logger { return For(SideClient) }

// Upstream logs failures on the upstream side (us talking to the remote site),
// e.g. remote dial, upstream TLS handshake, certificate rejection.
func Upstream() *slog.Logger { return For(SideUpstream) }

// DNS logs resolver operations and failures.
func DNS() *slog.Logger { return For(SideDNS) }

// System logs OS-level operations: cert store, service install, config, proxy.
func System() *slog.Logger { return For(SideSystem) }

// Control logs daemon lifecycle.
func Control() *slog.Logger { return For(SideControl) }

// For returns the current logger tagged with the given side.
func For(side Side) *slog.Logger {
	return Default().With("side", string(side))
}

func SetLevel(l string) {
	switch strings.ToUpper(strings.TrimSpace(l)) {
	case "DEBUG":
		logLevel.Set(slog.LevelDebug)
	case "INFO", "":
		logLevel.Set(slog.LevelInfo)
	case "WARN", "WARNING":
		logLevel.Set(slog.LevelWarn)
	case "ERROR":
		logLevel.Set(slog.LevelError)
	default:
		logLevel.Set(slog.LevelInfo)
	}
}

func SetOutput(path string) error {
	return updateLogger(path)
}
