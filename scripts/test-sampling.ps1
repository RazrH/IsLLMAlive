$ErrorActionPreference = 'Stop'
foreach ($script in Get-ChildItem -LiteralPath $PSScriptRoot -Filter '*.ps1') {
    $tokens=$null; $errors=$null
    [System.Management.Automation.Language.Parser]::ParseFile($script.FullName,[ref]$tokens,[ref]$errors) | Out-Null
    if ($errors.Count) { throw "$($script.Name): $($errors[0].Message)" }
}
function Assert-Equal($Actual, $Expected, [string]$Message) {
    if ($Actual -ne $Expected) { throw "$Message`: expected $Expected, got $Actual" }
}
$fixtureDirectory = Join-Path ([IO.Path]::GetTempPath()) ('isllmalive-sampling-test-'+[Guid]::NewGuid())
New-Item -ItemType Directory -Path $fixtureDirectory | Out-Null
try {
    @(
        [pscustomobject]@{time_utc='2026-10-10T00:00:00Z';private_working_set_bytes=1000;private_bytes=4000;working_set_bytes=5000;peak_working_set_bytes=5000;cpu_seconds=1;handles=10;threads=2;counter_error=''},
        [pscustomobject]@{time_utc='2026-10-10T00:00:10Z';private_working_set_bytes='';private_bytes=6000;working_set_bytes=7000;peak_working_set_bytes=7000;cpu_seconds=2;handles=12;threads=3;counter_error='CIM unavailable'},
        [pscustomobject]@{time_utc='2026-10-10T00:00:20Z';private_working_set_bytes=3000;private_bytes=5000;working_set_bytes=6000;peak_working_set_bytes=7000;cpu_seconds=3;handles=11;threads=2;counter_error=''}
    ) | Export-Csv -LiteralPath (Join-Path $fixtureDirectory 'process.csv') -NoTypeInformation
    $durationFixtures=@('1.25s','500ms','250µs','125us','100ns')
    $runtimeLines=foreach ($duration in $durationFixtures) {
        @{time='2026-10-10T00:00:00Z';diagnostics="Heap allocated bytes: 1000`nHeap in-use bytes: 4096`nHeap idle bytes: 8192`nHeap released bytes: 4096`nStack in-use bytes: 2048`nGC metadata bytes: 512`nOther runtime bytes: 256`nNext GC heap bytes: 4194304`nGC cycles: 3`nLast poll duration: $duration`nUnrelated detail: 99"} | ConvertTo-Json -Compress
    }
    $runtimeLines | Set-Content -LiteralPath (Join-Path $fixtureDirectory 'runtime.jsonl') -Encoding UTF8
    & (Join-Path $PSScriptRoot 'summarize-windows.ps1') -Directory $fixtureDirectory | Out-Null
    $summary=Get-Content -Raw -LiteralPath (Join-Path $fixtureDirectory 'summary.json') | ConvertFrom-Json
    Assert-Equal $summary.process_samples 3 'Process sample count'
    Assert-Equal $summary.runtime_samples 5 'Runtime sample count'
    Assert-Equal $summary.private_working_set_bytes.count 2 'Missing counter excluded'
    Assert-Equal $summary.private_working_set_bytes.average 2000 'Private WS average'
    Assert-Equal $summary.private_working_set_bytes.maximum 3000 'Private WS peak'
    Assert-Equal $summary.private_working_set_bytes.last 3000 'Private WS last'
    Assert-Equal $summary.private_bytes.average 5000 'Private bytes average'
    Assert-Equal $summary.handles.maximum 12 'Handle peak'
    Assert-Equal $summary.counter_errors 1 'Counter errors'
    Assert-Equal $summary.cpu_percent_of_one_core 10 'CPU seconds normalized'
    foreach ($metric in @('Heap in-use bytes','Heap idle bytes','Heap released bytes','Stack in-use bytes','GC metadata bytes','Other runtime bytes','Next GC heap bytes','GC cycles')) {
        Assert-Equal $summary.runtime.$metric.count 5 "Runtime metric $metric"
    }
    Assert-Equal $summary.runtime.'GC cycles'.last 3 'GC count'
    Assert-Equal $summary.runtime.'Heap idle bytes'.average 8192 'Heap idle average'
    if ([Math]::Abs($summary.runtime.poll_duration_seconds_sample_weighted.average-0.35007502) -gt 1e-12) { throw 'Duration units were not converted to seconds' }
    Assert-Equal $summary.runtime.poll_duration_seconds_sample_weighted.last 0.0000001 'Nanosecond duration'
    if ($summary.runtime.PSObject.Properties.Name -contains 'Unrelated detail') { throw 'Unrelated diagnostic parsed as metric' }

    $single=@([pscustomobject]@{time_utc='2026-10-10T00:00:00Z';private_working_set_bytes='';private_bytes=4000;working_set_bytes=5000;peak_working_set_bytes=5000;cpu_seconds=1;handles=10;threads=2;counter_error='unavailable'})
    $single | Export-Csv -LiteralPath (Join-Path $fixtureDirectory 'process.csv') -NoTypeInformation
    & (Join-Path $PSScriptRoot 'summarize-windows.ps1') -Directory $fixtureDirectory | Out-Null
    $summary=Get-Content -Raw -LiteralPath (Join-Path $fixtureDirectory 'summary.json') | ConvertFrom-Json
    Assert-Equal $summary.private_working_set_bytes.count 0 'Entirely missing counter'
    Assert-Equal $summary.cpu_percent_of_one_core 0 'Single sample CPU'
    'Sampling self-check passed'
} finally {
    $resolvedFixture=[IO.Path]::GetFullPath($fixtureDirectory)
    $expectedRoot=[IO.Path]::GetFullPath([IO.Path]::GetTempPath())
    if (-not $resolvedFixture.StartsWith($expectedRoot,[StringComparison]::OrdinalIgnoreCase)) { throw 'Unexpected fixture cleanup path' }
    Remove-Item -LiteralPath $resolvedFixture -Recurse
}