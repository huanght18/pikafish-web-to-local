@echo off
setlocal
cd /d "%~dp0"

set "PYTHON_EXE="
if exist ".venv\Scripts\python.exe" set "PYTHON_EXE=.venv\Scripts\python.exe"
if not defined PYTHON_EXE set "PYTHON_EXE=python"

echo [INFO] Using Python: %PYTHON_EXE%
"%PYTHON_EXE%" -c "import websockets" >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Missing dependency: websockets
    echo [HINT] Run: %PYTHON_EXE% -m pip install websockets
    pause
    exit /b 1
)

echo [INFO] Starting WebSocket server...
"%PYTHON_EXE%" servepkf2.py

echo.
echo [INFO] Server stopped.
pause
