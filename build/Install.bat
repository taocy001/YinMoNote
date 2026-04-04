@echo off
:: YinMoNote Windows Installer
:: Launches Install.ps1 with ExecutionPolicy Bypass to work regardless of
:: the system's PowerShell execution policy setting.
powershell.exe -ExecutionPolicy Bypass -File "%~dp0Install.ps1"
pause
