@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion

echo ========================================
echo   Rizline 谱面预览图生成器
echo ========================================
echo.

if not exist "riz-chart-renderer.exe" (
    echo 正在编译...
    go build
    if errorlevel 1 (
        echo 编译失败！请确保已安装 Go。
        pause
        exit /b 1
    )
    echo.
)

if not exist "charts" (
    echo 错误：找不到 charts 目录！
    pause
    exit /b 1
)

if not exist "output" mkdir output

echo.
echo 选择渲染模式：
echo 1. 渲染单个谱面
echo 2. 批量渲染所有谱面
echo 3. 自定义参数渲染
echo 4. 退出
echo.

choice /c 1234 /n /m "请选择 (1-4): "

if errorlevel 4 goto :eof
if errorlevel 3 goto custom
if errorlevel 2 goto batch
if errorlevel 1 goto single

:single
echo.
echo 可用的谱面文件：
dir /b charts\*.json
echo.
set /p CHART_FILE="请输入谱面文件名: "

if not exist "charts\%CHART_FILE%" (
    echo 文件不存在！
    pause
    goto :eof
)

set /p OUTPUT_FILE="请输入输出文件名（留空使用默认）: "

if "%OUTPUT_FILE%"=="" (
    riz-chart-renderer.exe -chart "charts\%CHART_FILE%" -output "output\%CHART_FILE:~0,-5%_preview.png"
) else (
    riz-chart-renderer.exe -chart "charts\%CHART_FILE%" -output "output\%OUTPUT_FILE%"
)

echo.
echo 完成！文件保存在 output 目录
pause
goto :eof

:batch
echo.
echo 正在批量渲染所有谱面...
echo.

del /q output\*_new.png 2>nul

for %%f in (charts\*.json) do (
    echo 正在渲染: %%~nxf
    riz-chart-renderer.exe -chart "%%f" -output "output\%%~nf_new.png"

    if errorlevel 1 (
        echo   ❌ 渲染失败
    ) else (
        echo   ✓ 完成
    )
    echo.
)

echo.
echo ========================================
echo   批量渲染完成！
echo ========================================
echo.
echo 输出位置: output\
pause
goto :eof

:custom
echo.
echo 自定义参数渲染
echo.

set /p CHART_FILE="请输入谱面文件名: "

if not exist "charts\%CHART_FILE%" (
    echo 文件不存在！
    pause
    goto :eof
)

set /p OUTPUT_FILE="请输入输出文件名（不含.png）: "
set /p COL_WIDTH="列宽度 (默认400，直接回车): "
set /p COL_HEIGHT="列高度 (默认2800，直接回车): "
set /p NOTE_SCALE="音符缩放 (默认1.0，直接回车): "

set CMD=riz-chart-renderer.exe -chart "charts\%CHART_FILE%" -output "output\%OUTPUT_FILE%.png"

if not "%COL_WIDTH%"=="" set CMD=!CMD! -colwidth %COL_WIDTH%
if not "%COL_HEIGHT%"=="" set CMD=!CMD! -colheight %COL_HEIGHT%
if not "%NOTE_SCALE%"=="" set CMD=!CMD! -scale %NOTE_SCALE%

echo.
echo 执行命令: %CMD%
echo.

%CMD%

echo.
echo 完成！
pause
goto :eof
