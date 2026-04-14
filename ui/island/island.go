// Package island implements the Dynamic Island UI component.
package island

import (
	"fmt"
	"image/color"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/claudemonitor/internal/monitor"
	"github.com/claudemonitor/internal/notify"
	"github.com/claudemonitor/internal/window"
)

// Status colors (Apple-style)
var (
	ColorRunning       = color.RGBA{R: 0, G: 122, B: 255, A: 255}       // Blue #007AFF
	ColorWaiting       = color.RGBA{R: 255, G: 149, B: 0, A: 255}       // Orange #FF9500
	ColorCompleted     = color.RGBA{R: 52, G: 199, B: 89, A: 255}       // Green #34C759
	ColorError         = color.RGBA{R: 255, G: 59, B: 48, A: 255}       // Red #FF3B30
	ColorBackground    = color.RGBA{R: 30, G: 30, B: 30, A: 230}        // Dark background
	ColorText          = color.RGBA{R: 255, G: 255, B: 255, A: 255}     // White text
	ColorTextSecondary = color.RGBA{R: 160, G: 160, B: 160, A: 255}     // Gray text
)

// Island represents the Dynamic Island UI.
type Island struct {
	app         fyne.App
	window      fyne.Window
	watcher     *monitor.WindowWatcher
	notifier    *notify.Notifier
	highlighter *window.Highlighter

	windows    []*monitor.ClaudeWindow
	currentIdx int
	isExpanded bool
	mu         sync.RWMutex

	// UI components
	container    *fyne.Container
	statusLabel  *widget.Label
	titleLabel   *widget.Label
	itemsList    *widget.List
	expandBtn    *widget.Button
}

// Run starts the Dynamic Island application.
func Run(a fyne.App) {
	island := &Island{
		app:         a,
		watcher:     monitor.NewWindowWatcher(),
		notifier:    notify.NewNotifier(),
		highlighter: window.NewHighlighter(),
		currentIdx:  0,
		isExpanded:  false,
	}

	island.createUI()
	island.startMonitoring()
	island.window.ShowAndRun()
}

// createUI creates the Dynamic Island UI components.
func (i *Island) createUI() {
	// Create a borderless, always-on-top window
	i.window = i.app.NewWindow("Claude Monitor")
	i.window.SetMaster()

	// Set window properties for Dynamic Island appearance
	i.window.Resize(fyne.NewSize(400, 60))
	i.window.CenterOnScreen() // Will be moved to top center later
	i.window.SetPadded(false)

	// Create collapsed view (default)
	i.statusLabel = widget.NewLabel("检测中...")
	i.statusLabel.Alignment = fyne.TextAlignCenter
	i.statusLabel.Importance = widget.MediumImportance

	i.titleLabel = widget.NewLabel("")
	i.titleLabel.Alignment = fyne.TextAlignCenter

	// Expand button (click to expand/collapse)
	i.expandBtn = widget.NewButton("", func() {
		i.toggleExpand()
	})
	i.expandBtn.Importance = widget.LowImportance

	// Create the main container
	bg := canvas.NewRectangle(ColorBackground)
	bg.SetMinSize(fyne.NewSize(400, 60))

	content := container.NewStack(
		bg,
		container.NewVBox(
			layout.NewSpacer(),
			container.NewHBox(
				layout.NewSpacer(),
				i.statusLabel,
				i.titleLabel,
				layout.NewSpacer(),
			),
			layout.NewSpacer(),
		),
	)

	i.container = content
	i.window.SetContent(content)

	// Make window stay on top and borderless
	i.window.SetCloseIntercept(func() {
		// Minimize to tray instead of closing
		i.window.Hide()
	})
}

// startMonitoring begins monitoring Claude Code windows.
func (i *Island) startMonitoring() {
	ticker := time.NewTicker(500 * time.Millisecond)

	go func() {
		for range ticker.C {
			i.refresh()
		}
	}()
}

// refresh updates the window list and UI.
func (i *Island) refresh() {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Get current windows
	newWindows, err := i.watcher.Refresh()
	if err != nil {
		return
	}

	// Check for status changes
	i.detectChanges(newWindows)

	i.windows = newWindows

	// Update UI
	i.updateUI()
}

// detectChanges checks for status changes and triggers notifications.
func (i *Island) detectChanges(newWindows []*monitor.ClaudeWindow) {
	// Check for windows needing confirmation
	for _, win := range newWindows {
		if win.Status == monitor.StatusWaitingConfirm {
			// Auto-expand and notify
			if !i.isExpanded {
				i.isExpanded = true
				i.notifier.NotifyWaiting(win)
			}
		}
	}
}

// updateUI updates the UI based on current state.
func (i *Island) updateUI() {
	if len(i.windows) == 0 {
		i.statusLabel.SetText("未检测到 Claude Code")
		i.titleLabel.SetText("")
		return
	}

	if i.isExpanded {
		i.updateExpandedView()
	} else {
		i.updateCollapsedView()
	}
}

// updateCollapsedView updates the collapsed (compact) view.
func (i *Island) updateCollapsedView() {
	// Rotate through windows every 3 seconds
	if len(i.windows) > 0 {
		win := i.windows[i.currentIdx%len(i.windows)]
		statusIcon := getStatusIcon(win.Status)

		// Truncate title
		title := win.Title
		if len(title) > 30 {
			title = title[:27] + "..."
		}

		i.statusLabel.SetText(fmt.Sprintf("%s %s", statusIcon, win.Status.String()))
		i.titleLabel.SetText(title)

		// Move to next window for next refresh
		i.currentIdx = (i.currentIdx + 1) % len(i.windows)
	}

	// Resize to collapsed size
	i.window.Resize(fyne.NewSize(400, 60))
}

// updateExpandedView updates the expanded view showing all windows.
func (i *Island) updateExpandedView() {
	// Show all windows in a list
	var items []string
	for _, win := range i.windows {
		statusIcon := getStatusIcon(win.Status)
		title := win.Title
		if len(title) > 25 {
			title = title[:22] + "..."
		}
		items = append(items, fmt.Sprintf("%s %s: %s", statusIcon, win.Status.String(), title))
	}

	// Resize to expanded size
	i.window.Resize(fyne.NewSize(400, float32(60+len(items)*30)))
}

// toggleExpand toggles between collapsed and expanded views.
func (i *Island) toggleExpand() {
	i.mu.Lock()
	i.isExpanded = !i.isExpanded
	i.mu.Unlock()
	i.updateUI()
}

// getStatusIcon returns the emoji icon for a status.
func getStatusIcon(status monitor.ClaudeStatus) string {
	switch status {
	case monitor.StatusRunning:
		return "🔵"
	case monitor.StatusWaitingConfirm:
		return "🟠"
	case monitor.StatusCompleted:
		return "🟢"
	case monitor.StatusError:
		return "🔴"
	default:
		return "⚪"
	}
}

// customTheme implements a dark theme for the Dynamic Island.
type customTheme struct{}

func (c *customTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return ColorBackground
	case theme.ColorNameForeground:
		return ColorText
	default:
		return theme.DefaultTheme().Color(name, variant)
	}
}

func (c *customTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (c *customTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (c *customTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

// Ensure customTheme implements fyne.Theme
var _ fyne.Theme = (*customTheme)(nil)

// init sets up the custom theme
func init() {
	// Set custom dark theme
	app.New().Settings().SetTheme(&customTheme{})
}
