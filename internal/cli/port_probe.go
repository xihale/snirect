package cli

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// portProbe is the result of asking the OS who is listening on the configured
// proxy port. It is the replacement for the deleted control API as `snirect
// status`'s source of runtime truth: since there is no longer a daemon HTTP
// surface to query, status infers whether the proxy is up by checking whether
// the configured TCP port is bound, and by whom.
type portProbe struct {
	occupied bool   // a process is listening on the port
	owner    string // process name (best-effort, e.g. "snirect")
	err      string // non-empty if the probe itself failed (tool missing, etc.)
}

// probeProxyPort reports who owns the configured proxy port. A 2-second budget
// keeps `status` snappy even when the underlying tool is slow.
//
// Limitations: this only reflects the HTTP-proxy listener. TUN mode does not
// bind a TCP port, so a running TUN daemon will report an idle port here —
// TUN users should consult the Service status block instead.
func probeProxyPort(port int) portProbe {
	if port == 0 {
		// No TCP port in play (TUN mode, or port not configured).
		return portProbe{err: "no proxy port configured"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	switch runtime.GOOS {
	case "windows":
		return probeWindows(ctx, port)
	default:
		return probeUnix(ctx, port)
	}
}

// probeUnix shells out to `lsof -nP -iTCP:<port> -sTCP:LISTEN -F pc` and parses
// the `p` (PID) and `c` (command) fields. lsof ships with macOS and most Linux
// distros; where it is absent the probe degrades to a status-unavailable line
// rather than reporting a misleading "free" port.
func probeUnix(ctx context.Context, port int) portProbe {
	out, err := exec.CommandContext(ctx, "lsof", "-nP",
		fmt.Sprintf("-iTCP:%d", port), "-sTCP:LISTEN", "-F", "pc").Output()
	if err != nil {
		// Distinguish "nothing listening" (lsof ran, exit 1, no output) from a
		// genuine tooling failure (lsof missing, timeout).
		if _, missing := err.(*exec.Error); missing {
			return portProbe{err: "lsof not installed"}
		}
		return portProbe{} // nothing is listening — port is free
	}

	// lsof -F emits one record per line, prefixed by a field tag: 'p' = PID,
	// 'c' = command. A single LISTEN entry yields one p/c pair.
	var pid, comm string
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) == 0 {
			continue
		}
		switch line[0] {
		case 'p':
			pid = line[1:]
		case 'c':
			comm = line[1:]
		}
	}
	if comm == "" && pid != "" {
		comm = "pid " + pid
	}
	return portProbe{occupied: true, owner: comm}
}

// probeWindows finds the LISTENING PID via `netstat -ano -p TCP`, then resolves
// it to an image name with `tasklist`. Both tools ship with every Windows.
func probeWindows(ctx context.Context, port int) portProbe {
	out, err := exec.CommandContext(ctx, "netstat", "-ano", "-p", "TCP").Output()
	if err != nil {
		return portProbe{err: "netstat failed: " + err.Error()}
	}

	// A LISTENING row looks like:  TCP  127.0.0.1:8080   0.0.0.0:0  LISTENING  1234
	needle := fmt.Sprintf(":%d", port)
	var pid string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 5 || fields[3] != "LISTENING" {
			continue
		}
		if strings.HasPrefix(fields[1], "127.0.0.1"+needle) ||
			strings.HasPrefix(fields[1], "0.0.0.0"+needle) ||
			strings.HasPrefix(fields[1], "[::]"+needle) ||
			strings.Contains(fields[1], needle) {
			pid = fields[4]
			break
		}
	}
	if pid == "" {
		return portProbe{} // nothing listening on the port
	}

	name := "pid " + pid
	if tl, err := exec.CommandContext(ctx, "tasklist", "/FO", "CSV", "/NH",
		"/FI", "PID eq "+pid).Output(); err == nil {
		// CSV row: "snirect.exe","1234","Services","0","5,120 K"
		if row := firstCSVField(string(bytes.TrimSpace(tl))); row != "" {
			name = row
		}
	}
	return portProbe{occupied: true, owner: name}
}

// firstCSVField returns the first comma-delimited field of the first CSV row,
// with surrounding quotes stripped. Empty on any parse failure.
func firstCSVField(csv string) string {
	if i := strings.IndexByte(csv, '\n'); i >= 0 {
		csv = csv[:i]
	}
	csv = strings.TrimSpace(csv)
	if !strings.HasPrefix(csv, "\"") {
		return ""
	}
	end := strings.IndexByte(csv[1:], '"')
	if end < 0 {
		return ""
	}
	return csv[1 : 1+end]
}

// looksLikeSnirect reports whether a process name is the snirect binary. It
// matches with a "snirect" prefix so platform suffixes (.exe) and versioned
// builds still count.
func looksLikeSnirect(name string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(name)), "snirect")
}
