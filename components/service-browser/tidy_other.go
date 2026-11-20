// 非 Windows:载具本身不可用(见 carrier.go),窗口整理为空实现。

//go:build !windows

package browser

func startTidy(func(string, ...any)) {}

func kickTidy() {}
