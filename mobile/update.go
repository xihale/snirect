package core

import (
	"context"
	"time"

	"github.com/xihale/snirect/update"
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
	info, err := update.Check(ctx, current, "android", "arm64")
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
