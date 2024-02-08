package runner

import "syscall"

var ProcessAttr *syscall.SysProcAttr = &syscall.SysProcAttr{
	HideWindow: true,
}
