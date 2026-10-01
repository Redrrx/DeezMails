@echo off
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0install-dependencies-windows.ps1"
exit /b %ERRORLEVEL%
