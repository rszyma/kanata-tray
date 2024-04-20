package os_specific

import "syscall"

var ProcessAttr *syscall.SysProcAttr = &syscall.SysProcAttr{}
