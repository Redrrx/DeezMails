@echo off
setlocal EnableExtensions

rem Build and run DeezMails using environment variables or .env.

cd /d "%~dp0"

where go >nul 2>nul
if errorlevel 1 (
  echo Error: Go 1.27.1 or newer is required.
  exit /b 1
)
where bun >nul 2>nul
if errorlevel 1 (
  echo Error: Bun 1.4.2 or newer is required.
  exit /b 1
)

echo Preparing frontend...
bun install --cwd frontend --frozen-lockfile
if errorlevel 1 exit /b 1
bun run --cwd frontend build
if errorlevel 1 exit /b 1

echo.
echo Starting production...
echo Stop with Ctrl+C.
go run ./backend/cmd/deezmails
exit /b %ERRORLEVEL%
