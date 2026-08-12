[CmdletBinding()]
param(
    [string]$Version = $env:NETDEBUG_VERSION,
    [string]$Repo = $(if ($env:NETDEBUG_REPO) { $env:NETDEBUG_REPO } else { "neko233-com/netdebug" }),
    [string]$InstallDir = $(if ($env:NETDEBUG_INSTALL_DIR) { $env:NETDEBUG_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "netdebug\bin" }),
    [switch]$NoPathUpdate,
    [switch]$Run
)

$ErrorActionPreference = "Stop"

function Fail([string]$Message) {
    throw "netdebug installer: $Message"
}

function Get-CandidateUrls([string]$OriginalUrl) {
    $urls = [System.Collections.Generic.List[string]]::new()
    $urls.Add($OriginalUrl)
    if ($env:NETDEBUG_DIRECT_ONLY -eq "1") { return $urls }
    $mirrors = if ($null -ne $env:NETDEBUG_UPDATE_MIRRORS) { $env:NETDEBUG_UPDATE_MIRRORS } else { "https://gh-proxy.com/,https://ghfast.top/,https://ghproxy.net/" }
    foreach ($mirror in ($mirrors -split ",")) {
        if ([string]::IsNullOrWhiteSpace($mirror)) { continue }
        $urls.Add(($mirror.TrimEnd("/") + "/" + $OriginalUrl))
    }
    return $urls
}

function Get-FastestUrl([string]$OriginalUrl) {
    $best = $null
    $bestSeconds = [double]::PositiveInfinity
    foreach ($candidate in (Get-CandidateUrls $OriginalUrl)) {
        $watch = [System.Diagnostics.Stopwatch]::StartNew()
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Method Head -Uri $candidate -MaximumRedirection 5 -TimeoutSec 5
            $watch.Stop()
            if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 400 -and $watch.Elapsed.TotalSeconds -lt $bestSeconds) {
                $best = $candidate
                $bestSeconds = $watch.Elapsed.TotalSeconds
            }
        } catch {
            $watch.Stop()
        }
    }
    if (-not $best) { Fail "no reachable update route for $OriginalUrl" }
    return $best
}

if ([string]::IsNullOrWhiteSpace($Version) -or $Version -eq "latest") {
    $apiUrl = Get-FastestUrl "https://api.github.com/repos/$Repo/releases/latest"
    $release = Invoke-RestMethod -Headers @{ "User-Agent" = "netdebug-installer" } -Uri $apiUrl
    $Version = $release.tag_name
}
if (-not $Version.StartsWith("v")) { $Version = "v$Version" }

$arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
switch ($arch) {
    "x64" { $archName = "amd64" }
    "arm64" { $archName = "arm64" }
    default { Fail "unsupported architecture: $arch" }
}
$versionNumber = $Version.TrimStart("v")
$asset = "netdebug_${versionNumber}_windows_${archName}.zip"
$baseUrl = "https://github.com/$Repo/releases/download/$Version"
$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("netdebug-install-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tempDir | Out-Null

try {
    Write-Host "Installing netdebug $Version (windows/$archName)"
    $archive = Join-Path $tempDir $asset
    $checksums = Join-Path $tempDir "checksums.txt"
    $assetUrl = Get-FastestUrl "$baseUrl/$asset"
    $routeBase = $assetUrl.Substring(0, $assetUrl.Length - ("/" + $asset).Length)
    Invoke-WebRequest -Headers @{ "User-Agent" = "netdebug-installer" } -Uri "$routeBase/$asset" -OutFile $archive
    Invoke-WebRequest -Headers @{ "User-Agent" = "netdebug-installer" } -Uri "$routeBase/checksums.txt" -OutFile $checksums

    $line = Get-Content $checksums | Where-Object { $_ -match "\s$([regex]::Escape($asset))$" } | Select-Object -First 1
    if (-not $line) { Fail "checksum missing for $asset" }
    $expected = ($line -split "\s+")[0].ToLowerInvariant()
    $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $archive).Hash.ToLowerInvariant()
    if ($expected -ne $actual) { Fail "checksum verification failed" }

    Expand-Archive -LiteralPath $archive -DestinationPath $tempDir -Force
    $binary = Join-Path $tempDir "netdebug.exe"
    if (-not (Test-Path -LiteralPath $binary)) { Fail "release archive has no netdebug.exe" }
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Copy-Item -LiteralPath $binary -Destination (Join-Path $InstallDir "netdebug.exe") -Force
    [Environment]::SetEnvironmentVariable("NETDEBUG_HOME", $InstallDir, "User")
    Write-Host "Installed: $(Join-Path $InstallDir 'netdebug.exe')"

    if (-not $NoPathUpdate) {
        $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
        $parts = @($userPath -split ";" | Where-Object { $_ })
        if ($parts -notcontains $InstallDir) {
            [Environment]::SetEnvironmentVariable("Path", (($parts + $InstallDir) -join ";"), "User")
            Write-Host "Added install directory to user PATH. Open a new terminal."
        }
    }
}
finally {
    Remove-Item -LiteralPath $tempDir -Recurse -Force -ErrorAction SilentlyContinue
}

if ($Run) {
    Write-Host "Running netdebug report"
    & (Join-Path $InstallDir "netdebug.exe")
    exit $LASTEXITCODE
}
