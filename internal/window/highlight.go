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
	// First, restore the window if minimized
	h.showWindow(hwnd, SW_RESTORE)

	// Bring to foreground
	h.setForegroundWindow(hwnd)

	// Flash the window to draw attention
	h.flashWindow(hwnd, true)

	return nil
}

// showWindow calls the Win32 ShowWindow function.
func (h *Highlighter) showWindow(hwnd uintptr, cmdShow int) bool {
	proc := h.user32.NewProc("ShowWindow")
	ret, _, _ := proc.Call(hwnd, uintptr(cmdShow))
	return ret != 0
}

// setForegroundWindow brings a window to the foreground.
func (h *Highlighter) setForegroundWindow(hwnd uintptr) bool {
	// Allow set foreground window permission
	// This is needed because Windows restricts which processes can set foreground window
	h.allowSetForegroundWindow()

	proc := h.user32.NewProc("SetForegroundWindow")
	ret, _, _ := proc.Call(hwnd)
	return ret != 0
}

// flashWindow flashes the window's title bar.
func (h *Highlighter) flashWindow(hwnd uintptr, invert bool) bool {
	proc := h.user32.NewProc("FlashWindow")
	var invertVal uintptr
	if invert {
		invertVal = 1
	}
	ret, _, _ := proc.Call(hwnd, invertVal)
	return ret != 0
}

// allowSetForegroundWindow allows the process to set foreground window.
func (h *Highlighter) allowSetForegroundWindow() {
	user32 := syscall.NewLazyDLL("user32.dll")
	proc := user32.NewProc("AllowSetForegroundWindow")
	// ASFW_ANY = -1, but we need to pass it as unsigned
	proc.Call(^uintptr(0)) // Allow any process (equivalent to -1)
}

// GetWindowRect gets the window's position and size.
func (h *Highlighter) GetWindowRect(hwnd uintptr) (left, top, right, bottom int32, ok bool) {
	type RECT struct {
		Left, Top, Right, Bottom int32
	}
	var rect RECT

	proc := h.user32.NewProc("GetWindowRect")
	ret, _, _ := proc.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	if ret == 0 {
		return 0, 0, 0, 0, false
	}
	return rect.Left, rect.Top, rect.Right, rect.Bottom, true
}

// IsWindowVisible checks if a window is visible.
func (h *Highlighter) IsWindowVisible(hwnd uintptr) bool {
	proc := h.user32.NewProc("IsWindowVisible")
	ret, _, _ := proc.Call(hwnd)
	return ret != 0
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
