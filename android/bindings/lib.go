package core

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/xihale/snirect/internal/cert"
	"github.com/xihale/snirect/internal/proxy"
)

// EngineCallbacks is the gomobile interface the Kotlin side implements. The
// engine pushes status log lines through OnStatusChanged and asks the host to
// route an outbound socket out of the VPN via Protect (VpnService.protect).
//
// Lifecycle contract (fixed API, mirrored 1:1 on the Kotlin side):
//
//   - StartEngine is NON-BLOCKING: it only parses the config JSON and spawns
//     the engine goroutine. All heavy work (CA load/keygen, rules, resolver,
//     proxy listen, netstack start) happens on that goroutine.
//   - OnEngineStarted fires exactly once when ALL components are ready: CA,
//     resolver, proxy listening, netstack run loops started.
//   - OnEngineError fires exactly once on ANY startup failure or runtime
//     engine death (TUN write failure, proxy accept loop death). The engine
//     is fully torn down when it fires, and it is never followed by
//     OnEngineStopped.
//   - OnEngineStopped fires exactly once after an explicit StopEngine tore
//     down an engine that existed (running, or still starting — the Kotlin
//     side treats Stopped as terminal-idle). reason is always "stopped". It
//     is never fired for pure startup failures.
//   - Start/stop races are resolved by an epoch counter: a start (or stop)
//     that wins while an older generation is initializing silences that
//     generation's OnEngine* callbacks entirely, so the host observes exactly
//     one terminal event per start attempt.
type EngineCallbacks interface {
	OnStatusChanged(status string)
	Protect(fd int) bool
	OnEngineStarted()
	OnEngineError(reason string)
	OnEngineStopped(reason string)
}

// engineParts bundles the resources a running engine owns. Ownership is
// exclusive: the parts live either in appState.engine (the published engine of
// the current epoch) or in exactly one goroutine's hands during startup and
// teardown — never both. CertificateManager.Close and Resolver.Close panic on
// double close, so ownership must never be ambiguous.
type engineParts struct {
	certManager *cert.CertificateManager
	proxyServer *proxy.ProxyServer
	bridge      *netstackBridge
	resolver    closeableResolver
}

// any reports whether the parts hold anything that needs tearing down.
func (p engineParts) any() bool {
	return p.certManager != nil || p.proxyServer != nil || p.bridge != nil || p.resolver != nil
}

// teardown stops the netstack pumps, shuts the proxy (which cleanly aborts
// in-flight CONNECT/tunnel copies), and closes the resolver and CA. Each field
// is closed exactly once by the single owner (see the ownership comment on
// engineParts).
func (p engineParts) teardown() {
	if p.bridge != nil {
		_ = p.bridge.Close()
	}
	if p.proxyServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := p.proxyServer.Shutdown(ctx); err != nil {
			LogError("CORE: Proxy Shutdown error: %v", err)
		}
	}
	if p.resolver != nil {
		_ = p.resolver.Close()
	}
	if p.certManager != nil {
		_ = p.certManager.Close()
	}
}

// app holds the mutable engine state. Mutex-protected so StartEngine,
// StopEngine, GetCACertificate and the engine goroutine can't race.
type app struct {
	mu sync.Mutex

	// epoch increments on every StartEngine AND StopEngine. Every state
	// transition (publishing parts, firing OnEngine*) re-checks it under mu,
	// so a superseded generation stays silent (audit L2's stop-during-start
	// race: the stop wins, the in-flight start discards its results).
	epoch uint64

	// hasEngine is true from the moment StartEngine spawns its goroutine
	// until a terminal transition (stop, startup failure, runtime death).
	// StopEngine fires OnEngineStopped only when it observes it true, so a
	// stop of an already-dead engine is a silent no-op and a startup failure
	// (which fired OnEngineError) is never followed by OnEngineStopped.
	hasEngine bool

	// engine holds the published parts of the current epoch's engine. Parts
	// under construction are NOT here yet — the starter goroutine keeps them
	// locally until its point of no return.
	engine engineParts

	dataDir string
}

// closeableResolver is the slice of *dns.Resolver StartEngine needs (Close).
// Kept as an interface so this file doesn't import core/dns just for Close().
type closeableResolver interface {
	Close() error
}

var appState = &app{}

// SetDataDir records where the CA cert/key live. Must be called before
// StartEngine / GetCACertificate.
func SetDataDir(path string) {
	appState.mu.Lock()
	defer appState.mu.Unlock()
	appState.dataDir = path
}

