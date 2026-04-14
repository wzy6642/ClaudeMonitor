// Package main is the entry point for ClaudeMonitor application.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/claudemonitor/internal/monitor"
	"github.com/claudemonitor/internal/notify"
	"github.com/claudemonitor/internal/window"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
)

// Window constants
const (
	WS_EX_LAYERED    = 0x00080000
	WS_EX_TOPMOST    = 0x00000008
	WS_EX_TOOLWINDOW = 0x00000080
	WS_POPUP         = 0x80000000
	WS_VISIBLE       = 0x10000000
	SW_SHOW          = 5
	SW_HIDE          = 0
	LWA_ALPHA        = 0x00000002
	SM_CXSCREEN      = 0

	WM_DESTROY     = 0x0002
	WM_PAINT       = 0x000F
	WM_LBUTTONDOWN = 0x0201
	WM_RBUTTONDOWN = 0x0204
	WM_CLOSE       = 0x0010
	WM_COMMAND     = 0x0111
	WM_USER        = 0x0400
	WM_TRAYICON    = WM_USER + 1

	TRANSPARENT = 1
	IDC_ARROW   = 32512

	// Tray constants
	NIM_ADD    = 0x00000000
	NIM_MODIFY = 0x00000001
	NIM_DELETE = 0x00000002

	NIF_MESSAGE = 0x00000001
	NIF_ICON    = 0x00000002
	NIF_TIP     = 0x00000004

	WM_LBUTTONDBLCLK = 0x0203
	WM_RBUTTONUP     = 0x0205

	TPM_RIGHTALIGN  = 0x0008
	TPM_BOTTOMALIGN = 0x0020

	ID_TRAY_SHOW      = 1001
	ID_TRAY_EXIT      = 1002
	ID_TRAY_OPACITY_25 = 1010
	ID_TRAY_OPACITY_50 = 1011
	ID_TRAY_OPACITY_75 = 1012
	ID_TRAY_OPACITY_90 = 1013
	ID_TRAY_OPACITY_100 = 1014
)

// DWM constants for Windows 11
const (
	DWMWA_WINDOW_CORNER_PREFERENCE = 33
	DWMWCP_ROUND                   = 2
)

// Colors (BGR format)
const (
	ColorBackground     = 0x001E1E1E
	ColorRunning        = 0x00FFBF00 // RGB(0, 191, 255) 深天蓝色
	ColorWaitingConfirm = 0x00007FFF // 橘黄色 (Orange)
	ColorCompleted      = 0x0000C734 // Green
	ColorError          = 0x00003BFF // Red
	ColorIdle           = 0x00AAAAAA // Gray
)

// Config 保存配置
type Config struct {
	Opacity byte `json:"opacity"` // 0-255
}

// WindowState tracks state history for each window
type WindowState struct {
	status       monitor.ClaudeStatus
	runningSince time.Time
	notified     bool
	needsConfirm bool
	confirmTime  time.Time
}

type AppState struct {
	hwnd         uintptr
	watcher      *monitor.WindowWatcher
	notifier     *notify.Notifier
	highlighter  *window.Highlighter
	windows      []*monitor.ClaudeWindow
	windowStates map[uintptr]*WindowState
	currentIdx   int
	font         uintptr
	running      bool
	hidden       bool
	opacity      byte
	lastClick    time.Time
	mu           sync.RWMutex
}

var appState AppState

// getConfigPath 获取配置文件路径
func getConfigPath() string {
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	return filepath.Join(exeDir, "config.json")
}

// loadConfig 加载配置
func loadConfig() Config {
	config := Config{Opacity: 220} // 默认值

	data, err := os.ReadFile(getConfigPath())
	if err != nil {
		return config
	}

	json.Unmarshal(data, &config)
	if config.Opacity == 0 {
		config.Opacity = 220
	}
	return config
}

// saveConfig 保存配置
func saveConfig(config Config) {
	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(getConfigPath(), data, 0644)
}

