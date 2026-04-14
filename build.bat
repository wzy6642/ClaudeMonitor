@echo off
chcp 65001 >nul
echo ╔══════════════════════════════════════════════════════════════╗
echo ║              Claude Monitor 构建脚本 v2.0                    ║
echo ╚══════════════════════════════════════════════════════════════╝
echo.

cd /d "%~dp0"

REM 检查 Go 是否安装
where go >nul 2>&1
if errorlevel 1 (
    echo [错误] Go 未安装或未添加到 PATH
    echo 请从 https://go.dev/dl/ 下载安装 Go
    pause
    exit /b 1
)

echo [1/3] 下载依赖...
go mod tidy
if errorlevel 1 (
    echo 错误: 依赖下载失败
    pause
    exit /b 1
)

echo.
echo [2/3] 编译窗口标题监控工具...
go build -ldflags="-s -w" -o titlewatch.exe ./cmd/titlewatch
if errorlevel 1 (
    echo 错误: 编译失败
    pause
    exit /b 1
)

echo.
echo [3/3] 编译灵动岛主程序...
go build -ldflags="-H windowsgui -s -w" -o ClaudeMonitor.exe .
if errorlevel 1 (
    echo 错误: 编译失败
    pause
    exit /b 1
)

echo.
echo ══════════════════════════════════════════════════════════════
echo 构建完成！
echo.
echo 生成的文件:
for %%f in (*.exe) do echo   - %%f  [%%~zf bytes]
echo.
echo 使用方法:
echo   1. 运行 titlewatch.exe 观察 Claude Code 的窗口标题格式
echo   2. 运行 ClaudeMonitor.exe 启动灵动岛监控
echo.
echo 提示: 右键点击灵动岛可关闭程序
echo ══════════════════════════════════════════════════════════════
pause