func getCAPaths() (string, string) {
	appState.mu.Lock()
	dir := appState.dataDir
	appState.mu.Unlock()
	if dir == "" {
		return "ca.crt", "ca.key"
	}
	return dir + "/ca.crt", dir + "/ca.key"
}

// StartEngine wires the Android TUN into the shared proxy pipeline:
//
//  1. Load CA + built-in rules + resolver (the resolver's DNS upstreams and the
//     proxy's upstream dials both run through the protect() bypass dialer so
//     they escape the TUN instead of looping back).
//  2. Start proxy.ProxyServer on 127.0.0.1:<auto port> — the single shared
//     SNI/MITM/cert-verify path, identical to desktop.
//  3. Build a gVisor netstack bridge over the VpnService FD that relays every
//     captured TCP flow into the local proxy via a synthesized CONNECT (see
//     netstack.go), and answers DNS via the resolver.
//
// StartEngine itself only parses the config JSON and spawns the engine
// goroutine — it returns within milliseconds even on a first run (CA RSA
// keygen and everything else heavy happens on the goroutine). Failures are
// reported through OnEngineError, success through OnEngineStarted. A config
// JSON parse failure is the one synchronous failure path: OnEngineError fires
// before the return.
//
// Starting while a previous engine exists (running or initializing) first
// tears that engine down completely inside the new goroutine — no leaked
// listener, bridge, or resolver goroutines (audit L2).
func StartEngine(fd int, configStr string, cb EngineCallbacks) {
	cbMutex.Lock()
	lastCb = cb
	cbMutex.Unlock()

	// Synchronous phase: JSON parse only. A malformed config never spawns a
	// goroutine; the caller learns via OnEngineError on the passed callback.
	var config Config
	if err := json.Unmarshal([]byte(configStr), &config); err != nil {
		LogError("CORE: Config Parse Error: %v", err)
		cbError(cb, fmt.Sprintf("config parse error: %v", err))
		return
	}

	// Log level rides in the JSON config (log_level), matching the prior
	// contract — Kotlin never called SetLogLevel directly.
	SetLogLevel(config.LogLevel)
	// Bridge snirect-core's slog into the Android log page early so resolver /
	// proxy / TUN failures are surfaced (not lost to stdout).
	installMobileSink()

	LogInfo("CORE: Starting... FD=%d", fd)

	appState.mu.Lock()
	// A new generation invalidates whatever came before it: bump the epoch
	// (silencing any still-starting older goroutine) and take ownership of the
	// previous engine's parts for teardown inside our goroutine.
	appState.epoch++
	gen := appState.epoch
	superseded := appState.engine
	appState.engine = engineParts{}
	appState.hasEngine = true
	appState.mu.Unlock()

	go runEngine(gen, fd, config, cb, superseded)
}