func main() {
	// 加载配置
	config := loadConfig()
	appState.opacity = config.Opacity

	appState.watcher = monitor.NewWindowWatcher()
	appState.notifier = notify.NewNotifier()
	appState.highlighter = window.NewHighlighter()
	appState.windowStates = make(map[uintptr]*WindowState)
	appState.running = true

	hwnd, err := createWindow()
	if err != nil {
		return
	}
	appState.hwnd = hwnd

	// 创建托盘图标
	createTrayIcon(hwnd)

	go startMonitoring()
	runMessageLoop()

	// 退出时清理
	removeTrayIcon(hwnd)
}

func createTrayIcon(hwnd uintptr) {
	type NOTIFYICONDATA struct {
		CbSize           uint32
		Hwnd             uintptr
		UID              uint32
		UFlags           uint32
		UCallbackMessage uint32
		HIcon            uintptr
		SzTip            [128]uint16
		DwState          uint32
		DwStateMask      uint32
		SzInfo           [256]uint16
		UVersion         uint32
		SzInfoTitle      [64]uint16
		DwInfoFlags      uint32
		GuidItem         [16]byte
		HBalloonIcon     uintptr
	}

	var nid NOTIFYICONDATA
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.Hwnd = hwnd
	nid.UID = 1
	nid.UFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP
	nid.UCallbackMessage = WM_TRAYICON

	loadIcon := user32.NewProc("LoadIconW")
	hIcon, _, _ := loadIcon.Call(0, 32512)

	nid.HIcon = hIcon

	tip := "Claude Monitor"
	for i, c := range []rune(tip) {
		if i >= 127 {
			break
		}
		nid.SzTip[i] = uint16(c)
	}

	shellNotifyIcon := shell32.NewProc("Shell_NotifyIconW")
	shellNotifyIcon.Call(uintptr(NIM_ADD), uintptr(unsafe.Pointer(&nid)))
}

func removeTrayIcon(hwnd uintptr) {
	type NOTIFYICONDATA struct {
		CbSize           uint32
		Hwnd             uintptr
		UID              uint32
		UFlags           uint32
		UCallbackMessage uint32
		HIcon            uintptr
		SzTip            [128]uint16
		DwState          uint32
		DwStateMask      uint32
		SzInfo           [256]uint16
		UVersion         uint32
		SzInfoTitle      [64]uint16
		DwInfoFlags      uint32
		GuidItem         [16]byte
		HBalloonIcon     uintptr
	}

	var nid NOTIFYICONDATA
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.Hwnd = hwnd
	nid.UID = 1

	shellNotifyIcon := shell32.NewProc("Shell_NotifyIconW")
	shellNotifyIcon.Call(uintptr(NIM_DELETE), uintptr(unsafe.Pointer(&nid)))
}

