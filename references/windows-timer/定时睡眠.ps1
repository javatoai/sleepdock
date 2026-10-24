# 选择 30 分钟至 8 小时后让电脑睡眠。
# 倒计时依赖当前 PowerShell 窗口；关闭窗口或按 Ctrl+C 即可取消。
# 可使用 -WhatIf 预览菜单和选择结果，不会等待或让电脑睡眠。
[CmdletBinding(SupportsShouldProcess = $true, ConfirmImpact = 'Low')]
param()

$ErrorActionPreference = 'Stop'

function Format-DelayText {
    param([int]$Minutes)

    $hours = [int][Math]::Floor($Minutes / 60)
    $remainingMinutes = $Minutes % 60
    if ($hours -eq 0) { return ('{0}分钟' -f $remainingMinutes) }
    if ($remainingMinutes -eq 0) { return ('{0}小时' -f $hours) }
    return ('{0}小时{1}分钟' -f $hours, $remainingMinutes)
}

try {
    Write-Host ''
    Write-Host '========== 定时睡眠 ==========' -ForegroundColor Cyan
    Write-Host '请选择多长时间后让电脑睡眠：'
    Write-Host ''

    # 编号 1 至 16，每档增加 30 分钟，最大为 8 小时。
    foreach ($index in 1..16) {
        $label = Format-DelayText -Minutes ($index * 30)
        Write-Host ('  {0,2}. {1}' -f $index, $label)
    }
    Write-Host '   0. 取消并退出'
    Write-Host ''

    while ($true) {
        $answer = Read-Host '请输入选项编号（0-16）'
        if ($null -eq $answer) { $answer = '' }
        $answer = $answer.Trim()

        if ($answer -eq '0' -or $answer -eq 'q') {
            Write-Host '已取消，没有安排睡眠。' -ForegroundColor Yellow
            return
        }

        $choice = 0
        if ([int]::TryParse($answer, [ref]$choice) -and $choice -ge 1 -and $choice -le 16) {
            break
        }
        Write-Host '输入无效，请输入 1-16；输入 0 取消。' -ForegroundColor Yellow
    }

    $minutes = $choice * 30
    $duration = [TimeSpan]::FromMinutes($minutes)
    $delayText = Format-DelayText -Minutes $minutes
    $targetTime = (Get-Date).AddMinutes($minutes)

    # 在倒计时和电源操作之前检查 WhatIf，便于安全验证。
    if (-not $PSCmdlet.ShouldProcess('本机', ('在{0}后进入睡眠' -f $delayText))) {
        return
    }

    # 使用 Windows 的睡眠接口；不选择休眠，不强制挂起应用。
    Add-Type -AssemblyName System.Windows.Forms

    Write-Host ''
    Write-Host ('已启动：{0}后睡眠。' -f $delayText) -ForegroundColor Green
    Write-Host ('预计睡眠时间：{0:yyyy-MM-dd HH:mm:ss}' -f $targetTime)
    Write-Host '请保持此窗口打开。按 Ctrl+C 或关闭窗口即可取消。' -ForegroundColor Yellow
    Write-Host ''

    # 使用单调计时器，避免调整电脑时钟影响倒计时。
    $timer = [Diagnostics.Stopwatch]::StartNew()
    while ($timer.Elapsed -lt $duration) {
        $secondsLeft = [int][Math]::Ceiling(($duration - $timer.Elapsed).TotalSeconds)
        $timeLeft = [TimeSpan]::FromSeconds([Math]::Max(0, $secondsLeft))
        $countdownText = '{0:00}:{1:00}:{2:00}' -f $timeLeft.Hours, $timeLeft.Minutes, $timeLeft.Seconds
        Write-Host -NoNewline ("`r距离睡眠还有：{0}    " -f $countdownText)
        Start-Sleep -Milliseconds 1000
    }
    $timer.Stop()

    Write-Host "`r距离睡眠还有：00:00:00    "
    Write-Host '时间到了，正在让电脑睡眠……' -ForegroundColor Cyan
    $sleepRequested = [System.Windows.Forms.Application]::SetSuspendState(
        [System.Windows.Forms.PowerState]::Suspend,
        $false,
        $false
    )
    if (-not $sleepRequested) {
        throw 'Windows 没有接受睡眠请求，请检查系统电源设置。'
    }
    Write-Host '睡眠请求已执行，定时任务结束。' -ForegroundColor Green
}
catch {
    Write-Host ''
    Write-Host ('执行失败：{0}' -f $_.Exception.Message) -ForegroundColor Red
    Read-Host '按回车键退出' | Out-Null
    exit 1
}