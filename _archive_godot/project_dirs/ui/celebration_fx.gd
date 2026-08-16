extends Control
class_name CelebrationFx

## 轻量原创特效层：全部使用 Godot CanvasItem 绘制，避免外部素材拉伸、穿模和版权风险。

var accent := Color("#ffd85b")
var blue := Color("#62c8f2")
var pink := Color("#f39adf")
var green := Color("#7ee6a8")
var text_color := Color("#fffdfb")
var particles: Array = []
var rings: Array = []
var sparks: Array = []
var toast_text := ""
var toast_color := Color.WHITE
var toast_time := 0.0
var toast_total := 0.0
var active := false


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	set_process(false)


func configure_palette(next_accent: Color, next_blue: Color, next_pink: Color, next_green: Color, next_text: Color) -> void:
	accent = next_accent
	blue = next_blue
	pink = next_pink
	green = next_green
	text_color = next_text
	queue_redraw()


func tap(center: Vector2, color: Color = Color.WHITE) -> void:
	_add_ring(center, color, 9.0, 34.0, 0.24)
	for index in range(5):
		var angle := TAU * float(index) / 5.0 - PI / 2.0
		_add_particle(center, Vector2(cos(angle), sin(angle)) * (38.0 + index * 3.0), color, 3.0, 0.28, "dot")
	_start()


func merge(first_center: Vector2, second_center: Vector2) -> void:
	var center := (first_center + second_center) * 0.5
	_add_ring(center, accent, 12.0, 92.0, 0.42)
	_add_ring(center, blue, 5.0, 58.0, 0.30)
	for index in range(10):
		var angle := TAU * float(index) / 10.0
		_add_particle(center, Vector2(cos(angle), sin(angle)) * (70.0 + (index % 3) * 12.0), accent if index % 2 == 0 else blue, 4.0, 0.45, "star")
	_start()


func success(center: Vector2) -> void:
	_add_ring(center, accent, 16.0, 180.0, 0.72)
	_add_ring(center, pink, 28.0, 260.0, 0.90)
	var colors := [accent, blue, pink, green, Color("#ffffff")]
	for index in range(34):
		var angle := TAU * float(index) / 34.0
		var speed := 80.0 + float((index * 17) % 120)
		_add_particle(center, Vector2(cos(angle), sin(angle)) * speed, colors[index % colors.size()], 4.0 + float(index % 3), 1.05, "confetti")
	toast("答对啦！", accent, 1.0)
	_start()


func locked(center: Vector2) -> void:
	for index in range(6):
		var angle := TAU * float(index) / 6.0
		_add_particle(center, Vector2(cos(angle), sin(angle)) * 22.0, pink, 3.0, 0.34, "dot")
	toast("再完成一些关卡就能解锁", pink, 1.15)
	_start()


func reward(center: Vector2, amount: int) -> void:
	_add_ring(center, accent, 8.0, 75.0, 0.4)
	for index in range(12):
		var angle := TAU * float(index) / 12.0
		_add_particle(center, Vector2(cos(angle), sin(angle)) * (40.0 + index * 2.0), accent, 3.5, 0.62, "coin")
	toast("+%d 金币" % amount, accent, 0.9)
	_start()


func toast(message: String, color: Color, duration: float = 1.0) -> void:
	toast_text = message
	toast_color = color
	toast_total = maxf(0.2, duration)
	toast_time = toast_total
	_start()


func _add_ring(center: Vector2, color: Color, start_radius: float, end_radius: float, duration: float) -> void:
	rings.append({"center": center, "color": color, "radius": start_radius, "start": start_radius, "end": end_radius, "life": duration, "total": duration})


func _add_particle(center: Vector2, velocity: Vector2, color: Color, size_value: float, duration: float, shape: String) -> void:
	particles.append({"position": center, "velocity": velocity, "color": color, "size": size_value, "life": duration, "total": duration, "shape": shape, "rotation": 0.0, "spin": 2.0 + float(particles.size() % 5)})


func _start() -> void:
	active = true
	set_process(true)
	queue_redraw()