func showTrayMenu(hwnd uintptr) {
	createPopupMenu := user32.NewProc("CreatePopupMenu")
	hMenu, _, _ := createPopupMenu.Call()

	appendMenu := user32.NewProc("AppendMenuW")

	// 显示窗口
	showText, _ := syscall.UTF16PtrFromString("显示窗口")
	appendMenu.Call(hMenu, 0, uintptr(ID_TRAY_SHOW), uintptr(unsafe.Pointer(showText)))

	// 不透明度子菜单
	opacityMenu, _, _ := createPopupMenu.Call()

	appState.mu.RLock()
	currentOpacity := appState.opacity
	appState.mu.RUnlock()

	opacityOptions := []struct {
		id     uintptr
		text   string
		value  byte
	}{
		{ID_TRAY_OPACITY_25, "25%", 64},
		{ID_TRAY_OPACITY_50, "50%", 128},
		{ID_TRAY_OPACITY_75, "75%", 192},
		{ID_TRAY_OPACITY_90, "90%", 230},
		{ID_TRAY_OPACITY_100, "100%", 255},
	}

	for _, opt := range opacityOptions {
		optText, _ := syscall.UTF16PtrFromString(opt.text)
		flags := uintptr(0)
		if currentOpacity == opt.value {
			flags = 0x0008 // MF_CHECKED
		}
		appendMenu.Call(opacityMenu, flags, opt.id, uintptr(unsafe.Pointer(optText)))
	}

	opacityText, _ := syscall.UTF16PtrFromString("不透明度")
	appendMenu.Call(hMenu, 0x0010, uintptr(opacityMenu), uintptr(unsafe.Pointer(opacityText))) // MF_POPUP

	// 分隔线
	sepText, _ := syscall.UTF16PtrFromString("-")
	appendMenu.Call(hMenu, 0x800, 0, uintptr(unsafe.Pointer(sepText)))

	// 退出
	exitText, _ := syscall.UTF16PtrFromString("退出")
	appendMenu.Call(hMenu, 0, uintptr(ID_TRAY_EXIT), uintptr(unsafe.Pointer(exitText)))

	// 获取鼠标位置
	type POINT struct {
		X, Y int32
	}
	var pt POINT
	getCursorPos := user32.NewProc("GetCursorPos")
	getCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	setForegroundWindow := user32.NewProc("SetForegroundWindow")
	setForegroundWindow.Call(hwnd)

	trackPopupMenu := user32.NewProc("TrackPopupMenu")
	trackPopupMenu.Call(hMenu, uintptr(TPM_RIGHTALIGN|TPM_BOTTOMALIGN), uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0)

	destroyMenu := user32.NewProc("DestroyMenu")
	destroyMenu.Call(hMenu)
}

func setOpacity(hwnd uintptr, opacity byte) {
	appState.mu.Lock()
	appState.opacity = opacity
	appState.mu.Unlock()

	setLayeredWindowAttributes := user32.NewProc("SetLayeredWindowAttributes")
	setLayeredWindowAttributes.Call(hwnd, 0, uintptr(opacity), LWA_ALPHA)

	// 保存配置
	saveConfig(Config{Opacity: opacity})
}

func createWindow() (uintptr, error) {
	getModuleHandle := kernel32.NewProc("GetModuleHandleW")
	hInstance, _, _ := getModuleHandle.Call(0)

	loadCursor := user32.NewProc("LoadCursorW")
	hCursor, _, _ := loadCursor.Call(0, IDC_ARROW)

	className, _ := syscall.UTF16PtrFromString("ClaudeMonitorClass")

	type WNDCLASSEXW struct {
		CbSize        uint32
		Style         uint32
		LpfnWndProc   uintptr
		CbClsExtra    int32
		CbWndExtra    int32
		HInstance     uintptr
		HIcon         uintptr
		HCursor       uintptr
		HbrBackground uintptr
		LpszMenuName  *uint16
		LpszClassName *uint16
		HIconSm       uintptr
	}

	wc := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		LpfnWndProc:   syscall.NewCallback(windowProc),
		HInstance:     hInstance,
		HCursor:       hCursor,
		LpszClassName: className,
	}

	registerClass := user32.NewProc("RegisterClassExW")
	registerClass.Call(uintptr(unsafe.Pointer(&wc)))

	getSystemMetrics := user32.NewProc("GetSystemMetrics")
	screenWidth, _, _ := getSystemMetrics.Call(SM_CXSCREEN)

	width := 420
	height := 56
	x := (int(screenWidth) - width) / 2
	y := 8

	windowName, _ := syscall.UTF16PtrFromString("ClaudeMonitor")
	createWindowEx := user32.NewProc("CreateWindowExW")

	hwnd, _, err := createWindowEx.Call(
		uintptr(WS_EX_LAYERED|WS_EX_TOPMOST|WS_EX_TOOLWINDOW),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		uintptr(WS_POPUP|WS_VISIBLE),
		uintptr(x), uintptr(y),
		uintptr(width), uintptr(height),
		0, 0, hInstance, 0,
	)

	if hwnd == 0 {
		return 0, fmt.Errorf("CreateWindowEx failed: %v", err)
	}

	enableRoundedCorners(hwnd)

	// 使用保存的不透明度
	setLayeredWindowAttributes := user32.NewProc("SetLayeredWindowAttributes")
	appState.mu.RLock()
	opacity := appState.opacity
	appState.mu.RUnlock()
	setLayeredWindowAttributes.Call(hwnd, 0, uintptr(opacity), LWA_ALPHA)

	appState.font = createFont()
	return hwnd, nil
}

