//go:build !windows

package syswin

import (
	"context"
	"fmt"
)

// SelectFileDialog is unsupported on non-Windows platforms.
func SelectFileDialog(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("native file dialog is only supported on Windows")
}
