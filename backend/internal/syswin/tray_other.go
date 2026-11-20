//go:build !windows

package syswin

import "context"

// RunTray blocks until ctx is cancelled on non-Windows platforms.
func RunTray(ctx context.Context, _ string, _ func(), _ func()) {
	<-ctx.Done()
}