// runEngine is the engine goroutine: builds every component, publishes it at
// the point of no return, fires OnEngineStarted, then blocks until the engine
// dies (fatal error → OnEngineError + teardown) or is stopped externally
// (StopEngine owns teardown and fires OnEngineStopped in that case).
func runEngine(gen uint64, fd int, config Config, cb EngineCallbacks, superseded engineParts) {
	// Double-start teardown: fully stop the previous engine before building
	// the new one. No callbacks fire for the superseded engine — only an
	// explicit StopEngine fires OnEngineStopped. Shutdown's 1s worst case
	// runs here, on this goroutine, keeping StartEngine non-blocking.
	if superseded.any() {
		LogInfo("CORE: previous engine still present; tearing it down for restart")
		superseded.teardown()
	}

	// parts accumulate the components under construction. Until the publish
	// step they are privately owned by this goroutine; every abort path below
	// tears them down locally.
	var parts engineParts

	// A panic must surface as OnEngineError, not a silently dead engine with
	// the UI still showing ACTIVE (audit B3/L1). After publish, parts have
	// moved into appState and the epoch check inside fireEngineError prevents
	// a double teardown.
	defer func() {
		if r := recover(); r != nil {
			LogError("CORE: engine panicked: %v", r)
			parts.teardown()
			fireEngineError(gen, cb, fmt.Sprintf("engine panic: %v", r))
		}
	}()

	// fail tears down the (unpublished) parts built so far and reports the
	// failure. If the epoch went stale, fireEngineError is a no-op — the
	// newer stop/start owns the terminal callback.
	fail := func(reason string) {
		parts.teardown()
		fireEngineError(gen, cb, reason)
	}

	// 1. CA. First run performs RSA keygen (seconds on a phone) — exactly why
	//    this lives on the goroutine, not in StartEngine.
	caPath, keyPath := getCAPaths()
	cm, err := cert.NewCertificateManager(caPath, keyPath)
	if err != nil {
		LogError("CORE: CA Init Error: %v", err)
		fail(fmt.Sprintf("CA init failed: %v", err))
		return
	}
	parts.certManager = cm
	LogInfo("CORE: CA ready")

	// 2. Resolver + rules + config, plus the protect() bypass dialer shared by
	//    the resolver (DNS upstreams) and the proxy (upstream TCP dials).
	//    initEngine also validates the DNS setup (Android bootstrapping
	//    constraints — see engine.go).
	coreCfg, resolver, ruleSet, bypassDialer, err := initEngine(&config, cb)
	if err != nil {
		LogError("CORE: Engine Init Error: %v", err)
		fail(err.Error())
		return
	}
	parts.resolver = resolver

	// 3. Local proxy server. Outbound dials use the protect() bypass dialer so
	//    the proxy's upstream traffic escapes the TUN.
	srv := proxy.NewProxyServer(coreCfg, ruleSet, cm, resolver)
	srv.SetOutboundDialer(bypassDialer)
	if err := srv.Listen(); err != nil {
		LogError("CORE: Proxy Listen Failed: %v", err)
		fail(fmt.Sprintf("proxy listen failed: %v", err))
		return
	}
	parts.proxyServer = srv
	port := coreCfg.Server.Port
	LogInfo("CORE: proxy listening on 127.0.0.1:%d", port)

	// 4. netstack bridge over the VpnService FD. The loopback hop to the proxy
	//    uses a plain (non-protect) dialer: it never enters the TUN.
	loopback := &net.Dialer{Timeout: 30 * time.Second}
	tun, err := newTunFile(fd)
	if err != nil {
		LogError("CORE: TUN fd wrap failed: %v", err)
		fail(fmt.Sprintf("tun fd: %v", err))
		return
	}
	bridge, err := newNetstackBridge(tun, port, config.MTU, coreCfg, resolver, ruleSet, loopback)
	if err != nil {
		LogError("CORE: Netstack Init Failed: %v", err)
		fail(fmt.Sprintf("netstack init failed: %v", err))
		return
	}
	parts.bridge = bridge

	// 5. Point of no return: publish the parts into appState. From here on
	//    only the epoch-current taker (StopEngine, or fireEngineError below)
	//    may tear them down; this goroutine observes outcomes via channels.
	appState.mu.Lock()
	if appState.epoch != gen {
		appState.mu.Unlock()
		// A stop (or newer start) won while we were initializing: discard
		// everything silently and fire nothing — the winner already fired
		// the terminal callback for us.
		LogDebug("CORE: engine generation %d superseded during startup; discarding", gen)
		parts.teardown()
		return
	}
	appState.engine = parts
	appState.mu.Unlock()
	// Ownership moved: from here on only the epoch-current taker may tear
	// these down. Dropping the local reference keeps the panic handler below
	// from double-closing them (Resolver.Close panics on a second close).
	parts = engineParts{}

	// 6. Run loops. Serve's accept loop and the bridge's TUN pumps both start
	//    immediately; a death on either channel is a fatal engine error.
	serveErrCh := make(chan error, 1)
	go func() { serveErrCh <- srv.Serve() }()

	runErrCh := make(chan error, 1)
	go func() { runErrCh <- bridge.Run() }()

	// 7. All components ready (proxy listening, resolver up, netstack pumps
	//    running): announce. The epoch check keeps a stop that raced us from
	//    seeing Started after its own Stopped (modulo the unavoidable
	//    check-then-fire window; Kotlin treats Stopped as terminal-idle, so a
	//    late Started is overwritten harmlessly).
	if isCurrentEpoch(gen) {
		cbStarted(cb)
	} else {
		LogDebug("CORE: engine generation %d stopped before startup completed", gen)
	}

	// 8. Block until the engine dies on its own or is stopped. Run returns a
	//    non-nil error only when the bridge hit a fatal TUN error; a nil
	//    return means Close() was called (StopEngine or our own death-path
	//    teardown). Serve returns nil on Shutdown, non-nil on accept death.
	var fatal error
	select {
	case err := <-serveErrCh:
		if err != nil {
			fatal = fmt.Errorf("proxy server failed: %v", err)
		}
		_ = bridge.Close() // unblock Run; its nil result is drained below
		<-runErrCh
	case err := <-runErrCh:
		if err != nil {
			fatal = err
		}
	}

	if fatal == nil {
		// Clean stop: StopEngine owns the teardown and fires OnEngineStopped.
		return
	}

	LogError("CORE: Engine died: %v", fatal)
	fireEngineError(gen, cb, fatal.Error())
}

