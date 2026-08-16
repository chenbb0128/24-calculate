extends Control
class_name CartoonDecor

## 纯 Godot 绘制的原创儿童数学王国背景，不依赖外部图片素材。
## 只使用几何图形、数字涂鸦和抽象星球，方便后续迁移到微信小游戏。

var bg := Color("#064633")
var surface := Color("#0b5a42")
var chalk := Color("#b9d8c5")
var accent := Color("#ffd34d")
var gold := Color("#e8b544")
var elapsed := 0.0


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	set_process(true)


func _process(delta: float) -> void:
	elapsed += delta
	queue_redraw()


func configure(next_bg: Color, next_surface: Color, next_chalk: Color, next_accent: Color, next_gold: Color) -> void:
	bg = next_bg
	surface = next_surface
	chalk = next_chalk
	accent = next_accent
	gold = next_gold
	queue_redraw()


func _draw() -> void:
	var width := size.x
	var height := size.y
	if width <= 1.0 or height <= 1.0:
		return

	# 柔和的梦幻色块：只放在屏幕边缘，中间区域留给按钮和题目。
	var haze := chalk
	haze.a = 0.07
	draw_circle(Vector2(width * 0.08, height * 0.35), 160.0, haze)
	draw_circle(Vector2(width * 0.92, height * 0.56), 190.0, haze)
	draw_circle(Vector2(width * 0.26, height * 0.93), 130.0, haze)

	# 云朵和星星让画面更像儿童数学乐园。
	_draw_cloud(Vector2(width * 0.10, height * 0.46), 0.9, chalk)
	_draw_cloud(Vector2(width * 0.89, height * 0.30), 0.72, chalk)
	_draw_star(Vector2(width * 0.08, height * 0.18), 17.0, 7.0, accent)
	_draw_star(Vector2(width * 0.92, height * 0.12), 12.0, 5.0, gold)
	_draw_star(Vector2(width * 0.91, height * 0.83), 19.0, 8.0, accent)
	_draw_star(Vector2(width * 0.07, height * 0.78), 11.0, 5.0, gold)

	# 边缘数字涂鸦，降低透明度，丰富画面但不抢题目注意力。
	var doodle := chalk
	doodle.a = 0.18
	draw_string(ThemeDB.fallback_font, Vector2(width * 0.04, height * 0.60), "+", HORIZONTAL_ALIGNMENT_LEFT, -1, 28, doodle)
	draw_string(ThemeDB.fallback_font, Vector2(width * 0.91, height * 0.47), "×", HORIZONTAL_ALIGNMENT_LEFT, -1, 26, doodle)
	draw_string(ThemeDB.fallback_font, Vector2(width * 0.06, height * 0.91), "24", HORIZONTAL_ALIGNMENT_LEFT, -1, 20, doodle)
	draw_string(ThemeDB.fallback_font, Vector2(width * 0.84, height * 0.69), "÷", HORIZONTAL_ALIGNMENT_LEFT, -1, 22, doodle)

	# 右下角原创小星球，做成低透明度背景装饰。
	_draw_planet(Vector2(width * 0.87, height * 0.88), minf(width, height) * 0.105)

	# 边缘小圆点带有轻微呼吸位移，增加生命感。
	var bob := sin(elapsed * 1.8) * 3.0
	var bubble := accent
	bubble.a = 0.22
	draw_circle(Vector2(width * 0.18, height * 0.12 + bob), 6.0, bubble)
	draw_circle(Vector2(width * 0.78, height * 0.20 - bob), 4.0, bubble)


func _draw_mascot(center: Vector2) -> void:
	var shadow := surface.darkened(0.28)
	shadow.a = 0.72
	draw_circle(center + Vector2(0, 5), 37.0, shadow)

	var body := StyleBoxFlat.new()
	body.bg_color = accent
	body.border_color = gold
	body.border_width_left = 3
	body.border_width_top = 3
	body.border_width_right = 3
	body.border_width_bottom = 3
	body.corner_radius_top_left = 15
	body.corner_radius_top_right = 15
	body.corner_radius_bottom_left = 15
	body.corner_radius_bottom_right = 15
	draw_style_box(body, Rect2(center - Vector2(32, 32), Vector2(64, 64)))

	var eye_color := bg.darkened(0.22)
	draw_circle(center + Vector2(-12, -4), 4.5, eye_color)
	draw_circle(center + Vector2(12, -4), 4.5, eye_color)
	draw_arc(center + Vector2(0, 5), 12.0, 0.2, 2.94, 18, eye_color, 2.0, true)
	draw_string(ThemeDB.fallback_font, center + Vector2(-19, 24), "24", HORIZONTAL_ALIGNMENT_LEFT, -1, 16, bg.darkened(0.18))


func _draw_cloud(center: Vector2, scale: float, color: Color) -> void:
	var cloud := color
	cloud.a = 0.18
	draw_circle(center + Vector2(-30, 5) * scale, 20.0 * scale, cloud)
	draw_circle(center + Vector2(0, -6) * scale, 28.0 * scale, cloud)
	draw_circle(center + Vector2(28, 5) * scale, 19.0 * scale, cloud)
	draw_rect(Rect2(center + Vector2(-48, 3) * scale, Vector2(96, 25) * scale), cloud)


func _draw_planet(center: Vector2, radius: float) -> void:
	var planet := surface.lightened(0.15)
	planet.a = 0.25
	draw_circle(center, radius, planet)
	var ring := accent
	ring.a = 0.22
	draw_arc(center, radius * 1.28, 0.18, PI - 0.18, 48, ring, maxf(4.0, radius * 0.05), true)
	draw_circle(center + Vector2(-radius * 0.28, -radius * 0.18), radius * 0.12, ring)
	draw_circle(center + Vector2(radius * 0.25, radius * 0.30), radius * 0.08, ring)


func _draw_star(center: Vector2, outer_radius: float, inner_radius: float, color: Color) -> void:
	var points := PackedVector2Array()
	for index in range(10):
		var angle := -PI / 2.0 + float(index) * PI / 5.0
		var radius := outer_radius if index % 2 == 0 else inner_radius
		points.append(center + Vector2(cos(angle), sin(angle)) * radius)
	draw_colored_polygon(points, color)
