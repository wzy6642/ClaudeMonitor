# Claude Monitor 窗口标题监控工具 (PowerShell 版)
# 用于实时监控 Claude Code 窗口标题变化

# 设置控制台编码为 UTF-8
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

Add-Type @"
    using System;
    using System.Runtime.InteropServices;
    using System.Text;

    public class WindowMonitor {
        [DllImport("user32.dll")]
        public static extern bool EnumWindows(EnumWindowsProc lpEnumFunc, IntPtr lParam);

        [DllImport("user32.dll", CharSet = CharSet.Unicode)]
        public static extern int GetWindowText(IntPtr hWnd, StringBuilder lpString, int nMaxCount);

        [DllImport("user32.dll")]
        public static extern int GetWindowThreadProcessId(IntPtr hWnd, out int lpdwProcessId);

        [DllImport("user32.dll")]
        public static extern bool IsWindowVisible(IntPtr hWnd);

        public delegate bool EnumWindowsProc(IntPtr hWnd, IntPtr lParam);
    }
"@

function Get-AllWindows {
    $script:windowList = @()

    $callback = {
        param([IntPtr]$hWnd, [IntPtr]$lParam)

        # Only get visible windows
        if (-not [WindowMonitor]::IsWindowVisible($hWnd)) {
            return $true
        }

        $title = New-Object System.Text.StringBuilder 512
        $len = [WindowMonitor]::GetWindowText($hWnd, $title, 512)

        if ($len -gt 0) {
            $processId = 0
            [WindowMonitor]::GetWindowThreadProcessId($hWnd, [ref]$processId) | Out-Null

            $processName = ""
            try {
                $proc = Get-Process -Id $processId -ErrorAction SilentlyContinue
                if ($proc) {
                    $processName = $proc.ProcessName
                }
            } catch {}

            $script:windowList += [PSCustomObject]@{
                Handle = $hWnd
                Title = $title.ToString()
                PID = $processId
                ProcessName = $processName
            }
        }

        return $true
    }

    [WindowMonitor]::EnumWindows($callback, [IntPtr]::Zero) | Out-Null
    return $script:windowList
}

function Get-ClaudeStatus {
    param([string]$title)

    # Spinner characters indicate activity
    $spinners = @("⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏", "⠐", "⠂")
    foreach ($s in $spinners) {
        if ($title.Contains($s)) {
            return "执行中", "🔵"
        }
    }

    $titleLower = $title.ToLower()

    # Waiting for confirmation
    $confirmKeywords = @("confirm", "permission", "waiting", "需要确认", "allow", "deny", "proceed")
    foreach ($kw in $confirmKeywords) {
        if ($titleLower.Contains($kw)) {
            return "等待确认", "🟠"
        }
    }

    # Completed
    $completedKeywords = @("completed", "finished", "done", "完成", "成功")
    foreach ($kw in $completedKeywords) {
        if ($titleLower.Contains($kw)) {
            return "已完成", "🟢"
        }
    }

    # Error
    $errorKeywords = @("error", "failed", "错误", "失败")
    foreach ($kw in $errorKeywords) {
        if ($titleLower.Contains($kw)) {
            return "错误", "🔴"
        }
    }

    return "空闲", "⚪"
}

function Show-UI {
    param([array]$windows, [hashtable]$prevTitles)

    $now = Get-Date -Format "HH:mm:ss"

    Write-Host ""
    Write-Host "  🖥️  Claude Code 窗口监控  │  $now" -ForegroundColor Cyan
    Write-Host "  ┌────────────────────────────────────────────────────────────┐"

    if ($windows.Count -eq 0) {
        Write-Host "  │  未检测到 Claude Code 窗口                                 │" -ForegroundColor Gray
        Write-Host "  │  请确保 Claude Code 正在 Windows Terminal 中运行          │" -ForegroundColor Gray
    } else {
        $idx = 1
        foreach ($win in $windows) {
            # Check if title changed
            $changed = ""
            $prevTitle = $prevTitles[$win.Handle]
            if ($prevTitle -ne $null -and $prevTitle -ne $win.Title) {
                $changed = " 🆕"
            }

            # Truncate title
            $title = $win.Title
            if ($title.Length -gt 30) {
                $title = $title.Substring(0, 27) + "..."
            }

            # Get status
            $statusText, $statusIcon = Get-ClaudeStatus $win.Title

            # Color based on status
            $color = switch ($statusText) {
                "执行中" { "Blue" }
                "等待确认" { "Yellow" }
                "已完成" { "Green" }
                "错误" { "Red" }
                default { "White" }
            }

            $line = "  │  [$idx] $statusIcon $statusText".PadRight(30) + $title + $changed
            Write-Host $line -ForegroundColor $color
            Write-Host "  │      进程: $($win.ProcessName.PadRight(12)) PID: $($win.PID)" -ForegroundColor DarkGray
            Write-Host "  │      句柄: 0x$($win.Handle.ToString('X8'))" -ForegroundColor DarkGray

            if ($idx -lt $windows.Count) {
                Write-Host "  │  ─────────────────────────────────────────────────────────  │" -ForegroundColor DarkGray
            }
            $idx++
        }
    }

    Write-Host "  └────────────────────────────────────────────────────────────┘"
    Write-Host ""
    Write-Host "  提示: 🆕 表示标题刚发生变化  │  按 Ctrl+C 退出" -ForegroundColor DarkYellow
}

# Main
Clear-Host
Write-Host "╔══════════════════════════════════════════════════════════════╗" -ForegroundColor Cyan
Write-Host "║         Claude Code 窗口标题监控工具 v1.0 (PowerShell)       ║" -ForegroundColor Cyan
Write-Host "║                                                              ║" -ForegroundColor Cyan
Write-Host "║    实时监控窗口标题变化，帮助确定 Claude Code 状态格式       ║" -ForegroundColor Cyan
Write-Host "╚══════════════════════════════════════════════════════════════╝" -ForegroundColor Cyan

$prevTitles = @{}

while ($true) {
    $allWindows = Get-AllWindows

    # Filter interesting windows
    $claudeWindows = $allWindows | Where-Object {
        $_.ProcessName -eq "WindowsTerminal" -or
        $_.ProcessName -eq "powershell" -or
        $_.ProcessName -eq "pwsh" -or
        $_.Title.ToLower().Contains("claude")
    }

    # Update previous titles
    foreach ($win in $claudeWindows) {
        $prevTitles[$win.Handle] = $win.Title
    }

    # Clear and redraw
    Clear-Host
    Write-Host "╔══════════════════════════════════════════════════════════════╗" -ForegroundColor Cyan
    Write-Host "║         Claude Code 窗口标题监控工具 v1.0 (PowerShell)       ║" -ForegroundColor Cyan
    Write-Host "║                                                              ║" -ForegroundColor Cyan
    Write-Host "║    实时监控窗口标题变化，帮助确定 Claude Code 状态格式       ║" -ForegroundColor Cyan
    Write-Host "╚══════════════════════════════════════════════════════════════╝" -ForegroundColor Cyan

    Show-UI -windows $claudeWindows -prevTitles $prevTitles

    Start-Sleep -Milliseconds 500
}
