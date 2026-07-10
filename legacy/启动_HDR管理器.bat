@echo off
title Game HDR Manager
cd /d "%~dp0"

echo ========================================
echo   Game HDR Manager
echo ========================================

:: 检查 Python 是否安装
where python >nul 2>&1
if %errorlevel% neq 0 (
    echo [错误] 未找到 Python！请先安装 Python 3。
    echo 下载地址: https://www.python.org/downloads/
    pause
    exit /b 1
)

:: 运行主程序
python game_hdr_manager.py

pause
