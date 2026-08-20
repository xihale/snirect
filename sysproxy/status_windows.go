package sysproxy

import (
	"os/exec"
	"strings"
)

func isSystemProxySet() bool {
	out, err := exec.Command("reg", "query",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Internet`,
		"/v", "AutoConfigURL",
	).Output()
	return err == nil && strings.Contains(string(out), "http://")
}
