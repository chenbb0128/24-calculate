extends Control
## 主菜单背景：用 Godot 原生绘制实现星云、渐变和底部波浪光影。
## 不依赖外部图片，方便手机竖屏和后续微信小游戏迁移。

var pulse := 0.0


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	queue_redraw()


func _process(delta: float) -> void:
	pulse += delta
	queue_redraw()


func _draw() -> void:
	var size := get_rect().size
	if size.x <= 0.0 or size.y <= 0.0:
		return

	# 顶部深蓝到下方紫蓝的纵向渐变。
	var bands := 40
	var band_height := size.y / float(bands)
	for index in range(bands):
		var ratio := float(index) / float(bands - 1)
		var top_color := Color("#1e1b4b")
		var bottom_color := Color("#312e81")
		var band_color := top_color.lerp(bottom_color, ratio)
		draw_rect(Rect2(0, band_height * index, size.x, band_height + 1.0), band_color)

	# 蓝色和紫色星云，叠加透明圆形会形成柔和的发光层。
	_draw_glow(Vector2(size.x * 0.18, size.y * 0.28), 250.0, Color("#155cc0"), 0.16)
	_draw_glow(Vector2(size.x * 0.76, size.y * 0.43), 300.0, Color("#8b28d7"), 0.18)
	_draw_glow(Vector2(size.x * 0.48, size.y * 0.63), 260.0, Color("#1d8de4"), 0.11)
	_draw_glow(Vector2(size.x * 0.55, size.y * 0.82), 340.0, Color("#da54d6"), 0.13)

	# 底部两层波浪，让主菜单不再像一张平面色块。
	var wave_back := PackedVector2Array([
		Vector2(0, size.y * 0.88),
		Vector2(size.x * 0.16, size.y * 0.84),
		Vector2(size.x * 0.34, size.y * 0.90),
		Vector2(size.x * 0.56, size.y * 0.85),
		Vector2(size.x * 0.78, size.y * 0.91),
		Vector2(size.x, size.y * 0.86),
		Vector2(size.x, size.y),
		Vector2(0, size.y),
	])
	draw_colored_polygon(wave_back, Color(0.18, 0.12, 0.48, 0.54))

	var wave_front := PackedVector2Array([
		Vector2(0, size.y * 0.95),
		Vector2(size.x * 0.17, size.y * 0.91),
		Vector2(size.x * 0.37, size.y * 0.96),
		Vector2(size.x * 0.59, size.y * 0.91),
		Vector2(size.x * 0.80, size.y * 0.96),
		Vector2(size.x, size.y * 0.92),
		Vector2(size.x, size.y),
		Vector2(0, size.y),
	])
	draw_colored_polygon(wave_front, Color(0.05, 0.08, 0.32, 0.72))

	# 中央一束微弱的星尘光，强调 Logo 所在区域。
	var sparkle_center := Vector2(size.x * 0.57, size.y * 0.32)
	for ring in range(4):
		var radius := 28.0 + float(ring) * 18.0
		var alpha := 0.08 - float(ring) * 0.014
		draw_circle(sparkle_center, radius, Color(0.25, 0.7, 1.0, alpha))


func _draw_glow(center: Vector2, radius: float, color: Color, strength: float) -> void:
	for layer in range(8, 0, -1):
		var ratio := float(layer) / 8.0
		var layer_radius := radius * ratio
		var alpha := strength * (1.0 - ratio) * 0.55
		draw_circle(center, layer_radius, Color(color.r, color.g, color.b, alpha))
