extends Control
## 24点挑战新版主菜单。
## 这里专门负责主菜单展示和入口转发，具体玩法仍由 mvp.gd 负责。

const SaveServiceScript = preload("res://services/save_service.gd")
const LevelCatalogScript = preload("res://core/level_catalog.gd")
const TaskServiceScript = preload("res://services/task_service.gd")
const PlayerStatsScript = preload("res://services/player_stats.gd")

@onready var stars_container: Control = $StarsContainer
@onready var float_nums: Control = $FloatNums
@onready var logo_24: Label = $Content/LogoArea/Logo24
@onready var coin_num: Label = $Content/TopBar/CoinPanel/CoinHBox/CoinNum
@onready var progress_label: Label = $Content/ProgressPanel/ProgressVBox/ProgressLabel
@onready var progress_bar: ProgressBar = $Content/ProgressPanel/ProgressVBox/ProgressBar
@onready var daily_btn: Button = $Content/ModeRow/DailyBtn
@onready var level_btn: Button = $Content/LevelBtn
@onready var level_progress: Label = $Content/LevelProgress
@onready var level_title: Label = $Content/LevelBtn/LevelTitle
@onready var friend_title: Label = $Content/FriendBtn/FriendTitle
@onready var endless_title: Label = $Content/EndlessBtn/EndlessTitle
@onready var daily_title: Label = $Content/ModeRow/DailyBtn/DailyTitle
@onready var daily_status: Label = $Content/ModeRow/DailyBtn/DailyStatus
@onready var task_title: Label = $Content/ModeRow/TaskBtn/TaskTitle
@onready var more_title: Label = $Content/ModeRow/MoreBtn/MoreTitle
@onready var game_controller: Control = $GameController
@onready var menu_content: Control = $Content

var progress: Dictionary = {}
var total_levels := 1
var unlocked_level := 1
var daily_done := false
var settings_popup: PanelContainer
var tasks_popup: PanelContainer
var more_popup: PanelContainer
var toast_panel: PanelContainer
var toast_label: Label
var toast_tween: Tween
var menu_music_toggle: Button
var menu_sfx_toggle: Button
var menu_track_button: Button
var menu_music_slider: HSlider
var menu_sfx_slider: HSlider

var stars: Array = []
var float_num_labels: Array = []
var logo_time := 0.0

const STAR_COUNT := 60
const FLOAT_NUM_COUNT := 6


func _ready() -> void:
	randomize()
	if is_instance_valid(game_controller) and game_controller.has_signal("home_requested"):
		game_controller.home_requested.connect(_on_game_home_requested)
		game_controller.visible = false
	menu_content.visible = true
	for decorative in [level_title, friend_title, endless_title, daily_title, daily_status, task_title, more_title, level_progress]:
		if is_instance_valid(decorative):
			decorative.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_load_progress()
	_generate_stars()
	_generate_float_nums()
	_build_menu_popups()
	_update_ui()


func _process(delta: float) -> void:
	logo_time += delta
	if is_instance_valid(logo_24):
		var glow := 0.42 + 0.28 * sin(logo_time * 2.0)
		logo_24.add_theme_color_override("font_shadow_color", Color(0.29, 0.184, 0.969, glow))

	for star_data in stars:
		var star: ColorRect = star_data["node"]
		var star_time: float = float(star_data["time"]) + delta
		star_data["time"] = star_time
		star.color.a = 0.25 + 0.75 * abs(sin(star_time * float(star_data["speed"])))

	for num_data in float_num_labels:
		var label: Label = num_data["node"]
		var num_time: float = float(num_data["time"]) + delta
		num_data["time"] = num_time
		var speed: float = float(num_data["speed"])
		label.position.y = float(num_data["base_y"]) + sin(num_time * speed) * 18.0
		label.position.x = float(num_data["base_x"]) + cos(num_time * speed * 0.6) * 10.0


