//go:build windows

// 载具进程的 JobObject 兜底:每个载具进程一个 KILL_ON_JOB_CLOSE 的 job,
// 载具正常/异常退出或宿主退出(句柄被内核关闭)时回收整棵进程树——
// msedgewebview2 残留会锁住 profile,让下次冷启动一直等不到端口。

package browser

import (
	"os/exec"
	"syscall"
	"unsafe"
)

const (
	jobObjectExtendedLimitInformation = 9
	jobObjectLimitKillOnJobClose      = 0x2000
	processTerminate                  = 0x0001
	processSetQuota                   = 0x0100
)

var (
	kernel32Job         = syscall.NewLazyDLL("kernel32.dll")
	pCreateJobObjectW   = kernel32Job.NewProc("CreateJobObjectW")
	pSetInformationJob  = kernel32Job.NewProc("SetInformationJobObject")
	pAssignProcessToJob = kernel32Job.NewProc("AssignProcessToJobObject")
	pOpenProcess        = kernel32Job.NewProc("OpenProcess")
	pCloseHandle        = kernel32Job.NewProc("CloseHandle")
)

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectExtendedLimitInformation2 struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

// jobHandle 是一个载具进程专属的 job;创建失败返回 nil(少一层兜底)。
type jobHandle struct{ h syscall.Handle }

// newJob 建 KILL_ON_JOB_CLOSE job;失败返回 nil,不影响载具启动。
func newJob() *jobHandle {
	h, _, _ := pCreateJobObjectW.Call(0, 0)
	if h == 0 {
		return nil
	}
	info := jobObjectExtendedLimitInformation2{
		BasicLimitInformation: jobObjectBasicLimitInformation{LimitFlags: jobObjectLimitKillOnJobClose},
	}
	r, _, _ := pSetInformationJob.Call(h, jobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info))
	if r == 0 {
		_, _, _ = pCloseHandle.Call(h)
		return nil
	}
	return &jobHandle{h: syscall.Handle(h)}
}

// assign 把载具进程放进 job;失败只记日志(调用方决定)。
func (j *jobHandle) assign(cmd *exec.Cmd) error {
	if j == nil || cmd.Process == nil {
		return nil
	}
	ph, _, err := pOpenProcess.Call(processTerminate|processSetQuota, 0, uintptr(uint32(cmd.Process.Pid)))
	if ph == 0 {
		return err
	}
	defer func() { _, _, _ = pCloseHandle.Call(ph) }()
	if r, _, err := pAssignProcessToJob.Call(uintptr(j.h), ph); r == 0 {
		return err
	}
	return nil
}

// close 关闭 job 句柄,内核随即结束 job 内所有进程(含 WebView2 子树)。
func (j *jobHandle) close() {
	if j == nil || j.h == 0 {
		return
	}
	_, _, _ = pCloseHandle.Call(uintptr(j.h))
	j.h = 0
}