func enableRoundedCorners(hwnd uintptr) {
	dwmSetWindowAttribute := dwmapi.NewProc("DwmSetWindowAttribute")
	cornerPreference := int32(DWMWCP_ROUND)
	dwmSetWindowAttribute.Call(
		hwnd,
		uintptr(DWMWA_WINDOW_CORNER_PREFERENCE),
		uintptr(unsafe.Pointer(&cornerPreference)),
		unsafe.Sizeof(cornerPreference),
	)
}

func createFont() uintptr {
	type LOGFONTW struct {
		Height         int32
		Width          int32
		Escapement     int32
		Orientation    int32
		Weight         int32
		Italic         byte
		Underline      byte
		StrikeOut      byte
		CharSet        byte
		OutPrecision   byte
		ClipPrecision  byte
		Quality        byte
		PitchAndFamily byte
		FaceName       [32]uint16
	}

	lf := LOGFONTW{
		Height:         -16,
		Weight:         700,
		CharSet:        1,
		PitchAndFamily: 34,
	}
	copy(lf.FaceName[:], syscall.StringToUTF16("Segoe UI"))

	createFontIndirect := gdi32.NewProc("CreateFontIndirectW")
	ret, _, _ := createFontIndirect.Call(uintptr(unsafe.Pointer(&lf)))
	return ret
}

func startMonitoring() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		appState.mu.RLock()
		running := appState.running
		appState.mu.RUnlock()

		if !running {
			return
		}

		<-ticker.C
		refresh()
	}
}

func refresh() {
	appState.mu.RLock()
	running := appState.running
	appState.mu.RUnlock()

	if !running {
		return
	}

	newWindows, err := appState.watcher.Refresh()
	if err != nil {
		return
	}

	now := time.Now()

	appState.mu.Lock()

	var notifyWindow *monitor.ClaudeWindow
	for _, newWin := range newWindows {
		state, existed := appState.windowStates[newWin.Handle]
		if !existed {
			state = &WindowState{
				status:       newWin.Status,
				runningSince: now,
			}
			appState.windowStates[newWin.Handle] = state
		}

		if state.needsConfirm {
			newWin.Status = monitor.StatusWaitingConfirm
		} else if newWin.Status == monitor.StatusRunning {
			state.runningSince = now
		} else if newWin.Status == monitor.StatusIdle {
			if state.status == monitor.StatusRunning {
				state.needsConfirm = true
				state.confirmTime = now
				newWin.Status = monitor.StatusWaitingConfirm
				notifyWindow = newWin
			}
		}

		state.status = newWin.Status
	}

	for handle := range appState.windowStates {
		found := false
		for _, w := range newWindows {
			if w.Handle == handle {
				found = true
				break
			}
		}
		if !found {
			delete(appState.windowStates, handle)
		}
	}

	appState.windows = newWindows

	if len(newWindows) > 1 {
		appState.currentIdx = (appState.currentIdx + 1) % len(newWindows)
	}

	invalidateRect := user32.NewProc("InvalidateRect")
	invalidateRect.Call(appState.hwnd, 0, 1)

	appState.mu.Unlock()

	if notifyWindow != nil {
		appState.notifier.NotifyWaiting(notifyWindow)
	}
}

