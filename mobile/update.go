package core

import (
	"context"
	"runtime"
	"time"

	"github.com/xihale/snirect/internal/update"
)

// UpdateInfo is the gomobile-facing subset of update.Info. Download/install
// stay on the desktop CLI; Android only checks and opens the release URL.
type UpdateInfo struct {
	Current string
	Latest  string
	Newer   bool
	URL     string
	Notes   string
}

// CheckUpdate looks up the latest GitHub release using the same Hosts-IP +
// empty-SNI path as the proxy. current is the host app's versionName.
func CheckUpdate(current string) (*UpdateInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// runtime.GOARCH 是当前 APK 实际跑的 ABI (arm64 版→arm64, x86_64 版→amd64),
	// 这样模拟器上的 x86_64 包不会误下 arm64 APK。
	info, err := update.Check(ctx, current, "android", runtime.GOARCH)
	if err != nil {
		return nil, err
	}
	notes := info.Notes
	if len(notes) > 400 {
		notes = notes[:400] + "…"
	}
	return &UpdateInfo{
		Current: current,
		Latest:  info.Latest,
		Newer:   info.Newer,
		URL:     info.URL,
		Notes:   notes,
	}, nil
}
