param(
    [switch]$Editor,
    [switch]$Headless,
    [string]$Scene = ""
)

$projectRoot = "D:\微信小游戏"
$godot = Join-Path $projectRoot "tools\Godot4.7.1Portable\Godot_v4.7.1-stable_win64_sc.exe"

if (-not (Test-Path -LiteralPath $godot)) {
    throw "D 盘便携版 Godot 不存在：$godot"
}

$arguments = @("--path", $projectRoot)
if ($Editor) { $arguments += "--editor" }
if ($Headless) { $arguments += "--headless" }
if (-not [string]::IsNullOrWhiteSpace($Scene)) {
    $arguments += @("--scene", $Scene)
}

& $godot @arguments
exit $LASTEXITCODE
