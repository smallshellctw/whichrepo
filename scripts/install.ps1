$ErrorActionPreference = "Stop"
$repo = "smallshellctw/whichrepo"
$version = if ($env:WHICHREPO_VERSION) { $env:WHICHREPO_VERSION } else { "latest" }
$installDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { Join-Path $HOME ".local\bin" }
$arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq "Arm64") { "arm64" } else { "amd64" }
if ($version -eq "latest") { $version = (Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest").tag_name }
$plainVersion = $version.TrimStart("v")
$archive = "whichrepo_${plainVersion}_windows_${arch}.zip"
$base = "https://github.com/$repo/releases/download/$version"
$temp = Join-Path ([System.IO.Path]::GetTempPath()) ("whichrepo-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $temp | Out-Null
try {
  Invoke-WebRequest "$base/$archive" -OutFile (Join-Path $temp $archive)
  Invoke-WebRequest "$base/checksums.txt" -OutFile (Join-Path $temp "checksums.txt")
  $expected = ((Get-Content (Join-Path $temp "checksums.txt") | Where-Object { $_ -match [regex]::Escape($archive) }) -split "\s+")[0]
  $actual = (Get-FileHash (Join-Path $temp $archive) -Algorithm SHA256).Hash.ToLower()
  if ($expected -ne $actual) { throw "Checksum verification failed" }
  Expand-Archive (Join-Path $temp $archive) -DestinationPath $temp
  New-Item -ItemType Directory -Force -Path $installDir | Out-Null
  Copy-Item (Join-Path $temp "whichrepo.exe") (Join-Path $installDir "whichrepo.exe") -Force
  Write-Host "Installed whichrepo to $(Join-Path $installDir 'whichrepo.exe')"
} finally { Remove-Item $temp -Recurse -Force -ErrorAction SilentlyContinue }