func _load_progress() -> void:
	progress = SaveServiceScript.load_progress()
	total_levels = maxi(1, LevelCatalogScript.all().size())
	# 存档记录的是已完成的 0-based 关卡数量，所以首页展示下一关。
	unlocked_level = clampi(int(progress.get("unlocked_level", 0)) + 1, 1, total_levels)
	daily_done = SaveServiceScript.is_daily_completed(progress, SaveServiceScript.today_key())


func _update_ui() -> void:
	if not is_instance_valid(coin_num):
		return
	coin_num.text = str(int(progress.get("coins", 0)))
	progress_label.text = "闯关进度 · 已解锁 1-%d 关" % unlocked_level
	progress_bar.value = float(unlocked_level) / float(total_levels) * 100.0
	level_btn.text = ""
	level_title.text = "闯关模式"
	level_progress.text = "1-%d" % unlocked_level

	if daily_done:
		daily_btn.text = ""
		daily_title.text = "今日挑战"
		daily_status.text = "已完成"
		daily_btn.modulate.a = 0.68
	else:
		daily_btn.text = ""
		daily_title.text = "每日挑战"
		daily_status.text = "每日更新"
		daily_btn.modulate.a = 1.0


func _generate_stars() -> void:
	if not is_instance_valid(stars_container):
		return
	var viewport_size := get_viewport_rect().size
	for i in range(STAR_COUNT):
		var star := ColorRect.new()
		var size := randf_range(2.0, 5.0)
		star.custom_minimum_size = Vector2(size, size)
		star.size = Vector2(size, size)
		star.position = Vector2(randf() * viewport_size.x, randf() * viewport_size.y)
		star.color = Color(0.82, 0.9, 1.0, randf_range(0.25, 1.0))
		star.mouse_filter = Control.MOUSE_FILTER_IGNORE
		stars_container.add_child(star)
		stars.append({"node": star, "time": randf() * TAU, "speed": randf_range(1.0, 3.5)})


func _generate_float_nums() -> void:
	if not is_instance_valid(float_nums):
		return
	var viewport_size := get_viewport_rect().size
	var symbols: Array[String] = ["2", "4", "6", "8", "+", "×"]
	var positions: Array[Vector2] = [
		Vector2(viewport_size.x * 0.06, viewport_size.y * 0.18),
		Vector2(viewport_size.x * 0.84, viewport_size.y * 0.30),
		Vector2(viewport_size.x * 0.08, viewport_size.y * 0.50),
		Vector2(viewport_size.x * 0.86, viewport_size.y * 0.62),
		Vector2(viewport_size.x * 0.16, viewport_size.y * 0.76),
		Vector2(viewport_size.x * 0.80, viewport_size.y * 0.12),
	]
	for i in range(FLOAT_NUM_COUNT):
		var label := Label.new()
		label.text = symbols[i]
		label.add_theme_font_size_override("font_size", randi_range(52, 80))
		label.add_theme_color_override("font_color", Color(1, 1, 1, 0.06))
		label.position = positions[i]
		label.mouse_filter = Control.MOUSE_FILTER_IGNORE
		float_nums.add_child(label)
		float_num_labels.append({
			"node": label,
			"base_x": positions[i].x,
			"base_y": positions[i].y,
			"time": randf() * TAU,
			"speed": randf_range(0.4, 1.0),
		})


func _show_controller() -> bool:
	if not is_instance_valid(game_controller):
		return false
	_load_progress()
	menu_content.visible = false
	game_controller.visible = true
	return true


func _on_setting_pressed() -> void:
	_ensure_popups()
	_hide_menu_popups(settings_popup)
	settings_popup.visible = not settings_popup.visible
	if settings_popup.visible:
		_refresh_menu_audio_ui()


func _on_level_pressed() -> void:
	if _show_controller() and game_controller.has_method("_show_menu"):
		game_controller.call("_show_menu")


func _on_friend_pressed() -> void:
	if _show_controller() and game_controller.has_method("_show_friend_lobby"):
		game_controller.call("_show_friend_lobby")


