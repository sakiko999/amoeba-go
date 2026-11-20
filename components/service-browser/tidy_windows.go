// 载具窗口整理:CDP 创建的 page target 是 WebView2 的独立顶层窗口,默认样式
// (可见 + 无 TOOLWINDOW)会进任务栏。载具进程周期性枚举自己拉起的 WebView2
// 进程树的顶层可见窗口,去掉 WS_EX_APPWINDOW 并补上 WS_EX_TOOLWINDOW|
// WS_EX_NOACTIVATE(幂等,不动其他样式位)。宿主建 target 后会经 stdin 触发
// 一次立即整理,避免任务栏按钮闪到下一个周期。纯 syscall;仅 Windows 使用。

//go:build windows

package browser

import (
	"os"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

const (
	gwlExStyle   = ^uintptr(19) // GWL_EXSTYLE(-20)
	wsExToolWin  = uintptr(0x80)
	wsExAppWin   = uintptr(0x40000)
	wsExNoAct    = uintptr(0x08000000)
	tidyInterval = 2 * time.Second
)

// tidyKick 让 startTidy 跳过当前等待立即整理一次(宿主 stdin "tidy" 触发)。
var tidyKick = make(chan struct{}, 1)

func kickTidy() {
	select {
	case tidyKick <- struct{}{}:
	default: // 已有待处理的触发
	}
}

var (
	user32            = syscall.NewLazyDLL("user32.dll")
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	pEnumWindows      = user32.NewProc("EnumWindows")
	pGetWindowPID     = user32.NewProc("GetWindowThreadProcessId")
	pIsWindowVisible  = user32.NewProc("IsWindowVisible")
	pGetWindowLongPtr = user32.NewProc("GetWindowLongPtrW")
	pSetWindowLongPtr = user32.NewProc("SetWindowLongPtrW")

	// tidyPids/tidyFixed 是 enum 回调与 tidyOnce 的通信槽;回调只能注册一次
	// (NewCallback 每进程上限 2000),init 期建好。
	tidyPids  atomic.Pointer[map[uint32]bool]
	tidyFixed atomic.Int32
	enumCB    = syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		pids := *tidyPids.Load()
		var pid uint32
		pGetWindowPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if !pids[pid] {
			return 1
		}
		if vis, _, _ := pIsWindowVisible.Call(hwnd); vis == 0 {
			return 1
		}
		style, _, _ := pGetWindowLongPtr.Call(hwnd, gwlExStyle)
		if style&wsExToolWin != 0 && style&wsExAppWin == 0 {
			return 1
		}
		pSetWindowLongPtr.Call(hwnd, gwlExStyle, style&^wsExAppWin|wsExToolWin|wsExNoAct)
		tidyFixed.Add(1)
		return 1
	})
)

// startTidy 载具主循环:周期整理直到进程退出(host 侧不调用)。
func startTidy(logf func(string, ...any)) {
	go func() {
		for {
			if n := tidyOnce(descendantPids(uint32(os.Getpid()))); n > 0 && logf != nil {
				logf("隐藏 %d 个 target 窗口(TOOLWINDOW|NOACTIVATE)", n)
			}
			select {
			case <-time.After(tidyInterval):
			case <-tidyKick:
			}
		}
	}()
}

func tidyOnce(pids map[uint32]bool) int {
	if len(pids) == 0 {
		return 0
	}
	tidyPids.Store(&pids)
	tidyFixed.Store(0)
	_, _, _ = pEnumWindows.Call(enumCB, 0)
	return int(tidyFixed.Load())
}

type processEntry struct {
	Size        uint32
	Usage       uint32
	ProcessID   uint32
	DefaultHeap uintptr
	ModuleID    uint32 // 不是 ULONG_PTR;写错会让后续字段整体错位
	Threads     uint32
	ParentPID   uint32
	PriClass    int32
	Flags       uint32
	ExeFile     [260]uint16 // MAX_PATH;尺寸必须与 C 结构完全一致
}

var (
	pSnapshot  = kernel32.NewProc("CreateToolhelp32Snapshot")
	pProcFirst = kernel32.NewProc("Process32FirstW")
	pProcNext  = kernel32.NewProc("Process32NextW")
	pClose     = kernel32.NewProc("CloseHandle")
)

// descendantPids 快照全系统 pid→parent,收集 selfPID 的全部子孙。
func descendantPids(selfPID uint32) map[uint32]bool {
	const snapProcess = 0x2
	snap, _, _ := pSnapshot.Call(snapProcess, 0)
	if snap == 0 || snap == ^uintptr(0) {
		return nil
	}
	defer pClose.Call(snap)

	parent := map[uint32]uint32{}
	var entry processEntry
	entry.Size = uint32(unsafe.Sizeof(entry))
	if r, _, _ := pProcFirst.Call(snap, uintptr(unsafe.Pointer(&entry))); r == 0 {
		return nil
	}
	for {
		parent[entry.ProcessID] = entry.ParentPID
		entry.Size = uint32(unsafe.Sizeof(entry))
		if r, _, _ := pProcNext.Call(snap, uintptr(unsafe.Pointer(&entry))); r == 0 {
			break
		}
	}
	out := map[uint32]bool{}
	for orig := range parent {
		pid := orig
		for range 32 { // 沿父链上行,防御成环
			if pid == selfPID {
				out[orig] = true
				break
			}
			up, ok := parent[pid]
			if !ok || up == pid {
				break
			}
			pid = up
		}
	}
	return out
}
