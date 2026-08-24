package sysproxy

import (
	"os/exec"
	"strings"
)

func isSystemProxySet() bool {
	if !HasTool("networksetup") {
		return false
	}
	services, err := exec.Command("networksetup", "-listallnetworkservices").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(services), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "*") {
			continue
		}
		out, err := exec.Command("networksetup", "-getautoproxyurl", line).Output()
		if err == nil && strings.Contains(string(out), "http://127.0.0.1") {
			return true
		}
	}
	return false
}