func _on_endless_pressed() -> void:
	if _show_controller() and game_controller.has_method("_start_endless_mode"):
		game_controller.call("_start_endless_mode")


func _on_daily_pressed() -> void:
	if daily_done:
		_show_menu_toast("今日挑战已完成\n明日零点更新")
		return
	if _show_controller() and game_controller.has_method("_start_daily_challenge"):
		game_controller.call("_start_daily_challenge")


func _on_task_pressed() -> void:
	_ensure_popups()
	_hide_menu_popups(tasks_popup)
	tasks_popup.visible = not tasks_popup.visible
	if tasks_popup.visible:
		_refresh_menu_tasks_ui()


func _on_more_pressed() -> void:
	_ensure_popups()
	_hide_menu_popups(more_popup)
	more_popup.visible = not more_popup.visible


func _on_more_shop_pressed() -> void:
	_open_controller_page("_show_shop")


func _on_more_achievements_pressed() -> void:
	_open_controller_page("_show_achievements")


func _on_more_leaderboard_pressed() -> void:
	_open_controller_page("_show_leaderboard")


func _on_more_stats_pressed() -> void:
	_open_controller_page("_show_stats")


func _open_controller_page(method_name: String) -> void:
	_hide_menu_popups()
	if _show_controller() and game_controller.has_method(method_name):
		game_controller.call(method_name)


func _build_menu_popups() -> void:
	settings_popup = _make_popup("设置", 390.0, 430.0)
	var settings_content := settings_popup.get_node("Content") as VBoxContainer
	menu_music_toggle = _make_popup_button("背景音乐：开启")
	menu_music_toggle.pressed.connect(_on_menu_music_toggle_pressed)
	settings_content.add_child(menu_music_toggle)
	var music_label := _make_popup_label("背景音乐音量", 13, Color(0.82, 0.84, 1.0))
	settings_content.add_child(music_label)
	menu_music_slider = _make_popup_slider()
	menu_music_slider.value_changed.connect(_on_menu_music_volume_changed)
	settings_content.add_child(menu_music_slider)
	menu_track_button = _make_popup_button("更换音乐")
	menu_track_button.pressed.connect(_on_menu_track_pressed)
	settings_content.add_child(menu_track_button)
	menu_sfx_toggle = _make_popup_button("按键音效：开启")
	menu_sfx_toggle.pressed.connect(_on_menu_sfx_toggle_pressed)
	settings_content.add_child(menu_sfx_toggle)
	var sfx_label := _make_popup_label("按键音效音量", 13, Color(0.82, 0.84, 1.0))
	settings_content.add_child(sfx_label)
	menu_sfx_slider = _make_popup_slider()
	menu_sfx_slider.value_changed.connect(_on_menu_sfx_volume_changed)
	settings_content.add_child(menu_sfx_slider)
	var settings_note := _make_popup_label("设置会自动保存，进入微信小游戏后也会继续使用。", 12, Color(0.72, 0.75, 0.92))
	settings_note.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	settings_content.add_child(settings_note)

	tasks_popup = _make_popup("每日任务", 460.0, 560.0, true)
	var tasks_content := tasks_popup.get_node("Content") as VBoxContainer
	var tasks_note := _make_popup_label("每日零点刷新，完成后奖励会自动到账。", 13, Color(0.82, 0.84, 1.0))
	tasks_note.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	tasks_content.add_child(tasks_note)
	for task_id in ["campaign_clear", "endless_questions", "combo"]:
		var task_row := _make_popup_label("", 15, Color.WHITE)
		task_row.name = "Task_%s" % task_id
		task_row.custom_minimum_size = Vector2(0, 56)
		task_row.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
		task_row.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		tasks_content.add_child(task_row)
	var weekly_title := _make_popup_label("本周任务", 15, Color("#8fe7ff"))
	weekly_title.name = "WeeklyTitle"
	tasks_content.add_child(weekly_title)
	for task_id in ["weekly_campaign", "weekly_daily", "weekly_endless", "weekly_friend"]:
		var task_row := _make_popup_label("", 14, Color.WHITE)
		task_row.name = "Weekly_%s" % task_id
		task_row.custom_minimum_size = Vector2(0, 44)
		task_row.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
		task_row.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		tasks_content.add_child(task_row)

	more_popup = _make_popup("更多功能", 340.0, 370.0, true)
	var more_content := more_popup.get_node("Content") as VBoxContainer
	var more_note := _make_popup_label("选择一个功能继续。", 13, Color(0.82, 0.84, 1.0))
	more_content.add_child(more_note)
	var shop_button := _make_popup_button("主题商店")
	shop_button.pressed.connect(_on_more_shop_pressed)
	more_content.add_child(shop_button)
	var achievements_button := _make_popup_button("成就徽章")
	achievements_button.pressed.connect(_on_more_achievements_pressed)
	more_content.add_child(achievements_button)
	var leaderboard_button := _make_popup_button("排行榜")
	leaderboard_button.pressed.connect(_on_more_leaderboard_pressed)
	more_content.add_child(leaderboard_button)
	var stats_button := _make_popup_button("挑战记录")
	stats_button.pressed.connect(_on_more_stats_pressed)
	more_content.add_child(stats_button)

	toast_panel = _make_toast_panel()


