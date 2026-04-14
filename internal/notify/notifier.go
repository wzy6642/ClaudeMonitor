// Package notify provides notification functionality for Claude Monitor.
package notify

import (
	"syscall"
	"unsafe"

	"github.com/claudemonitor/internal/monitor"
)

// Notifier handles notifications for Claude Code status changes.
type Notifier struct {
	user32 *syscall.LazyDLL
}

// NewNotifier creates a new Notifier instance.
func NewNotifier() *Notifier {
	return &Notifier{
		user32: syscall.NewLazyDLL("user32.dll"),
	}
}

// NotifyWaiting sends a notification when Claude Code is waiting for confirmation.
func (n *Notifier) NotifyWaiting(win *monitor.ClaudeWindow) {
	// 异步发送通知，避免阻塞主线程
	go func() {
		n.PlaySound(SoundNotify)
		n.flashTaskbar()
	}()
}

// NotifyCompleted sends a notification when Claude Code has completed.
func (n *Notifier) NotifyCompleted(win *monitor.ClaudeWindow) {
	go func() {
		n.PlaySound(SoundSuccess)
	}()
}

// SoundType represents different notification sounds.
type SoundType int

const (
	SoundNotify SoundType = iota
	SoundSuccess
	SoundError
)

// PlaySound plays a notification sound.
func (n *Notifier) PlaySound(soundType SoundType) {
	var soundName string
	switch soundType {
	case SoundNotify:
		soundName = "SystemNotification"
	case SoundSuccess:
		soundName = "SystemNotification"
	case SoundError:
		soundName = "SystemHand"
	}
	n.playSystemSound(soundName)
}

// playSystemSound plays a Windows system sound by name.
func (n *Notifier) playSystemSound(soundName string) {
	winmm := syscall.NewLazyDLL("winmm.dll")
	playsound := winmm.NewProc("PlaySoundW")

	soundPtr, err := syscall.UTF16PtrFromString(soundName)
	if err != nil {
		return
	}

	// SND_ASYNC | SND_NODEFAULT | SND_ALIAS
	const flags = 0x0001 | 0x0002 | 0x00010000
	playsound.Call(uintptr(unsafe.Pointer(soundPtr)), 0, uintptr(flags))
}

// flashTaskbar flashes the taskbar to get user attention
func (n *Notifier) flashTaskbar() {
	user32 := syscall.NewLazyDLL("user32.dll")
	flashWindowEx := user32.NewProc("FlashWindowEx")

	type FLASHWINFO struct {
		cbSize    uint32
		hwnd      uintptr
		dwFlags   uint32
		uCount    uint32
		dwTimeout uint32
	}

	// FLASHW_ALL | FLASHW_TIMERNOFG = 0x00000003 | 0x0000000C
	const FLASHW_ALL = 0x00000003
	const FLASHW_TIMERNOFG = 0x0000000C

	// Get foreground window
	getForegroundWindow := user32.NewProc("GetForegroundWindow")
	hwnd, _, _ := getForegroundWindow.Call()

	if hwnd != 0 {
		info := FLASHWINFO{
			cbSize:    uint32(unsafe.Sizeof(FLASHWINFO{})),
			hwnd:      hwnd,
			dwFlags:   FLASHW_ALL | FLASHW_TIMERNOFG,
			uCount:    3,
			dwTimeout: 0,
		}
		flashWindowEx.Call(uintptr(unsafe.Pointer(&info)))
	}
}
