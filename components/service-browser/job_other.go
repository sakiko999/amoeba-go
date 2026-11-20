//go:build !windows

package browser

import "os/exec"

type jobHandle struct{}

func newJob() *jobHandle                  { return &jobHandle{} }
func (*jobHandle) assign(*exec.Cmd) error { return nil }
func (*jobHandle) close()                 {}