func _process(delta: float) -> void:
	var has_work := false
	for index in range(particles.size() - 1, -1, -1):
		var particle: Dictionary = particles[index]
		particle["life"] = float(particle["life"]) - delta
		if float(particle["life"]) <= 0.0:
			particles.remove_at(index)
			continue
		particle["position"] = particle["position"] + particle["velocity"] * delta
		particle["velocity"] = particle["velocity"] * (1.0 - minf(0.9, delta * 1.6)) + Vector2(0.0, 48.0) * delta
		particle["rotation"] = float(particle["rotation"]) + float(particle["spin"]) * delta
		particles[index] = particle
		has_work = true
	for index in range(rings.size() - 1, -1, -1):
		var ring: Dictionary = rings[index]
		ring["life"] = float(ring["life"]) - delta
		if float(ring["life"]) <= 0.0:
			rings.remove_at(index)
			continue
		var progress := 1.0 - float(ring["life"]) / float(ring["total"])
		ring["radius"] = lerpf(float(ring["start"]), float(ring["end"]), progress)
		rings[index] = ring
		has_work = true
	if toast_time > 0.0:
		toast_time = maxf(0.0, toast_time - delta)
		has_work = true
	active = has_work or not particles.is_empty() or not rings.is_empty() or toast_time > 0.0
	if not active:
		set_process(false)
	queue_redraw()


func _draw() -> void:
	for ring in rings:
		var alpha := clampf(float(ring["life"]) / float(ring["total"]), 0.0, 1.0)
		var color: Color = ring["color"]
		color.a = alpha * 0.72
		draw_arc(ring["center"], float(ring["radius"]), 0.0, TAU, 48, color, 3.0, true)
	for particle in particles:
		var alpha := clampf(float(particle["life"]) / float(particle["total"]), 0.0, 1.0)
		var color: Color = particle["color"]
		color.a = alpha
		var center: Vector2 = particle["position"]
		var size_value := float(particle["size"]) * (0.7 + alpha * 0.3)
		match str(particle["shape"]):
			"star":
				_draw_star(center, size_value, size_value * 0.38, color, float(particle["rotation"]))
			"confetti":
				var points := PackedVector2Array([center + Vector2(-size_value, -size_value * 0.45), center + Vector2(size_value, -size_value * 0.45), center + Vector2(size_value, size_value * 0.45), center + Vector2(-size_value, size_value * 0.45)])
				draw_set_transform(center, float(particle["rotation"]), Vector2.ONE)
				draw_colored_polygon(PackedVector2Array([Vector2(-size_value, -size_value * 0.45), Vector2(size_value, -size_value * 0.45), Vector2(size_value, size_value * 0.45), Vector2(-size_value, size_value * 0.45)]), color)
				draw_set_transform(Vector2.ZERO, 0.0, Vector2.ONE)
			"coin":
				draw_circle(center, size_value, color)
				draw_circle(center, size_value * 0.48, Color(color, alpha * 0.5))
			_:
				draw_circle(center, size_value, color)
	if toast_time > 0.0 and not toast_text.is_empty():
		var fade := minf(1.0, toast_time * 5.0)
		var toast_width := minf(size.x - 56.0, maxf(210.0, 24.0 + ThemeDB.fallback_font.get_string_size(toast_text, HORIZONTAL_ALIGNMENT_LEFT, -1, 20).x))
		var toast_rect := Rect2(Vector2((size.x - toast_width) * 0.5, size.y * 0.48), Vector2(toast_width, 52.0))
		var panel := StyleBoxFlat.new()
		var panel_color := Color("#302762")
		panel_color.a = fade * 0.94
		panel.bg_color = panel_color
		panel.border_color = Color(toast_color, fade)
		panel.border_width_left = 2
		panel.border_width_top = 2
		panel.border_width_right = 2
		panel.border_width_bottom = 2
		panel.corner_radius_top_left = 26
		panel.corner_radius_top_right = 26
		panel.corner_radius_bottom_left = 26
		panel.corner_radius_bottom_right = 26
		draw_style_box(panel, toast_rect)
		var draw_color := Color(text_color, fade)
		draw_string(ThemeDB.fallback_font, Vector2(toast_rect.position.x, toast_rect.position.y + 33.0), toast_text, HORIZONTAL_ALIGNMENT_CENTER, toast_rect.size.x, 20, draw_color)


func _draw_star(center: Vector2, outer_radius: float, inner_radius: float, color: Color, rotation: float) -> void:
	var points := PackedVector2Array()
	for index in range(10):
		var angle := rotation - PI / 2.0 + float(index) * PI / 5.0
		var radius := outer_radius if index % 2 == 0 else inner_radius
		points.append(center + Vector2(cos(angle), sin(angle)) * radius)
	draw_colored_polygon(points, color)
