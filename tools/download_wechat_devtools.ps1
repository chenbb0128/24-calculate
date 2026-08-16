$downloadDir = 'D:\微信小游戏\downloads'
$outputPath = Join-Path $downloadDir 'wechat_devtools_2.01.2601082_win32_x64.exe'
$downloadUrl = 'https://dldir1.qq.com/WechatWebDev/nightly/p-3bd19c2db3a642a0b39af853efaf67f8/0.54.1/wechat_devtools_2.01.2601082_win32_x64.exe'

New-Item -ItemType Directory -Path $downloadDir -Force | Out-Null
if (Test-Path -LiteralPath $outputPath) {
    Remove-Item -LiteralPath $outputPath -Force
}

$client = New-Object System.Net.WebClient
$client.Headers.Add('User-Agent', 'Mozilla/5.0')
$client.DownloadFile($downloadUrl, $outputPath)

$file = Get-Item -LiteralPath $outputPath
$hash = Get-FileHash -LiteralPath $outputPath -Algorithm SHA256
[PSCustomObject]@{
    Path = $file.FullName
    SizeMB = [math]::Round($file.Length / 1MB, 2)
    SHA256 = $hash.Hash
}