func _ensure_popups() -> void:
	if not is_instance_valid(settings_popup):
		_build_menu_popups()


func _make_popup(title_text: String, width: float, height: float, centered: bool = false) -> PanelContainer:
	var panel := PanelContainer.new()
	panel.name = "%sPopup" % title_text
	if centered:
		panel.set_anchors_preset(Control.PRESET_CENTER)
		# 内容超过预设高度时，向上下同时扩展，保持弹窗的实际几何中心在屏幕中心。
		panel.grow_horizontal = Control.GROW_DIRECTION_BOTH
		panel.grow_vertical = Control.GROW_DIRECTION_BOTH
		panel.offset_left = -width * 0.5
		panel.offset_top = -height * 0.5
		panel.offset_right = width * 0.5
		panel.offset_bottom = height * 0.5
	else:
		panel.set_anchors_preset(Control.PRESET_TOP_RIGHT)
		panel.offset_left = -width - 24.0
		panel.offset_top = 96.0
		panel.offset_right = -24.0
		panel.offset_bottom = 96.0 + height
	panel.z_index = 80
	panel.visible = false
	panel.add_theme_stylebox_override("panel", _popup_style())
	add_child(panel)
	var content := VBoxContainer.new()
	content.name = "Content"
	content.add_theme_constant_override("separation", 9)
	panel.add_child(content)
	var header := HBoxContainer.new()
	content.add_child(header)
	var title := _make_popup_label(title_text, 21, Color.WHITE)
	title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	title.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	header.add_child(title)
	var close := _make_popup_button("×")
	close.custom_minimum_size = Vector2(44, 38)
	close.pressed.connect(_close_all_menu_popups)
	header.add_child(close)
	return panel


func _make_toast_panel() -> PanelContainer:
	var panel := PanelContainer.new()
	panel.set_anchors_preset(Control.PRESET_CENTER)
	panel.offset_left = -190.0
	panel.offset_top = -54.0
	panel.offset_right = 190.0
	panel.offset_bottom = 54.0
	panel.z_index = 100
	panel.visible = false
	panel.add_theme_stylebox_override("panel", _popup_style())
	add_child(panel)
	toast_label = _make_popup_label("", 19, Color.WHITE)
	toast_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	toast_label.custom_minimum_size = Vector2(0, 82)
	panel.add_child(toast_label)
	return panel


func _make_popup_label(text_value: String, font_size: int, color: Color) -> Label:
	var label := Label.new()
	label.text = text_value
	label.add_theme_font_size_override("font_size", font_size)
	label.add_theme_color_override("font_color", color)
	label.add_theme_color_override("font_outline_color", Color(0.04, 0.02, 0.14, 0.95))
	label.add_theme_constant_override("outline_size", 2)
	label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	return label


