//go:build !windows

package notify

import "os/exec"

func hideWindow(cmd *exec.Cmd) {}
