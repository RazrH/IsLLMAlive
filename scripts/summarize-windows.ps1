param([Parameter(Mandatory=$true)][string]$Directory)
$ErrorActionPreference='Stop'
$sampleDirectory=(Resolve-Path -LiteralPath $Directory).Path
$rows=@(Import-Csv -LiteralPath (Join-Path $sampleDirectory 'process.csv'))
if ($rows.Count -eq 0) { throw 'No process samples' }
function Measure-Field([string]$Name) {
    $values=@($rows | Where-Object { $_.$Name -ne '' } | ForEach-Object { [double]$_.$Name })
    if ($values.Count -eq 0) { return @{count=0} }
    $measurement=$values | Measure-Object -Minimum -Maximum -Average
    return @{count=$values.Count;minimum=$measurement.Minimum;maximum=$measurement.Maximum;average=$measurement.Average;last=$values[-1]}
}
$runtimeMetrics=@{}
$runtimeCount=0
Get-Content -LiteralPath (Join-Path $sampleDirectory 'runtime.jsonl') | ForEach-Object {
    $sample=$_ | ConvertFrom-Json
    $runtimeCount++
    foreach ($line in ($sample.diagnostics -split "`n")) {
        if ($line -match '^(Heap live bytes \(last GC\)|Heap allocated bytes|Heap in-use bytes|Heap idle bytes|Heap released bytes|Stack in-use bytes|GC metadata bytes|Other runtime bytes|Next GC heap bytes|GC cycles|Runtime retained bytes|Goroutines|Last poll HTTP requests): (\d+)$') {
            $name=$Matches[1];$value=[double]$Matches[2]
        } elseif ($line -match '^Last poll duration: ([0-9.]+)(ns|µs|us|ms|s)$') {
            $value=[double]$Matches[1]
            switch ($Matches[2]) { 'ns' {$value/=1e9} 'µs' {$value/=1e6} 'us' {$value/=1e6} 'ms' {$value/=1e3} }
            $name='poll_duration_seconds_sample_weighted'
        } else { continue }
        if (-not $runtimeMetrics.ContainsKey($name)) { $runtimeMetrics[$name]=@{maximum=$value;sum=0.0;count=0;last=0.0} }
        $entry=$runtimeMetrics[$name];$entry.maximum=[Math]::Max($entry.maximum,$value);$entry.sum+=$value;$entry.count++;$entry.last=$value
    }
}
foreach ($entry in $runtimeMetrics.Values) { $entry.average=$entry.sum/$entry.count; $entry.Remove('sum') }
$elapsed=([datetime]$rows[-1].time_utc-[datetime]$rows[0].time_utc).TotalSeconds
$cpuDelta=[double]$rows[-1].cpu_seconds-[double]$rows[0].cpu_seconds
$summary=@{
    process_samples=$rows.Count;runtime_samples=$runtimeCount;sampled_elapsed_seconds=$elapsed
    private_working_set_bytes=(Measure-Field 'private_working_set_bytes')
    private_bytes=(Measure-Field 'private_bytes');working_set_bytes=(Measure-Field 'working_set_bytes')
    peak_working_set_bytes=(Measure-Field 'peak_working_set_bytes')
    handles=(Measure-Field 'handles');threads=(Measure-Field 'threads')
    cpu_seconds_delta=$cpuDelta;cpu_percent_of_one_core=$(if($elapsed -gt 0){100*$cpuDelta/$elapsed}else{0})
    runtime=$runtimeMetrics;counter_errors=@($rows | Where-Object {$_.counter_error -ne ''}).Count
    caveat='Private working set differs from Go heap. Live heap is the last-GC metric; no forced GC. Poll duration averages are sample-weighted. Inspect manifest and raw samples before acceptance.'
}
$utf8=New-Object Text.UTF8Encoding($false)
[IO.File]::WriteAllText((Join-Path $sampleDirectory 'summary.json'),($summary|ConvertTo-Json -Depth 6),$utf8)
$summary | ConvertTo-Json -Depth 6