func _make_popup_button(text_value: String) -> Button:
	var button := Button.new()
	button.text = text_value
	button.custom_minimum_size = Vector2(0, 46)
	button.focus_mode = Control.FOCUS_NONE
	_style_menu_button(button, Color("#5a4db0"))
	return button


func _make_popup_slider() -> HSlider:
	var slider := HSlider.new()
	slider.min_value = 0.0
	slider.max_value = 1.0
	slider.step = 0.05
	slider.custom_minimum_size = Vector2(0, 24)
	return slider


func _popup_style() -> StyleBoxFlat:
	var style := StyleBoxFlat.new()
	style.bg_color = Color(0.10, 0.08, 0.28, 0.97)
	style.border_width_left = 2
	style.border_width_top = 2
	style.border_width_right = 2
	style.border_width_bottom = 2
	style.border_color = Color(0.74, 0.78, 1.0, 0.78)
	style.corner_radius_top_left = 24
	style.corner_radius_top_right = 24
	style.corner_radius_bottom_left = 24
	style.corner_radius_bottom_right = 24
	style.content_margin_left = 16
	style.content_margin_right = 16
	style.content_margin_top = 14
	style.content_margin_bottom = 14
	style.shadow_color = Color(0.02, 0.01, 0.12, 0.68)
	style.shadow_size = 14
	style.shadow_offset = Vector2(0, 5)
	return style


func _style_menu_button(button: Button, color: Color) -> void:
	var normal := StyleBoxFlat.new()
	normal.bg_color = Color(color.r, color.g, color.b, 0.9)
	normal.border_width_left = 2
	normal.border_width_top = 2
	normal.border_width_right = 2
	normal.border_width_bottom = 2
	normal.border_color = Color(0.66, 0.86, 1.0, 0.72)
	normal.corner_radius_top_left = 18
	normal.corner_radius_top_right = 18
	normal.corner_radius_bottom_left = 18
	normal.corner_radius_bottom_right = 18
	var hover := normal.duplicate()
	hover.bg_color = Color(0.34, 0.72, 0.92, 0.94)
	hover.border_color = Color.WHITE
	var pressed := normal.duplicate()
	pressed.bg_color = Color(0.48, 0.38, 0.82, 0.98)
	button.add_theme_stylebox_override("normal", normal)
	button.add_theme_stylebox_override("hover", hover)
	button.add_theme_stylebox_override("pressed", pressed)
	button.add_theme_stylebox_override("focus", hover)
	button.add_theme_color_override("font_color", Color.WHITE)
	button.add_theme_color_override("font_hover_color", Color.WHITE)
	button.add_theme_color_override("font_pressed_color", Color.WHITE)
	button.add_theme_color_override("font_outline_color", Color(0.04, 0.02, 0.14, 0.95))
	button.add_theme_constant_override("outline_size", 2)
	button.mouse_filter = Control.MOUSE_FILTER_STOP


func _hide_menu_popups(except: Control = null) -> void:
	for popup in [settings_popup, tasks_popup, more_popup]:
		if is_instance_valid(popup) and popup != except:
			popup.visible = false


func _close_all_menu_popups() -> void:
	_hide_menu_popups()


func _show_menu_toast(message: String) -> void:
	_ensure_popups()
	_hide_menu_popups()
	toast_label.text = message
	toast_panel.modulate = Color.WHITE
	toast_panel.visible = true
	if toast_tween:
		toast_tween.kill()
	toast_tween = create_tween()
	toast_tween.tween_interval(1.35)
	toast_tween.tween_property(toast_panel, "modulate", Color(1, 1, 1, 0), 0.32)
	toast_tween.tween_callback(func() -> void: toast_panel.visible = false)


func _audio_service() -> Node:
	if is_instance_valid(game_controller):
		var service = game_controller.get("audio_service")
		if service is Node:
			return service
	return null


