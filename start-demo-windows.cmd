@echo off
setlocal EnableExtensions



cd /d "%~dp0"

where bun >nul 2>nul
if errorlevel 1 (
  echo Error: Bun 1.4.2 or newer is required.
  exit /b 1
)

if not defined PORT set "PORT=8080"

echo Preparing frontend...
bun install --cwd frontend --frozen-lockfile
if errorlevel 1 exit /b 1
bun run --cwd frontend build:demo
if errorlevel 1 exit /b 1

echo.
echo Starting demo at http://127.0.0.1:%PORT%
echo Stop with Ctrl+C.
bun run --cwd frontend preview:demo --port %PORT%
exit /b %ERRORLEVEL%