// isCurrentEpoch reports whether gen is still the live engine generation.
func isCurrentEpoch(gen uint64) bool {
	appState.mu.Lock()
	defer appState.mu.Unlock()
	return appState.epoch == gen
}

// fireEngineError tears the published engine of generation gen down (when gen
// is still current), marks the engine slot dead, and reports the failure via
// OnEngineError on the callback that started that generation. When gen is
// stale it is a complete no-op: a newer StopEngine/StartEngine owns the state
// and has already fired its terminal callback (OnEngine* must never fire for
// a superseded engine).
func fireEngineError(gen uint64, cb EngineCallbacks, reason string) {
	appState.mu.Lock()
	if appState.epoch != gen {
		appState.mu.Unlock()
		return
	}
	parts := appState.engine
	appState.engine = engineParts{}
	appState.hasEngine = false
	appState.mu.Unlock()

	parts.teardown()

	LogError("CORE: Engine error: %s", reason)
	cbError(cb, reason)
}

// StopEngine tears the engine down: stops the netstack pumps, shuts the proxy
// (which cleanly aborts in-flight CONNECT/tunnel copies), closes the resolver
// and CA. Synchronous and idempotent.
//
// Epoch semantics: bumping the epoch first means a StartEngine still
// initializing loses the race — its goroutine discards its results at its
// point of no return and fires nothing. OnEngineStopped("stopped") fires only
// when an engine actually existed (running or starting), which is also the
// terminal-idle signal for a stop that cancelled a still-starting engine.
func StopEngine() {
	appState.mu.Lock()
	appState.epoch++
	parts := appState.engine
	appState.engine = engineParts{}
	had := appState.hasEngine
	appState.hasEngine = false
	appState.mu.Unlock()

	if !had {
		// Nothing running or starting: idempotent no-op, no callback (a pure
		// startup failure already fired OnEngineError).
		return
	}

	parts.teardown()

	// The lifecycle callback goes to the most recent callback (the engine
	// being stopped was started with it). Log forwarding (OnStatusChanged)
	// deliberately stays wired so late teardown diagnostics still reach the
	// UI log page.
	cbMutex.RLock()
	cb := lastCb
	cbMutex.RUnlock()
	cbStopped(cb, "stopped")
	LogInfo("CORE: Stopped")
}

// GetCACertificate returns the root CA PEM, loading it on demand if the engine
// isn't running. Used by the app's CA-export/install flow.
//
// The cold-load path creates a CertificateManager only to read the PEM, then
// closes it immediately: keeping it published would leak its cleanupRoutine
// goroutine when a later StartEngine discarded it (audit L2). NewCertificateManager
// persists the CA on first generation, so a subsequent StartEngine loads the
// files and performs the keygen at most once ever.
func GetCACertificate() []byte {
	LogDebug("CORE: GetCACertificate called")
	caPath, keyPath := getCAPaths()

	appState.mu.Lock()
	cm := appState.engine.certManager
	appState.mu.Unlock()

	if cm == nil {
		LogInfo("CORE: Loading CA from %s", caPath)
		cold, err := cert.NewCertificateManager(caPath, keyPath)
		if err != nil {
			LogError("CORE: CA Load Failed: %v", err)
			return nil
		}
		var pemBytes []byte
		if cold.RootCert != nil {
			pemBytes = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cold.RootCert.Raw})
		}
		_ = cold.Close() // cold-loaded only: stop the cleanupRoutine, don't leak it
		return pemBytes
	}

	if cm.RootCert != nil {
		LogDebug("CORE: Returning PEM cert")
		return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cm.RootCert.Raw})
	}
	return nil
}

// cbStarted / cbError / cbStopped invoke the lifecycle callbacks defensively:
// a nil callback (gomobile never passes nil for OnStatusChanged, but the
// lifecycle set is new) or a JVM-side exception must never take down an engine
// goroutine — the same recover() discipline as the log pump in log.go.
func cbStarted(cb EngineCallbacks) {
	if cb == nil {
		return
	}
	defer func() { _ = recover() }()
	cb.OnEngineStarted()
}

func cbError(cb EngineCallbacks, reason string) {
	if cb == nil {
		return
	}
	defer func() { _ = recover() }()
	cb.OnEngineError(reason)
}

func cbStopped(cb EngineCallbacks, reason string) {
	if cb == nil {
		return
	}
	defer func() { _ = recover() }()
	cb.OnEngineStopped(reason)
}
