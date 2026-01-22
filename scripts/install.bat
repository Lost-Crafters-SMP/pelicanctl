@echo off
REM Install script for pelicanctl (Windows CMD fallback)
REM This script launches the PowerShell installer

echo Installing pelicanctl...
echo.
echo This script will use PowerShell to install pelicanctl.
echo If you prefer to install manually, use:
echo   powershell -ExecutionPolicy Bypass -File install.ps1
echo.

powershell -ExecutionPolicy Bypass -Command "& {[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; Invoke-WebRequest -Uri 'https://raw.githubusercontent.com/Lost-Crafters-SMP/pelicanctl/main/scripts/install.ps1' -OutFile '%TEMP%\pelicanctl-install.ps1'; & '%TEMP%\pelicanctl-install.ps1'; Remove-Item '%TEMP%\pelicanctl-install.ps1'}"

if errorlevel 1 (
    echo.
    echo Installation failed. Please try running the PowerShell script directly:
    echo   powershell -ExecutionPolicy Bypass -Command "irm https://raw.githubusercontent.com/Lost-Crafters-SMP/pelicanctl/main/scripts/install.ps1 | iex"
    exit /b 1
)