func runMessageLoop() {
	type MSG struct {
		Hwnd    uintptr
		Message uint32
		WParam  uintptr
		LParam  uintptr
		Time    uint32
		Pt      [8]byte
	}

	var msg MSG

	getMessage := user32.NewProc("GetMessageW")
	translateMessage := user32.NewProc("TranslateMessage")
	dispatchMessage := user32.NewProc("DispatchMessageW")

	for {
		appState.mu.RLock()
		running := appState.running
		appState.mu.RUnlock()

		if !running {
			break
		}

		ret, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 {
			break
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func windowProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_DESTROY:
		appState.mu.Lock()
		appState.running = false
		appState.mu.Unlock()
		postQuitMessage := user32.NewProc("PostQuitMessage")
		postQuitMessage.Call(0)
		return 0

	case WM_CLOSE:
		appState.mu.Lock()
		appState.running = false
		appState.mu.Unlock()
		destroyWindow := user32.NewProc("DestroyWindow")
		destroyWindow.Call(hwnd)
		return 0

	case WM_COMMAND:
		switch wParam {
		case ID_TRAY_SHOW:
			showWindow := user32.NewProc("ShowWindow")
			showWindow.Call(hwnd, SW_SHOW)
			setForegroundWindow := user32.NewProc("SetForegroundWindow")
			setForegroundWindow.Call(hwnd)
			appState.mu.Lock()
			appState.hidden = false
			appState.mu.Unlock()
		case ID_TRAY_EXIT:
			appState.mu.Lock()
			appState.running = false
			appState.mu.Unlock()
			postQuitMessage := user32.NewProc("PostQuitMessage")
			postQuitMessage.Call(0)
		case ID_TRAY_OPACITY_25:
			setOpacity(hwnd, 64)
		case ID_TRAY_OPACITY_50:
			setOpacity(hwnd, 128)
		case ID_TRAY_OPACITY_75:
			setOpacity(hwnd, 192)
		case ID_TRAY_OPACITY_90:
			setOpacity(hwnd, 230)
		case ID_TRAY_OPACITY_100:
			setOpacity(hwnd, 255)
		}
		return 0

	case WM_TRAYICON:
		if lParam == WM_LBUTTONDBLCLK {
			showWindow := user32.NewProc("ShowWindow")
			showWindow.Call(hwnd, SW_SHOW)
			setForegroundWindow := user32.NewProc("SetForegroundWindow")
			setForegroundWindow.Call(hwnd)
			appState.mu.Lock()
			appState.hidden = false
			appState.mu.Unlock()
		} else if lParam == WM_RBUTTONUP {
			showTrayMenu(hwnd)
		}
		return 0

	case WM_PAINT:
		onPaint(hwnd)
		return 0

	case WM_LBUTTONDOWN:
		// 防抖：300ms 内只响应一次点击
		appState.mu.Lock()
		if time.Since(appState.lastClick) < 300*time.Millisecond {
			appState.mu.Unlock()
			return 0
		}
		appState.lastClick = time.Now()
		windows := appState.windows
		currentIdx := appState.currentIdx
		appState.mu.Unlock()

		if len(windows) > 0 {
			win := windows[currentIdx%len(windows)]

			go func(handle uintptr) {
				appState.highlighter.HighlightWindow(handle)

				appState.mu.Lock()
				if state, ok := appState.windowStates[handle]; ok {
					if state.needsConfirm {
						state.needsConfirm = false
						state.notified = false
						state.runningSince = time.Time{}
					}
				}
				appState.mu.Unlock()
			}(win.Handle)
		}
		return 0

	case WM_RBUTTONDOWN:
		showWindow := user32.NewProc("ShowWindow")
		showWindow.Call(hwnd, SW_HIDE)
		appState.mu.Lock()
		appState.hidden = true
		appState.mu.Unlock()
		return 0
	}

	defWindowProc := user32.NewProc("DefWindowProcW")
	ret, _, _ := defWindowProc.Call(hwnd, msg, wParam, lParam)
	return ret
}

func onPaint(hwnd uintptr) {
	type PAINTSTRUCT struct {
		Hdc         uintptr
		fErase      bool
		RcPaint     [16]byte
		fRestore    bool
		fIncUpdate  bool
		RgbReserved [32]byte
	}

	type RECT struct {
		Left   int32
		Top    int32
		Right  int32
		Bottom int32
	}

	type SIZE struct {
		Cx int32
		Cy int32
	}

	var ps PAINTSTRUCT

	beginPaint := user32.NewProc("BeginPaint")
	endPaint := user32.NewProc("EndPaint")

	hdc, _, _ := beginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	defer endPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))

	var rect RECT
	getClientRect := user32.NewProc("GetClientRect")
	getClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))

	createSolidBrush := gdi32.NewProc("CreateSolidBrush")
	fillRect := user32.NewProc("FillRect")
	deleteObject := gdi32.NewProc("DeleteObject")

	brush, _, _ := createSolidBrush.Call(0x00202020)
	fillRect.Call(hdc, uintptr(unsafe.Pointer(&rect)), brush)
	deleteObject.Call(brush)

	setBkMode := gdi32.NewProc("SetBkMode")
	setTextColor := gdi32.NewProc("SetTextColor")
	setBkMode.Call(hdc, TRANSPARENT)

	if appState.font != 0 {
		selectObject := gdi32.NewProc("SelectObject")
		selectObject.Call(hdc, appState.font)
	}

	appState.mu.RLock()
	windows := appState.windows
	currentIdx := appState.currentIdx
	appState.mu.RUnlock()

	var text string
	var color uintptr

	if len(windows) == 0 {
		color = ColorIdle
		text = "No Claude Code detected"
	} else {
		idx := currentIdx % len(windows)
		win := windows[idx]
		color = getStatusColor(win.Status)
		statusText := getStatusText(win.Status)
		title := truncateTitle(win.Title, 35)
		text = fmt.Sprintf("[%s] %s", statusText, title)
	}

	setTextColor.Call(hdc, color)

	textPtr, _ := syscall.UTF16PtrFromString(text)
	textLen := len([]rune(text))

	var size SIZE
	getTextExtentPoint32 := gdi32.NewProc("GetTextExtentPoint32W")
	getTextExtentPoint32.Call(hdc, uintptr(unsafe.Pointer(textPtr)), uintptr(textLen), uintptr(unsafe.Pointer(&size)))

	windowWidth := rect.Right - rect.Left
	windowHeight := rect.Bottom - rect.Top
	x := (int(windowWidth) - int(size.Cx)) / 2
	y := (int(windowHeight) - int(size.Cy)) / 2

	textOut := gdi32.NewProc("TextOutW")
	textOut.Call(hdc, uintptr(x), uintptr(y), uintptr(unsafe.Pointer(textPtr)), uintptr(textLen))
}

func getStatusColor(status monitor.ClaudeStatus) uintptr {
	switch status {
	case monitor.StatusRunning:
		return ColorRunning
	case monitor.StatusWaitingConfirm:
		return ColorWaitingConfirm
	case monitor.StatusCompleted:
		return ColorCompleted
	case monitor.StatusError:
		return ColorError
	default:
		return ColorIdle
	}
}

func getStatusText(status monitor.ClaudeStatus) string {
	switch status {
	case monitor.StatusRunning:
		return "RUN"
	case monitor.StatusWaitingConfirm:
		return "WAIT!"
	case monitor.StatusCompleted:
		return "DONE"
	case monitor.StatusError:
		return "ERR"
	default:
		return "IDLE"
	}
}

func truncateTitle(title string, maxLen int) string {
	runes := []rune(title)
	if len(runes) <= maxLen {
		return title
	}
	return string(runes[:maxLen-3]) + "..."
}
