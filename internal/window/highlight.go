// Package window provides window manipulation functionality for Claude Monitor.
package window

import (
	"syscall"
	"unsafe"
)

// Highlighter provides methods to highlight and bring windows to foreground.
type Highlighter struct {
	user32 *syscall.LazyDLL
}

// NewHighlighter creates a new Highlighter instance.
func NewHighlighter() *Highlighter {
	return &Highlighter{
		user32: syscall.NewLazyDLL("user32.dll"),
	}
}

// HighlightWindow brings a window to the foreground and flashes it.
func (h *Highlighter) HighlightWindow(hwnd uintptr) error {
	// Check if window is valid
	isWindow := h.user32.NewProc("IsWindow")
	valid, _, _ := isWindow.Call(hwnd)
	if valid == 0 {
		return nil
	}

	// 检查窗口是否最小化
	isIconic := h.user32.NewProc("IsIconic")
	iconic, _, _ := isIconic.Call(hwnd)

	setForegroundWindow := h.user32.NewProc("SetForegroundWindow")

	if iconic != 0 {
		// 方法1: 使用 OpenIcon 恢复最小化窗口
		openIcon := h.user32.NewProc("OpenIcon")
		openIcon.Call(hwnd)

		// 方法2: 发送 WM_SYSCOMMAND SC_RESTORE 消息
		const WM_SYSCOMMAND = 0x0112
		const SC_RESTORE = 0xF120
		sendMessage := h.user32.NewProc("SendMessageW")
		sendMessage.Call(hwnd, WM_SYSCOMMAND, SC_RESTORE, 0)

		// 方法3: 使用 ShowWindowAsync
		showWindowAsync := h.user32.NewProc("ShowWindowAsync")
		showWindowAsync.Call(hwnd, uintptr(SW_RESTORE))
	}

	// 使用 SetWindowPos 强制显示并置顶
	setWindowPos := h.user32.NewProc("SetWindowPos")
	const HWND_TOPMOST = ^uintptr(0) // -1
	const HWND_TOP = uintptr(0)
	const SWP_NOSIZE = 0x0001
	const SWP_NOMOVE = 0x0002
	const SWP_SHOWWINDOW = 0x0040

	setWindowPos.Call(hwnd, HWND_TOPMOST, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_SHOWWINDOW)

	// SetForegroundWindow
	setForegroundWindow.Call(hwnd)

	// BringWindowToTop
	bringWindowToTop := h.user32.NewProc("BringWindowToTop")
	bringWindowToTop.Call(hwnd)

	// 取消 TOPMOST
	setWindowPos.Call(hwnd, HWND_TOP, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE)

	// 闪烁任务栏
	h.flashWindowEx(hwnd)

	return nil
}

// flashWindowEx flashes the window's taskbar button.
func (h *Highlighter) flashWindowEx(hwnd uintptr) {
	flashWindowEx := h.user32.NewProc("FlashWindowEx")

	type FLASHWINFO struct {
		cbSize    uint32
		hwnd      uintptr
		dwFlags   uint32
		uCount    uint32
		dwTimeout uint32
	}

	const FLASHW_ALL = 0x00000003
	const FLASHW_TIMERNOFG = 0x0000000C

	info := FLASHWINFO{
		cbSize:    uint32(unsafe.Sizeof(FLASHWINFO{})),
		hwnd:      hwnd,
		dwFlags:   FLASHW_ALL | FLASHW_TIMERNOFG,
		uCount:    5,
		dwTimeout: 0,
	}
	flashWindowEx.Call(uintptr(unsafe.Pointer(&info)))
}

// Constants for ShowWindow
const (
	SW_HIDE            = 0
	SW_SHOWNORMAL      = 1
	SW_SHOWMINIMIZED   = 2
	SW_SHOWMAXIMIZED   = 3
	SW_SHOWNOACTIVATE  = 4
	SW_SHOW            = 5
	SW_MINIMIZE        = 6
	SW_SHOWMINNOACTIVE = 7
	SW_SHOWNA          = 8
	SW_RESTORE         = 9
	SW_SHOWDEFAULT     = 10
	SW_FORCEMINIMIZE   = 11
)
