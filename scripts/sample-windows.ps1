param(
    [Parameter(Mandatory=$true)][string]$Executable,
    [Parameter(Mandatory=$true)][string]$OutputDirectory,
    [ValidateSet(5,20)][int]$MonitorCount = 5,
    [ValidateRange(10,172800)][int]$DurationSeconds = 86400,
    [ValidateRange(1,300)][int]$IntervalSeconds = 30,
    [string]$Label = 'optimized',
    [ValidateSet(1,10)][int]$RefreshMinutes = 10
)
$ErrorActionPreference = 'Stop'
$sourceExecutable = (Resolve-Path -LiteralPath $Executable).Path
$sourceBuildInfo = @(& go version -m $sourceExecutable)
if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect executable build information' }

$sampleDirectory = [IO.Path]::GetFullPath($OutputDirectory)
if (Test-Path -LiteralPath $sampleDirectory) { throw "OutputDirectory must be new: $sampleDirectory" }
New-Item -ItemType Directory -Path $sampleDirectory | Out-Null
$sampleExecutable = Join-Path $sampleDirectory 'isllmalive.exe'
Copy-Item -LiteralPath $sourceExecutable -Destination $sampleExecutable
$templates = @(
    @{type='openai';name='OpenAI';enabled=$true;notify_on=$false;components=@('none');status_page='https://status.openai.com'},
    @{type='statuspage';name='Claude';enabled=$true;notify_on=$false;endpoint='https://status.claude.com';components=@('none');status_page='https://status.claude.com'},
    @{type='google';name='Google AI';enabled=$true;notify_on=$false;components=@('none');status_page='https://aistudio.google.com/status'},
    @{type='deepseek';name='DeepSeek';enabled=$true;notify_on=$false;endpoint='https://status.deepseek.com/feed.rss';components=@('API Service','Web Chat Service');status_page='https://status.deepseek.com'},
    @{type='apiget';name='Z.ai';enabled=$true;notify_on=$false;endpoint='https://api.z.ai/api/paas/v4/chat/completions';status_page='https://z.ai'}
)
$monitors = @()
for ($index = 0; $index -lt $MonitorCount; $index++) {
    $monitor = [ordered]@{}
    $templates[$index % 5].GetEnumerator() | Sort-Object Name | ForEach-Object { $monitor[$_.Key] = $_.Value }
    if ($MonitorCount -gt 5) { $monitor.name = "$($monitor.name) $([int][Math]::Floor($index / 5)+1)" }
    $monitors += $monitor
}
$config = [ordered]@{language='en-US';refresh_interval_minutes=$RefreshMinutes;global_notify_on=$false;monitors=$monitors}
$utf8 = New-Object Text.UTF8Encoding($false)
[IO.File]::WriteAllText((Join-Path $sampleDirectory 'config.json'), ($config | ConvertTo-Json -Depth 10), $utf8)
$runtimePath = Join-Path $sampleDirectory 'runtime.jsonl'
$stdoutPath = Join-Path $sampleDirectory 'stdout.log'
$stderrPath = Join-Path $sampleDirectory 'stderr.log'
$arguments = @('--metrics-file', ('"'+$runtimePath+'"'), '--metrics-interval', "$($IntervalSeconds)s", '--run-for', "$($DurationSeconds)s")
$started = [DateTime]::UtcNow
$previousGoDebug = $env:GODEBUG
$samplingGoDebug = 'fips140=off'
if ($previousGoDebug) { $samplingGoDebug = $previousGoDebug+',fips140=off' }
try {
    $env:GODEBUG = $samplingGoDebug
    $child = Start-Process -FilePath $sampleExecutable -WorkingDirectory $sampleDirectory -ArgumentList $arguments -WindowStyle Hidden -PassThru -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath
} finally { $env:GODEBUG = $previousGoDebug }
$manifest = @{
    label=$Label;pid=$child.Id;monitor_count=$MonitorCount;started_utc=$started.ToString('o')
    duration_seconds=$DurationSeconds;interval_seconds=$IntervalSeconds
    executable_sha256=(Get-FileHash -LiteralPath $sampleExecutable -Algorithm SHA256).Hash
    build_info=$sourceBuildInfo;godebug=$samplingGoDebug;config_sha256=(Get-FileHash -LiteralPath (Join-Path $sampleDirectory 'config.json') -Algorithm SHA256).Hash
    refresh_minutes=$RefreshMinutes;notifications=$false;completed=$false;forced_termination=$false
}
$manifestPath = Join-Path $sampleDirectory 'manifest.json'
[IO.File]::WriteAllText($manifestPath, ($manifest | ConvertTo-Json), $utf8)
$csvPath = Join-Path $sampleDirectory 'process.csv'
try {
    while (-not $child.HasExited -and [DateTime]::UtcNow -lt $started.AddSeconds($DurationSeconds)) {
        $child.Refresh()
        $privateWorkingSet = $null
        $counterError = ''
        try {
            $counter = Get-CimInstance Win32_PerfFormattedData_PerfProc_Process -Filter "IDProcess=$($child.Id)"
            if ($null -ne $counter) { $privateWorkingSet = $counter.WorkingSetPrivate } else { $counterError = 'private working set counter unavailable' }
        } catch { $counterError = $_.Exception.Message }
        [pscustomobject]@{
            time_utc=[DateTime]::UtcNow.ToString('o');pid=$child.Id
            private_working_set_bytes=$privateWorkingSet
            working_set_bytes=$child.WorkingSet64;private_bytes=$child.PrivateMemorySize64
            peak_working_set_bytes=$child.PeakWorkingSet64;cpu_seconds=$child.TotalProcessorTime.TotalSeconds
            handles=$child.HandleCount;threads=$child.Threads.Count;counter_error=$counterError
            vmmap_running=[bool](Get-Process -Name vmmap,vmmap64 -ErrorAction SilentlyContinue)
        } | Export-Csv -LiteralPath $csvPath -NoTypeInformation -Append -Encoding UTF8
        Start-Sleep -Seconds $IntervalSeconds
        $child.Refresh()
    }
    if (-not $child.HasExited -and -not $child.WaitForExit(30000)) {
        # Only stop the exact child process object created by this sampler.
        $manifest.forced_termination = $true
        $child.Kill()
        $child.WaitForExit()
    }
    $child.WaitForExit()
    $manifest.exit_code = $child.ExitCode
    $manifest.finished_utc = [DateTime]::UtcNow.ToString('o')
    $manifest.elapsed_seconds = ([DateTime]::UtcNow - $started).TotalSeconds
    $manifest.completed = ($manifest.elapsed_seconds -ge $DurationSeconds -and -not $manifest.forced_termination -and $child.ExitCode -eq 0 -and (Get-Item -LiteralPath $runtimePath).Length -gt 0)
} finally {
    if (-not $child.HasExited) { $manifest.forced_termination=$true; $child.Kill(); $child.WaitForExit() }
    [IO.File]::WriteAllText($manifestPath, ($manifest | ConvertTo-Json), $utf8)
}
& (Join-Path $PSScriptRoot 'summarize-windows.ps1') -Directory $sampleDirectory
Write-Output $sampleDirectory
