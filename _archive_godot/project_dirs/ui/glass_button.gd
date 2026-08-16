extends Button
## 玻璃拟态按钮：统一处理高光、悬停发光和点击反馈。

@export var accent_color := Color("#5eeaff")
@export var disabled_glass := false

var base_scale := Vector2.ONE
var hover_amount := 0.0


func _ready() -> void:
	focus_mode = Control.FOCUS_NONE
	mouse_default_cursor_shape = Control.CURSOR_POINTING_HAND
	# 清掉 Godot 默认按钮皮肤，由本脚本绘制玻璃外壳。
	var transparent := StyleBoxFlat.new()
	transparent.bg_color = Color(0, 0, 0, 0)
	transparent.border_width_left = 0
	transparent.border_width_top = 0
	transparent.border_width_right = 0
	transparent.border_width_bottom = 0
	for state in ["normal", "hover", "pressed", "disabled", "focus"]:
		add_theme_stylebox_override(state, transparent)
	base_scale = scale
	mouse_entered.connect(_on_mouse_entered)
	mouse_exited.connect(_on_mouse_exited)
	button_down.connect(_on_button_down)
	button_up.connect(_on_button_up)
	queue_redraw()


func _process(delta: float) -> void:
	var target := 1.0 if is_hovered() and not disabled else 0.0
	hover_amount = move_toward(hover_amount, target, delta * 7.0)
	queue_redraw()


func _draw() -> void:
	if disabled and not disabled_glass:
		return
	var rect := Rect2(Vector2(3, 3), size - Vector2(6, 6))
	if rect.size.x <= 8.0 or rect.size.y <= 8.0:
		return
	var radius := minf(26.0, minf(rect.size.x, rect.size.y) * 0.18)
	var glow_color := Color(accent_color.r, accent_color.g, accent_color.b, 0.16 + hover_amount * 0.18)
	# 外部光晕。
	for index in range(5):
		var spread := float(12 - index * 2)
		var glow_rect := rect.grow(spread)
		draw_style_box(_rounded_box(Color(glow_color.r, glow_color.g, glow_color.b, glow_color.a * (0.12 - index * 0.015)), Color.TRANSPARENT, 0, radius + spread), glow_rect)

	# 玻璃主体、底部颜色和高亮描边。
	var glass_color := Color(accent_color.r, accent_color.g, accent_color.b, 0.20 + hover_amount * 0.10)
	draw_style_box(_rounded_box(glass_color, Color(1.0, 1.0, 1.0, 0.20), 2, radius), rect)
	var lower_rect := Rect2(rect.position + Vector2(0, rect.size.y * 0.56), Vector2(rect.size.x, rect.size.y * 0.44))
	draw_style_box(_rounded_box(Color(0.08, 0.06, 0.24, 0.14), Color.TRANSPARENT, 0, radius), lower_rect)

	# 上半部分的圆形图标气泡，配合按钮自身的第一行符号。
	var icon_center := Vector2(rect.get_center().x, rect.position.y + minf(82.0, rect.size.y * 0.34))
	var icon_radius := minf(42.0, rect.size.x * 0.22)
	draw_circle(icon_center, icon_radius + 4.0, Color(1.0, 1.0, 1.0, 0.18))
	draw_circle(icon_center, icon_radius, Color(accent_color.r, accent_color.g, accent_color.b, 0.38 + hover_amount * 0.12))
	draw_arc(icon_center, icon_radius, PI * 0.12, PI * 1.08, 32, Color(1, 1, 1, 0.48), 2.0, true)

	# 左上角白色反光。
	var shine := PackedVector2Array([
		Vector2(rect.position.x + radius * 0.6, rect.position.y + 8),
		Vector2(rect.position.x + rect.size.x * 0.38, rect.position.y + 8),
		Vector2(rect.position.x + rect.size.x * 0.28, rect.position.y + 13),
		Vector2(rect.position.x + radius * 0.5, rect.position.y + 13),
	])
	draw_colored_polygon(shine, Color(1, 1, 1, 0.42))
	draw_line(Vector2(rect.position.x + rect.size.x - 30, rect.position.y + 14), Vector2(rect.position.x + rect.size.x - 21, rect.position.y + 22), Color(1, 1, 1, 0.25), 3.0, true)


func _rounded_box(bg: Color, border: Color, border_width: int, radius: float) -> StyleBoxFlat:
	var box := StyleBoxFlat.new()
	box.bg_color = bg
	box.border_color = border
	box.border_width_left = border_width
	box.border_width_top = border_width
	box.border_width_right = border_width
	box.border_width_bottom = border_width
	box.corner_radius_top_left = int(radius)
	box.corner_radius_top_right = int(radius)
	box.corner_radius_bottom_left = int(radius)
	box.corner_radius_bottom_right = int(radius)
	return box


func _on_mouse_entered() -> void:
	var tween := create_tween()
	tween.set_trans(Tween.TRANS_BACK).set_ease(Tween.EASE_OUT)
	tween.tween_property(self, "scale", base_scale * 1.025, 0.12)


func _on_mouse_exited() -> void:
	var tween := create_tween()
	tween.set_trans(Tween.TRANS_QUAD).set_ease(Tween.EASE_OUT)
	tween.tween_property(self, "scale", base_scale, 0.14)


func _on_button_down() -> void:
	var tween := create_tween()
	tween.set_trans(Tween.TRANS_QUAD).set_ease(Tween.EASE_OUT)
	tween.tween_property(self, "scale", base_scale * 0.965, 0.07)


func _on_button_up() -> void:
	var tween := create_tween()
	tween.set_trans(Tween.TRANS_BACK).set_ease(Tween.EASE_OUT)
	tween.tween_property(self, "scale", base_scale * (1.025 if is_hovered() else 1.0), 0.12)
