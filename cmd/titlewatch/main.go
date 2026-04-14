// TitleWatch is a command-line tool to monitor window title changes in real-time.
// This helps identify the actual window title formats used by Claude Code in different states.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/claudemonitor/internal/monitor"
)

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║         Claude Code 窗口标题监控工具 v1.0                    ║")
	fmt.Println("║    实时监控窗口标题变化，帮助确定 Claude Code 状态格式        ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("使用方法:")
	fmt.Println("  1. 在另一个终端运行 Claude Code")
	fmt.Println("  2. 观察此工具显示的窗口标题变化")
	fmt.Println("  3. 记录不同状态下的标题格式")
	fmt.Println()
	fmt.Println("按 Ctrl+C 退出")
	fmt.Println()
	fmt.Println("────────────────────────────────────────────────────────────────")

	watcher := monitor.NewWindowWatcher()
	prevTitles := make(map[uintptr]string)

	// Clear screen for better visibility
	clearScreen()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		windows, err := watcher.Refresh()
		if err != nil {
			fmt.Printf("错误: %v\n", err)
			continue
		}

		// Clear and redraw
		clearScreen()

		fmt.Println()
		fmt.Printf("  🖥️  Claude Code 窗口监控  │  %s", time.Now().Format("15:04:05"))
		fmt.Println()
		fmt.Println("  ┌────────────────────────────────────────────────────────────┐")

		if len(windows) == 0 {
			fmt.Println("  │  未检测到 Claude Code 窗口                                 │")
			fmt.Println("  │  请确保 Claude Code 正在 Windows Terminal 中运行          │")
		} else {
			for i, win := range windows {
				// Check if title changed
				changed := ""
				if prevTitles[win.Handle] != win.Title {
					changed = " 🆕 变化!"
				}
				prevTitles[win.Handle] = win.Title

				// Truncate title if too long
				title := win.Title
				if len(title) > 40 {
					title = title[:37] + "..."
				}

				// Status indicator
				statusIcon := getStatusIcon(win.Status)
				statusText := win.Status.String()

				fmt.Printf("  │  [%d] %s %-10s %s%s\n", i+1, statusIcon, statusText, title, changed)
				fmt.Printf("  │      进程: %-15s  PID: %d\n", win.ProcessName, win.PID)
				fmt.Printf("  │      句柄: 0x%X\n", win.Handle)
				if i < len(windows)-1 {
					fmt.Println("  │  ─────────────────────────────────────────────────────────  │")
				}
			}
		}

		fmt.Println("  └────────────────────────────────────────────────────────────┘")
		fmt.Println()
		fmt.Println("  提示: 🆕 表示标题刚发生变化，请留意 Claude Code 的操作")
	}
}

func clearScreen() {
	cmd := exec.Command("cmd", "/c", "cls")
	cmd.Stdout = os.Stdout
	cmd.Run()
}

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

// init is not needed, removing it
func _() {
	// This is just to ensure the package compiles
	_ = strings.Join([]string{}, "")
}
