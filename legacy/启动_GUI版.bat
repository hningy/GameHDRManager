@echo off
cd /d "%~dp0"
title Game HDR Manager

echo ============================================
echo   Game HDR Manager
echo ============================================
echo.

REM ---- Find Python ----
set PY=

if exist "E:\code_software\miniconda\python.exe" (
    set PY=E:\code_software\miniconda\python.exe
    echo Using: E:\code_software\miniconda\python.exe
    goto run
)

if exist "E:\code_software\miniconda3\python.exe" (
    set PY=E:\code_software\miniconda3\python.exe
    echo Using: E:\code_software\miniconda3\python.exe
    goto run
)

where python >nul 2>&1
if not errorlevel 1 (set PY=python & goto run)

where python3 >nul 2>&1
if not errorlevel 1 (set PY=python3 & goto run)

where py >nul 2>&1
if not errorlevel 1 (set PY=py & goto run)

echo Python not found!
echo Please install Python or check: E:\code_software\miniconda\
pause
exit /b 1

:run
echo.
"%PY%" --version
if errorlevel 1 (
    echo Failed to run Python: %PY%
    pause
    exit /b 1
)

echo.
echo Starting GUI...
echo.
"%PY%" "%~dp0game_hdr_gui.py"
pause
