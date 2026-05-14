//go:build windows

package tools

import (
	"os/exec"
)

func setSysProcAttr(cmd *exec.Cmd) {
	// Windows doesn't support Setpgid; nothing to do.
}