func _refresh_menu_audio_ui() -> void:
	progress = SaveServiceScript.load_progress()
	var audio: Dictionary = progress.get("audio", {})
	var music_enabled := bool(audio.get("music_enabled", true))
	var sfx_enabled := bool(audio.get("sfx_enabled", true))
	menu_music_toggle.text = "♫ 背景音乐：开启" if music_enabled else "♫ 背景音乐：关闭"
	menu_sfx_toggle.text = "♪ 按键音效：开启" if sfx_enabled else "♪ 按键音效：关闭"
	menu_music_slider.set_value_no_signal(clampf(inverse_lerp(-36.0, -6.0, float(audio.get("music_volume_db", -25.0))), 0.0, 1.0))
	menu_sfx_slider.set_value_no_signal(clampf(inverse_lerp(-24.0, 0.0, float(audio.get("sfx_volume_db", -9.0))), 0.0, 1.0))
	var service := _audio_service()
	var track_name := "当前曲目"
	if service and service.has_method("get_music_track_name"):
		track_name = str(service.call("get_music_track_name"))
	menu_track_button.text = "更换音乐 · %s" % track_name
	_style_menu_button(menu_music_toggle, Color("#4969b8") if music_enabled else Color("#3b315f"))
	_style_menu_button(menu_sfx_toggle, Color("#4969b8") if sfx_enabled else Color("#3b315f"))


func _refresh_menu_tasks_ui() -> void:
	progress = SaveServiceScript.load_progress()
	var snapshot := TaskServiceScript.snapshot(progress, SaveServiceScript.today_key())
	for task_id in ["campaign_clear", "endless_questions", "combo"]:
		var label := tasks_popup.get_node("Content/Task_%s" % task_id) as Label
		var task: Dictionary = snapshot[task_id]
		var mark := "✓" if bool(task["claimed"]) else "%d/%d" % [int(task["value"]), int(task["target"])]
		label.text = "%s   %s\n奖励 +%d 金币" % [mark, str(task["title"]), int(task["reward"])]
		label.add_theme_color_override("font_color", Color("#ffe38a") if bool(task["claimed"]) else Color.WHITE)
	var weekly := TaskServiceScript.weekly_snapshot(progress, SaveServiceScript.today_key())
	for task_id in ["weekly_campaign", "weekly_daily", "weekly_endless", "weekly_friend"]:
		var label := tasks_popup.get_node("Content/Weekly_%s" % task_id) as Label
		var task: Dictionary = weekly[task_id]
		var mark := "✓" if bool(task["claimed"]) else "%d/%d" % [int(task["value"]), int(task["target"])]
		label.text = "%s   %s\n奖励 +%d 金币" % [mark, str(task["title"]), int(task["reward"])]
		label.add_theme_color_override("font_color", Color("#ffe38a") if bool(task["claimed"]) else Color.WHITE)


func _on_menu_music_toggle_pressed() -> void:
	if game_controller.has_method("_toggle_music_enabled"):
		game_controller.call("_toggle_music_enabled")
	_refresh_menu_audio_ui()


func _on_menu_sfx_toggle_pressed() -> void:
	if game_controller.has_method("_toggle_sfx_enabled"):
		game_controller.call("_toggle_sfx_enabled")
	_refresh_menu_audio_ui()


func _on_menu_track_pressed() -> void:
	if game_controller.has_method("_next_music_track"):
		game_controller.call("_next_music_track")
	_refresh_menu_audio_ui()


func _on_menu_music_volume_changed(value: float) -> void:
	if game_controller.has_method("_on_music_volume_changed"):
		game_controller.call("_on_music_volume_changed", value)


func _on_menu_sfx_volume_changed(value: float) -> void:
	if game_controller.has_method("_on_sfx_volume_changed"):
		game_controller.call("_on_sfx_volume_changed", value)


func _on_game_home_requested() -> void:
	_hide_menu_popups()
	_load_progress()
	if is_instance_valid(game_controller):
		game_controller.visible = false
	menu_content.visible = true
	visible = true
	_update_ui()
