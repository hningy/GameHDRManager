@echo off
chcp 65001 >nul
cd /d "%~dp0"

echo ========================================
echo   Game HDR Manager - 环境诊断
echo ========================================
echo.

:: 找 Python
for %%p in (py python3 python) do (
    where %%p >nul 2>&1
    if %errorlevel% equ 0 (
        set PY=%%p
        goto :run
    )
)
echo [错误] 没找到 Python！请先安装 Python 3。
pause
exit /b 1

:run
echo 使用: %PY%
%PY% --version
echo.

%PY% -c "import sys; print(f'Python: {sys.version}'); print(f'路径: {sys.executable}')"
echo.

echo [1] tkinter...
%PY% -c "import tkinter; tk = __import__('tkinter'); r=tk.Tk(); r.destroy(); print('  tkinter OK')"

echo [2] ctypes/user32...
%PY% -c "import ctypes; u=ctypes.windll.user32; print('  ctypes OK')"

echo [3] subprocess...
%PY% -c "import subprocess as s; s.run('echo test', shell=True, creationflags=0x08000000); print('  subprocess OK')"

echo [4] 检查脚本语法...
%PY% -c "import ast; ast.parse(open('game_hdr_gui.py',encoding='utf-8').read()); print('  语法 OK')"

echo [5] 尝试导入脚本...
%PY% -c "import sys; sys.path.insert(0,'.'); exec(open('game_hdr_gui.py',encoding='utf-8').read().replace('if __name__','if False'))" 2>&1

echo.
echo 诊断完成。如果上面没有红色"Error"就说明环境正常。
pause
