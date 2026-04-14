// Package monitor provides window monitoring functionality for Claude Code instances.
package monitor

import (
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

// ClaudeStatus represents the current status of a Claude Code instance.
type ClaudeStatus int

const (
	StatusIdle ClaudeStatus = iota
	StatusRunning
	StatusWaitingConfirm
	StatusCompleted
	StatusError
)

func (s ClaudeStatus) String() string {
	switch s {
	case StatusRunning:
		return "RUNNING"
	case StatusWaitingConfirm:
		return "WAITING"
	case StatusCompleted:
		return "DONE"
	case StatusError:
		return "ERROR"
	default:
		return "IDLE"
	}
}

// ClaudeWindow represents a detected Claude Code window.
type ClaudeWindow struct {
	Handle      uintptr
	Title       string
	Status      ClaudeStatus
	PID         uint32
	ProcessName string
}

// WindowWatcher monitors windows for Claude Code instances.
type WindowWatcher struct {
	user32                     *syscall.LazyDLL
	enumWindowsProc            *syscall.LazyProc
	getWindowTextProc          *syscall.LazyProc
	getWindowThreadProcessIdProc *syscall.LazyProc
	enumCallback               uintptr // 缓存 callback，避免重复创建
}

// NewWindowWatcher creates a new WindowWatcher instance.
func NewWindowWatcher() *WindowWatcher {
	u := syscall.NewLazyDLL("user32.dll")
	return &WindowWatcher{
		user32:                     u,
		enumWindowsProc:            u.NewProc("EnumWindows"),
		getWindowTextProc:          u.NewProc("GetWindowTextW"),
		getWindowThreadProcessIdProc: u.NewProc("GetWindowThreadProcessId"),
	}
}

// Refresh scans all windows and updates the window list.
func (w *WindowWatcher) Refresh() ([]*ClaudeWindow, error) {
	var windows []*ClaudeWindow
	var windowsMu sync.Mutex // 保护 windows 切片的并发访问

	callback := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
		title := w.getText(hwnd)
		if title == "" {
			return 1
		}

		if !isRealClaudeCodeWindow(title) {
			return 1
		}

		var pid uint32
		w.getWindowThreadProcessIdProc.Call(hwnd, uintptr(unsafe.Pointer(&pid)))

		status := detectStatus(title)

		windowsMu.Lock()
		windows = append(windows, &ClaudeWindow{
			Handle: hwnd,
			Title:  title,
			Status: status,
			PID:    pid,
		})
		windowsMu.Unlock()

		return 1
	})

	w.enumWindowsProc.Call(callback, 0)

	return windows, nil
}

func (w *WindowWatcher) getText(hwnd uintptr) string {
	buf := make([]uint16, 512)
	w.getWindowTextProc.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 512)
	return syscall.UTF16ToString(buf)
}

// isRealClaudeCodeWindow checks if this is a real Claude Code terminal window
func isRealClaudeCodeWindow(title string) bool {
	if strings.Contains(title, "ClaudeMonitor") {
		return false
	}

	title = strings.TrimSpace(title)

	if strings.HasSuffix(title, "Claude Code") {
		return true
	}

	if strings.Contains(title, "Claude Code") {
		return true
	}

	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏", "⠐", "⠂", "✳", "●", "○"}
	for _, s := range spinners {
		if strings.HasPrefix(title, s) {
			return true
		}
	}

	return false
}

// detectStatus analyzes the window title to determine Claude Code status.
func detectStatus(title string) ClaudeStatus {
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏", "⠐", "⠂", "●"}
	for _, s := range spinners {
		if strings.HasPrefix(title, s) {
			return StatusRunning
		}
	}

	titleLower := strings.ToLower(title)
	waitingKeywords := []string{"proceed?", "confirm", "permission", "waiting", "select", "yes / no"}
	for _, kw := range waitingKeywords {
		if strings.Contains(titleLower, kw) {
			return StatusWaitingConfirm
		}
	}

	completedKeywords := []string{"completed", "finished", "done", "success"}
	for _, kw := range completedKeywords {
		if strings.Contains(titleLower, kw) {
			return StatusCompleted
		}
	}

	errorKeywords := []string{"error", "failed", "exception"}
	for _, kw := range errorKeywords {
		if strings.Contains(titleLower, kw) {
			return StatusError
		}
	}

	return StatusIdle
}
