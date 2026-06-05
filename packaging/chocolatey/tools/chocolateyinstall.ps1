$ErrorActionPreference = 'Stop'
$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Definition

# {{VERSION}} and {{SHA256}} are filled in by the Windows CI at pack time
# (from the just-built, just-uploaded release asset).
$packageArgs = @{
  packageName    = 'goto-window'
  fileFullPath   = Join-Path $toolsDir 'goto.exe'
  url64bit       = 'https://github.com/Espigah/goto/releases/download/v{{VERSION}}/goto_{{VERSION}}_windows_amd64.exe'
  checksum64     = '{{SHA256}}'
  checksumType64 = 'sha256'
}

# downloads the official release exe (verifying the checksum) into the package
# tools dir; Chocolatey then shims it as `goto` on PATH.
Get-ChocolateyWebFile @packageArgs
