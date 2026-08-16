extends Control
class_name TimerBadge

## 游戏页专用倒计时牌：时间、横向进度条、紧迫颜色和轻微脉冲。

var time_left := 0.0
var max_time := 120.0
var endless := false
var surface := Color("#0b5a42")
var surface_dark := Color("#073b2d")
var text_color := Color("#fff9e9")
var muted := Color("#b9d8c5")
var accent := Color("#ffd34d")
var warning := Color("#ffe08a")
var danger := Color("#ff9b7d")


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	custom_minimum_size = Vector2(144, 52)


func configure_palette(next_surface: Color, next_text: Color, next_muted: Color, next_accent: Color, next_warning: Color, next_danger: Color) -> void:
	surface = next_surface
	surface_dark = next_surface.darkened(0.28)
	text_color = next_text
	muted = next_muted
	accent = next_accent
	warning = next_warning
	danger = next_danger
	queue_redraw()


func set_timer(next_time_left: float, next_max_time: float, is_endless: bool) -> void:
	time_left = maxf(0.0, next_time_left)
	max_time = maxf(0.1, next_max_time)
	endless = is_endless
	queue_redraw()


func _draw() -> void:
	if size.x <= 2.0 or size.y <= 2.0:
		return

	var ratio := clampf(time_left / max_time, 0.0, 1.0)
	var tone := accent
	if ratio <= 0.5:
		tone = warning
	if ratio <= 0.2:
		tone = danger

	var panel := StyleBoxFlat.new()
	panel.bg_color = Color(1.0, 1.0, 1.0, 0.08)
	panel.border_color = Color(tone.r, tone.g, tone.b, 0.78)
	panel.border_width_left = 2
	panel.border_width_top = 2
	panel.border_width_right = 2
	panel.border_width_bottom = 2
	panel.corner_radius_top_left = 17
	panel.corner_radius_top_right = 17
	panel.corner_radius_bottom_left = 17
	panel.corner_radius_bottom_right = 17
	panel.shadow_color = Color(0.02, 0.01, 0.12, 0.58)
	panel.shadow_size = 9
	panel.shadow_offset = Vector2(0, 2)
	var safe_width := maxf(4.0, size.x - 2.0)
	var safe_height := maxf(4.0, size.y - 4.0)
	draw_style_box(panel, Rect2(Vector2(1, 2), Vector2(safe_width, safe_height)))

	var title := "能量" if endless else "剩余时间"
	var title_pos := Vector2(12, 19)
	draw_string(ThemeDB.fallback_font, title_pos + Vector2(1, 1), title, HORIZONTAL_ALIGNMENT_LEFT, -1, 11, Color(0.03, 0.02, 0.12, 0.95))
	draw_string(ThemeDB.fallback_font, title_pos, title, HORIZONTAL_ALIGNMENT_LEFT, -1, 11, Color(0.9, 0.9, 1.0, 0.86))
	var value_text := "%.1f" % time_left
	# draw_string 的 position 是文字基线，不是文字区域左上角；数值基线放到牌内，避免跑到边框外。
	var value_rect := Rect2(62, 28, maxf(10.0, size.x - 74.0), 26)
	draw_string(ThemeDB.fallback_font, value_rect.position + Vector2(1, 2), value_text, HORIZONTAL_ALIGNMENT_RIGHT, value_rect.size.x, 22, Color(0.03, 0.02, 0.12, 0.98))
	draw_string(ThemeDB.fallback_font, value_rect.position, value_text, HORIZONTAL_ALIGNMENT_RIGHT, value_rect.size.x, 22, text_color)

	var track := Rect2(12, size.y - 13, size.x - 24, 6)
	var track_style := StyleBoxFlat.new()
	track_style.bg_color = Color(0.08, 0.06, 0.24, 0.76)
	track_style.corner_radius_top_left = 4
	track_style.corner_radius_top_right = 4
	track_style.corner_radius_bottom_left = 4
	track_style.corner_radius_bottom_right = 4
	draw_style_box(track_style, track)
	var fill := track
	fill.size.x = maxf(6.0, track.size.x * ratio)
	var fill_style := StyleBoxFlat.new()
	fill_style.bg_color = tone
	fill_style.corner_radius_top_left = 4
	fill_style.corner_radius_top_right = 4
	fill_style.corner_radius_bottom_left = 4
	fill_style.corner_radius_bottom_right = 4
	# 只绘制进度条本体，不额外绘制端点圆点或右上角脉冲点，
	# 避免倒计时接近结束时出现像素点误叠到数字/边框上。
	if fill.size.x > 0.5:
		draw_style_box(fill_style, fill)
