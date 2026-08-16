Add-Type -AssemblyName System.Drawing

$size = 576
$output = Join-Path (Join-Path $PSScriptRoot '..\wechat_game\assets') '24dian-miniapp-avatar.png'
$output = [System.IO.Path]::GetFullPath($output)
$bitmap = New-Object System.Drawing.Bitmap($size, $size, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
$graphics = [System.Drawing.Graphics]::FromImage($bitmap)
$graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
$graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
$graphics.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
$graphics.TextRenderingHint = [System.Drawing.Text.TextRenderingHint]::AntiAliasGridFit

function Brush($hex) { New-Object System.Drawing.SolidBrush([System.Drawing.ColorTranslator]::FromHtml($hex)) }
function Pen($hex, $width) { New-Object System.Drawing.Pen([System.Drawing.ColorTranslator]::FromHtml($hex), $width) }
function RoundRectPath($x, $y, $w, $h, $r) {
    $path = New-Object System.Drawing.Drawing2D.GraphicsPath
    $d = $r * 2
    $path.AddArc($x, $y, $d, $d, 180, 90)
    $path.AddArc($x + $w - $d, $y, $d, $d, 270, 90)
    $path.AddArc($x + $w - $d, $y + $h - $d, $d, $d, 0, 90)
    $path.AddArc($x, $y + $h - $d, $d, $d, 90, 90)
    $path.CloseFigure()
    return $path
}

$graphics.Clear([System.Drawing.Color]::FromArgb(15, 13, 47))
$outerPath = RoundRectPath 16 16 544 544 94
$graphics.FillPath((Brush '#25205d'), $outerPath)
$graphics.DrawPath((Pen '#50e3ff' 8), $outerPath)
$innerPath = RoundRectPath 38 38 500 500 74
$graphics.FillPath((Brush '#161441'), $innerPath)
$graphics.DrawEllipse((Pen '#8ee8bd' 8), 88, 88, 400, 400)
$graphics.DrawEllipse((Pen '#704c9e' 5), 118, 118, 340, 340)

$symbolFont = New-Object System.Drawing.Font('Segoe UI Symbol', 42, [System.Drawing.FontStyle]::Bold, [System.Drawing.GraphicsUnit]::Pixel)
$graphics.DrawString('+', $symbolFont, (Brush '#50e3ff'), 98, 142)
$graphics.DrawString('-', $symbolFont, (Brush '#ff86b5'), 430, 142)
$graphics.DrawString('x', $symbolFont, (Brush '#ffd166'), 100, 405)
$graphics.DrawString('/', $symbolFont, (Brush '#8ee8bd'), 430, 405)

$mainFont = New-Object System.Drawing.Font('Arial Rounded MT Bold', 190, [System.Drawing.FontStyle]::Bold, [System.Drawing.GraphicsUnit]::Pixel)
$measure = $graphics.MeasureString('24', $mainFont)
$x = ($size - $measure.Width) / 2
$y = ($size - $measure.Height) / 2 - 8
$graphics.DrawString('24', $mainFont, (Brush '#ffffff'), $x + 4, $y + 8)
$graphics.DrawString('24', $mainFont, (Brush '#50e3ff'), $x, $y)
$graphics.FillEllipse((Brush '#ffd166'), 274, 492, 28, 28)

$finalBitmap = New-Object System.Drawing.Bitmap(144, 144, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
$finalGraphics = [System.Drawing.Graphics]::FromImage($finalBitmap)
$finalGraphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
$finalGraphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
$finalGraphics.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
$finalGraphics.DrawImage($bitmap, 0, 0, 144, 144)
$finalBitmap.Save($output, [System.Drawing.Imaging.ImageFormat]::Png)

$finalGraphics.Dispose()
$finalBitmap.Dispose()
$graphics.Dispose()
$bitmap.Dispose()
Write-Output $output
