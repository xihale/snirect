package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xihale/snirect/internal/color"
)

type loggerSnapshot struct {
	base     *slog.Logger
	client   *slog.Logger
	upstream *slog.Logger
	dns      *slog.Logger
	system   *slog.Logger
	control  *slog.Logger
	handler  *MultiHandler
}

var (
	currentSnapshot atomic.Pointer[loggerSnapshot]
	logLevel        *slog.LevelVar
	mu              sync.Mutex
	logFile         *os.File
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

func buildSnapshot(mh *MultiHandler) *loggerSnapshot {
	base := slog.New(mh)
	return &loggerSnapshot{
		base:     base,
		client:   base.With("side", string(SideClient)),
		upstream: base.With("side", string(SideUpstream)),
		dns:      base.With("side", string(SideDNS)),
		system:   base.With("side", string(SideSystem)),
		control:  base.With("side", string(SideControl)),
		handler:  mh,
	}
}

func updateLogger(path string) error {
	consoleWriter := &consoleHandler{
		w:   os.Stdout,
		wMu: &sync.Mutex{},
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

	snap := buildSnapshot(handler)

	mu.Lock()
	oldFile := logFile
	logFile = newFile
	currentSnapshot.Store(snap)
	mu.Unlock()

	if oldFile != nil && oldFile != newFile {
		_ = oldFile.Close()
	}

	if newFile != nil {
		absPath, _ := filepath.Abs(path)
		snap.base.Info("Logging to file", "path", absPath)
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
	// File gets every enabled record, unchanged.
	if h.file != nil && h.file.Enabled(ctx, r.Level) {
		if err := h.file.Handle(ctx, r); err != nil {
			return err
		}
	}
	// Sinks (e.g. mobile UI forwarder) get every enabled record too, after the file.
	h.sinksMu.RLock()
	sinks := h.sinks
	h.sinksMu.RUnlock()
	for _, s := range sinks {
		if !s.Enabled(ctx, r.Level) {
			continue
		}
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
	h.sinksMu.RLock()
	srcSinks := h.sinks
	h.sinksMu.RUnlock()
	if len(srcSinks) > 0 {
		newSinks := make([]slog.Handler, len(srcSinks))
		for i, s := range srcSinks {
			newSinks[i] = s.WithAttrs(attrs)
		}
		newH.sinks = newSinks
	}
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
	h.sinksMu.RLock()
	srcSinks := h.sinks
	h.sinksMu.RUnlock()
	if len(srcSinks) > 0 {
		newSinks := make([]slog.Handler, len(srcSinks))
		for i, s := range srcSinks {
			newSinks[i] = s.WithGroup(name)
		}
		newH.sinks = newSinks
	}
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
	if sink == nil {
		return false
	}
	mu.Lock()
	defer mu.Unlock()
	snap := currentSnapshot.Load()
	if snap == nil || snap.handler == nil {
		return false
	}
	mh := snap.handler
	mh.sinksMu.Lock()
	nextSinks := make([]slog.Handler, len(mh.sinks)+1)
	copy(nextSinks, mh.sinks)
	nextSinks[len(mh.sinks)] = sink
	mh.sinks = nextSinks
	mh.sinksMu.Unlock()

	// Rebuild the cached per-side loggers so they carry sink.WithAttrs("side", ...).
	currentSnapshot.Store(buildSnapshot(mh))
	return true
}

type consoleBuf struct {
	buf   []byte
	attrs []slog.Attr
}

var consoleBufPool = sync.Pool{
	New: func() any {
		return &consoleBuf{
			buf:   make([]byte, 0, 512),
			attrs: make([]slog.Attr, 0, 16),
		}
	},
}

// consoleHandler formats logs for human readability:
//
//	[time] [LEVEL] [side] host  message  attr=value …
//
// where [side] and host are pulled out of the record attrs when present, so the
// reader can immediately see which site and which side of the connection a line
// concerns.
type consoleHandler struct {
	w           io.Writer
	wMu         *sync.Mutex
	attrs       []slog.Attr
	groups      []string
	groupPrefix string
}

func (h *consoleHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= logLevel.Level()
}

func (h *consoleHandler) Handle(ctx context.Context, r slog.Record) error {
	cb := consoleBufPool.Get().(*consoleBuf)
	cb.buf = cb.buf[:0]
	cb.attrs = cb.attrs[:0]

	var side, process, host string
	collectAttr := func(a slog.Attr) {
		switch a.Key {
		case "side":
			side = a.Value.String()
			return
		case "process":
			process = a.Value.String()
			return
		case "host":
			host = a.Value.String()
			return
		case "target":
			// Some DNS lines use "target" instead of "host".
			if host == "" {
				host = a.Value.String()
				return
			}
		}
		cb.attrs = append(cb.attrs, a)
	}

	for _, a := range h.attrs {
		collectAttr(a)
	}
	r.Attrs(func(a slog.Attr) bool {
		collectAttr(a)
		return true
	})

	reset := color.If(color.Reset)
	timeColor := color.If(color.Gray)
	var levelColor string
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

	buf := cb.buf
	// [time]
	buf = append(buf, timeColor...)
	buf = append(buf, '[')
	buf = r.Time.AppendFormat(buf, "15:04:05")
	buf = append(buf, ']')
	buf = append(buf, reset...)
	// [LEVEL]
	buf = append(buf, " ["...)
	buf = append(buf, levelColor...)
	buf = append(buf, r.Level.String()...)
	buf = append(buf, reset...)
	buf = append(buf, ']')

	// Build the prefix: `[side] [process] host`. Each segment is optional — a
	// control/lifecycle line often has none; a TUN-mode client line has all
	// three. Render only the present ones so we never print placeholders.
	if side != "" {
		buf = append(buf, " ["...)
		buf = append(buf, color.If(color.Cyan)...)
		buf = append(buf, side...)
		buf = append(buf, reset...)
		buf = append(buf, ']')
	}
	if process != "" {
		buf = append(buf, " ["...)
		buf = append(buf, color.If(color.Gray)...)
		buf = append(buf, process...)
		buf = append(buf, reset...)
		buf = append(buf, ']')
	}
	if host != "" {
		buf = append(buf, ' ')
		buf = append(buf, color.If(color.Blue)...)
		buf = append(buf, host...)
		buf = append(buf, reset...)
	}

	buf = append(buf, ' ')
	buf = append(buf, r.Message...)

	for _, a := range cb.attrs {
		buf, _ = appendAttr(buf, h.groupPrefix, a, true)
	}
	buf = append(buf, '\n')

	if h.wMu != nil {
		h.wMu.Lock()
	}
	_, err := h.w.Write(buf)
	if h.wMu != nil {
		h.wMu.Unlock()
	}

	clear(cb.attrs)
	cb.attrs = cb.attrs[:0]
	if cap(buf) <= 64*1024 {
		cb.buf = buf[:0]
		consoleBufPool.Put(cb)
	}
	return err
}

func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &clone
}

func (h *consoleHandler) WithGroup(name string) slog.Handler {
	clone := *h
	clone.groups = append(append([]string(nil), h.groups...), name)
	if len(clone.groups) > 0 {
		clone.groupPrefix = strings.Join(clone.groups, ".") + "."
	}
	return &clone
}

// Write implements io.Writer to support standard logger compatibility if needed
func (h *consoleHandler) Write(p []byte) (n int, err error) {
	if h.wMu != nil {
		h.wMu.Lock()
		defer h.wMu.Unlock()
	}
	return h.w.Write(p)
}

func appendAttr(buf []byte, groupPrefix string, attr slog.Attr, needSpace bool) ([]byte, bool) {
	if attr.Equal(slog.Attr{}) {
		return buf, false
	}

	resolved := attr.Value.Resolve()
	if resolved.Kind() == slog.KindGroup {
		childPrefix := groupPrefix
		if attr.Key != "" {
			childPrefix = groupPrefix + attr.Key + "."
		}
		wroteAny := false
		for _, child := range resolved.Group() {
			var wrote bool
			buf, wrote = appendAttr(buf, childPrefix, child, needSpace)
			if wrote {
				needSpace = true
				wroteAny = true
			}
		}
		return buf, wroteAny
	}

	if needSpace {
		buf = append(buf, ' ')
	}
	buf = append(buf, groupPrefix...)
	buf = append(buf, attr.Key...)
	buf = append(buf, '=')

	switch resolved.Kind() {
	case slog.KindString:
		buf = strconv.AppendQuote(buf, resolved.String())
	case slog.KindDuration:
		buf = append(buf, resolved.Duration().String()...)
	case slog.KindTime:
		buf = resolved.Time().AppendFormat(buf, time.RFC3339)
	case slog.KindBool:
		buf = strconv.AppendBool(buf, resolved.Bool())
	case slog.KindInt64:
		buf = strconv.AppendInt(buf, resolved.Int64(), 10)
	case slog.KindUint64:
		buf = strconv.AppendUint(buf, resolved.Uint64(), 10)
	case slog.KindFloat64:
		buf = strconv.AppendFloat(buf, resolved.Float64(), 'g', -1, 64)
	default:
		buf = fmt.Append(buf, resolved.Any())
	}
	return buf, true
}

// Default returns the current logger. Re-read via atomic snapshot on every call
// so a runtime SetOutput or InstallSink update is picked up immediately.
func Default() *slog.Logger {
	if snap := currentSnapshot.Load(); snap != nil && snap.base != nil {
		return snap.base
	}
	return slog.Default()
}

func With(args ...any) *slog.Logger {
	return Default().With(args...)
}

// Per-side accessors return pre-built *slog.Logger instances from the active
// atomic snapshot, avoiding mutex locks and per-call With("side", ...) allocs.

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
	snap := currentSnapshot.Load()
	if snap == nil {
		return slog.Default().With("side", string(side))
	}
	switch side {
	case SideClient:
		return snap.client
	case SideUpstream:
		return snap.upstream
	case SideDNS:
		return snap.dns
	case SideSystem:
		return snap.system
	case SideControl:
		return snap.control
	default:
		return snap.base.With("side", string(side))
	}
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
