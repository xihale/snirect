package cmd

import (
	"os"
	"testing"

	"github.com/xihale/snirect/config"
)

func TestSkipPrivilegedCAInstall(t *testing.T) {
	t.Setenv("INVOCATION_ID", "")
	if skipPrivilegedCAInstall() {
		t.Fatal("empty INVOCATION_ID must not skip")
	}

	t.Setenv("INVOCATION_ID", "00000000000000000000000000000000")
	if os.Geteuid() == 0 {
		if skipPrivilegedCAInstall() {
			t.Fatal("root systemd unit must not skip")
		}
		return
	}
	if !skipPrivilegedCAInstall() {
		t.Fatal("non-root systemd unit must skip")
	}

	// Must not invoke sudo. Missing cert file is enough: CheckCertStatus
	// returns false and we only warn.
	installCA(&config.Config{CAInstall: "auto"}, t.TempDir())
	installCA(&config.Config{CAInstall: "always"}, t.TempDir())
}
