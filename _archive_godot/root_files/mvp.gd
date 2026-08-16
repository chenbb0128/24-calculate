extends Control

signal home_requested

const PuzzleGeneratorScript = preload("res://core/puzzle_generator.gd")
const LevelCatalogScript = preload("res://core/level_catalog.gd")
const MatchDataScript = preload("res://core/match_data.gd")
const DailyChallengeScript = preload("res://core/daily_challenge.gd")
const EndlessModeScript = preload("res://core/endless_mode.gd")
const SkinCatalogScript = preload("res://core/skin_catalog.gd")
const SaveServiceScript = preload("res://services/save_service.gd")
const AdServiceScript = preload("res://services/ad_service.gd")
const RewardServiceScript = preload("res://services/reward_service.gd")
const LeaderboardServiceScript = preload("res://services/leaderboard_service.gd")
const TaskServiceScript = preload("res://services/task_service.gd")
const AchievementServiceScript = preload("res://services/achievement_service.gd")
const AudioServiceScript = preload("res://services/audio_service.gd")
const CartoonDecorScript = preload("res://ui/cartoon_decor.gd")
const TimerBadgeScript = preload("res://ui/timer_badge.gd")
const CelebrationFxScript = preload("res://ui/celebration_fx.gd")
const FriendMatchServiceScript = preload("res://core/friend_match_service.gd")
const ShareServiceScript = preload("res://services/share_service.gd")
const PlatformAdapterScript = preload("res://services/platform_adapter.gd")
const PlayerStatsScript = preload("res://services/player_stats.gd")

var BG := Color("#1e1b4b")
var chapter_tint := Color("#312e81")
var SURFACE := Color("#25215b")
var SURFACE_2 := Color("#312e81")
var CARD := Color("#ffffff")
var CARD_SELECTED := Color("#22d3ee")
var CARD_TEXT := Color("#ffffff")
var TEXT := Color("#ffffff")
var MUTED := Color("#a5b4fc")
var ACCENT := Color("#fbbf24")
var WARNING := Color("#fbbf24")
var DANGER := Color("#f97316")
var BLUE := Color("#22d3ee")
var GOLD := Color("#fbbf24")

var generator: RefCounted
var ad_service: RefCounted
var platform_adapter: RefCounted
var audio_service: Node
var levels: Array = []
var progress: Dictionary = {}
var phase := "home"
const LEVELS_PER_PAGE := 20
var menu_page := 0
var current_date_key := ""
var date_check_elapsed := 0.0
var current_level := 0
var daily_mode := false
var daily_date_key := ""
var daily_config: Dictionary = {}
var daily_rule_id := ""
var endless_mode := false
var friend_mode := false
var friend_room: Dictionary = {}
var friend_puzzles: Array = []
var friend_opponent_plan: Array = []
var friend_opponent_progress: Dictionary = {}
var endless_seed := 0
var endless_solved_questions := 0
var endless_transitioning := false
var endless_used_keys: Dictionary = {}
var endless_speed_ratio := 1.0
var endless_fast_streak := 0
var endless_score_multiplier := 1.0
var current_question := 0
var level_puzzles: Array = []
var current_puzzle: Dictionary = {}
var cards: Array = []
var original_cards: Array = []
var selected_index := -1
var selected_operator := ""
var undo_stack: Array = []
var question_elapsed := 0.0
var level_time_left := 0.0
var timer_max_time := 120.0
var level_score := 0
var level_combo := 0
var max_combo := 0
var mistakes := 0
var hints_left := 0
var free_undo_available := true
var free_hint_available := true
var hint_used := false
var session_achievement_unlocks: Array = []
var question_used_operators: Array = []
var level_attempts: Array = []
var match_data: Dictionary = {}
var background_texture: TextureRect
var background_gradient: Gradient
var cartoon_decor: Control
var timer_badge: Control
var celebration_fx: Control

var home_panel: VBoxContainer
var menu_panel: VBoxContainer
var game_panel: VBoxContainer
var result_panel: VBoxContainer
var level_grid: GridContainer
var home_progress_label: Label
var home_coins_label: Label
var home_coins_panel: PanelContainer
var endless_status_label: Label
var campaign_button: Button
var daily_button: Button
var daily_status_label: Label
var shop_button: Button
var friend_mode_button: Button
var endless_button: Button
var level_page_title: Label
var chapter_banner: PanelContainer
var chapter_title_label: Label
var chapter_subtitle_label: Label
var chapter_progress_label: Label
var level_intro_instruction_label: Label
var level_label: Label
var score_label: Label
var combo_label: Label
var question_label: Label
var question_target_label: Label
var question_progress_bar: ProgressBar
var rule_label: Label
var status_label: Label
var numbers_row: GridContainer
var operator_row: HBoxContainer
var operator_buttons: Array = []
var menu_page_label: Label
var menu_prev_button: Button
var menu_next_button: Button
var undo_button: Button
var hint_button: Button
var reset_button: Button
var game_back_button: Button
var result_title: Label
var result_stats: Label
var result_stars: Label
var result_detail: Label
var next_button: Button
var result_back_button: Button
var result_ad_button: Button
var result_ad_mode := ""
var result_ad_reward_coins := 0
var result_ad_claimed := false
var result_restart_button: Button
var result_reward_label: Label
var result_record_label: Label
var result_share_button: Button
var login_notice := ""
var last_friend_result: Dictionary = {}
var tutorial_overlay: Control
var feedback_tween: Tween
var shop_panel: VBoxContainer
var shop_coins_label: Label
var shop_status_label: Label
var shop_list: VBoxContainer
var achievements_panel: VBoxContainer
var achievements_summary_label: Label
var achievements_list: VBoxContainer
var achievements_button: Button
var friend_panel: VBoxContainer
var friend_room_code_label: Label
var friend_status_label: Label
var friend_rule_label: Label
var friend_invite_button: Button
var friend_join_button: Button
var friend_start_button: Button
var friend_race_panel: PanelContainer
var friend_race_label: Label
var leaderboard_panel: VBoxContainer
var leaderboard_button: Button
var leaderboard_board_friends_button: Button
var leaderboard_board_global_button: Button
var leaderboard_campaign_button: Button
var leaderboard_daily_button: Button
var leaderboard_endless_button: Button
var leaderboard_summary_label: Label
var leaderboard_status_label: Label
var leaderboard_list: VBoxContainer
var leaderboard_board_id := LeaderboardServiceScript.BOARD_FRIENDS
var leaderboard_mode_id := LeaderboardServiceScript.MODE_CAMPAIGN
var game_header: HBoxContainer
var game_timer_row: HBoxContainer
var game_info_panel: PanelContainer
var game_info_box: VBoxContainer
var game_info_instruction_label: Label
var game_status_panel: PanelContainer
var numbers_title_label: Label
var operator_title_label: Label
var actions_title_label: Label
var game_actions: HBoxContainer
var game_restart_button: Button
var game_bottom_row: HBoxContainer
var target_chip_panel: PanelContainer
var home_task_label: Label
var home_achievement_label: Label
var weekly_task_label: Label
var daily_tasks_button: Button
var daily_tasks_panel: PanelContainer
var more_button: Button
var more_panel: PanelContainer
var stats_panel: VBoxContainer
var stats_summary_label: Label
var stats_detail_label: Label
var stats_button: Button
var audio_settings_panel: PanelContainer
var audio_settings_button: Button
var audio_music_toggle: Button
var audio_music_slider: HSlider
var audio_music_volume_label: Label
var audio_track_button: Button
var audio_sfx_toggle: Button
var audio_sfx_slider: HSlider
var audio_sfx_volume_label: Label
var audio_settings_hint: Label


func _ready() -> void:
	generator = PuzzleGeneratorScript.new()
	ad_service = AdServiceScript.new()
	platform_adapter = PlatformAdapterScript.new()
	audio_service = AudioServiceScript.new()
	audio_service.name = "AudioService"
	add_child(audio_service)
	levels = LevelCatalogScript.all()
	progress = SaveServiceScript.load_progress()
	AchievementServiceScript.ensure_progress(progress)
	SaveServiceScript.save_progress(progress)
	audio_service.apply_settings(progress.get("audio", {}))
	current_date_key = SaveServiceScript.today_key()
	ad_service.configure(progress.get("ads", {}), current_date_key)
	TaskServiceScript.ensure_day(progress, current_date_key)
	PlayerStatsScript.ensure_progress(progress)
	var login_reward := RewardServiceScript.claim_login_reward(progress, current_date_key)
	if not bool(login_reward.get("already_claimed", false)):
		login_notice = str(login_reward.get("message", ""))
	LeaderboardServiceScript.ensure_progress(progress)
	_apply_skin(str(progress.get("equipped_skin", "classic")))
	_build_shell()
	_show_home()


func _process(delta: float) -> void:
	date_check_elapsed += delta
	if date_check_elapsed >= 1.0:
		date_check_elapsed = 0.0
		_refresh_day_state()
	if phase != "playing":
		return
	level_time_left = maxf(0.0, level_time_left - delta)
	question_elapsed += delta
	_update_hud()
	if audio_service:
		audio_service.update_countdown(level_time_left)
	if level_time_left <= 0.0:
		if endless_mode and not endless_transitioning:
			_endless_miss("时间到啦")
		else:
			_finish_level(false, "时间到啦，再试一次")
	if friend_mode and phase == "playing":
		_update_friend_race()


func _refresh_day_state() -> void:
	var next_date_key := SaveServiceScript.today_key()
	if next_date_key == current_date_key:
		return
	current_date_key = next_date_key
	ad_service.configure(progress.get("ads", {}), current_date_key)
	TaskServiceScript.ensure_day(progress, current_date_key)
	SaveServiceScript.save_progress(progress)
	# 正在答题时不打断当前题；返回首页后会立即显示新一天的挑战。
	if phase != "playing":
		_show_home()
	elif daily_mode and status_label:
		status_label.text = "新的一天开始了，当前挑战完成后返回首页即可领取今日挑战"
		status_label.add_theme_color_override("font_color", ACCENT)


func _build_shell() -> void:
	background_gradient = Gradient.new()
	background_gradient.colors = PackedColorArray([BG.lightened(0.08), Color("#312e81")])
	var gradient_texture := GradientTexture2D.new()
	gradient_texture.gradient = background_gradient
	gradient_texture.width = 720
	gradient_texture.height = 1100
	gradient_texture.fill_from = Vector2(0.0, 0.0)
	gradient_texture.fill_to = Vector2(1.0, 1.0)
	background_texture = TextureRect.new()
	background_texture.texture = gradient_texture
	background_texture.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	background_texture.stretch_mode = TextureRect.STRETCH_SCALE
	background_texture.mouse_filter = Control.MOUSE_FILTER_IGNORE
	background_texture.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	add_child(background_texture)

	cartoon_decor = CartoonDecorScript.new()
	cartoon_decor.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	cartoon_decor.configure(BG, SURFACE, MUTED, ACCENT, GOLD)
	add_child(cartoon_decor)

	var margin := MarginContainer.new()
	margin.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	margin.add_theme_constant_override("margin_left", 28)
	margin.add_theme_constant_override("margin_right", 28)
	margin.add_theme_constant_override("margin_top", 24)
	margin.add_theme_constant_override("margin_bottom", 24)
	add_child(margin)

	var content := VBoxContainer.new()
	content.add_theme_constant_override("separation", 12)
	content.size_flags_vertical = Control.SIZE_EXPAND_FILL
	margin.add_child(content)
	celebration_fx = CelebrationFxScript.new()
	celebration_fx.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	celebration_fx.z_index = 30
	celebration_fx.configure_palette(ACCENT, BLUE, Color("#f39adf"), Color("#7ee6a8"), TEXT)
	add_child(celebration_fx)

	home_panel = VBoxContainer.new()
	home_panel.size_flags_vertical = Control.SIZE_EXPAND_FILL
	home_panel.add_theme_constant_override("separation", 12)
	content.add_child(home_panel)
	_build_home()

	menu_panel = VBoxContainer.new()
	menu_panel.add_theme_constant_override("separation", 12)
	content.add_child(menu_panel)
	_build_menu()

	game_panel = VBoxContainer.new()
	game_panel.add_theme_constant_override("separation", 12)
	content.add_child(game_panel)
	_build_game_panel()

	result_panel = VBoxContainer.new()
	result_panel.add_theme_constant_override("separation", 12)
	content.add_child(result_panel)
	_build_result_panel()
	shop_panel = VBoxContainer.new()
	shop_panel.add_theme_constant_override("separation", 12)
	content.add_child(shop_panel)
	_build_shop()
	achievements_panel = VBoxContainer.new()
	achievements_panel.add_theme_constant_override("separation", 12)
	content.add_child(achievements_panel)
	_build_achievements()
	friend_panel = VBoxContainer.new()
	friend_panel.add_theme_constant_override("separation", 12)
	content.add_child(friend_panel)
	_build_friend_match()
	leaderboard_panel = VBoxContainer.new()
	leaderboard_panel.add_theme_constant_override("separation", 12)
	content.add_child(leaderboard_panel)
	_build_leaderboard()
	_build_audio_settings()
	_build_daily_tasks()
	_build_more_panel()
	_build_stats_panel(content)
	_build_tutorial_overlay()


func _build_home() -> void:
	# 首页保持单一主线：开始游戏，其余功能收进“更多功能”。
	home_panel.add_theme_constant_override("separation", 8)
	var top_bar := HBoxContainer.new()
	top_bar.add_theme_constant_override("separation", 10)
	home_panel.add_child(top_bar)
	var top_spacer := Control.new()
	top_spacer.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	top_bar.add_child(top_spacer)
	home_coins_panel = PanelContainer.new()
	home_coins_panel.custom_minimum_size = Vector2(142, 52)
	_style_coin_badge(home_coins_panel)
	top_bar.add_child(home_coins_panel)
	var coin_content := HBoxContainer.new()
	coin_content.alignment = BoxContainer.ALIGNMENT_CENTER
	coin_content.add_theme_constant_override("separation", 5)
	home_coins_panel.add_child(coin_content)
	var coin_icon := _label("●", 20, ACCENT)
	coin_icon.custom_minimum_size = Vector2(24, 28)
	coin_icon.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	coin_content.add_child(coin_icon)
	home_coins_label = _label("金币  0", 16, ACCENT)
	home_coins_label.custom_minimum_size = Vector2(94, 42)
	home_coins_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	coin_content.add_child(home_coins_label)
	audio_settings_button = Button.new()
	audio_settings_button.text = "设置"
	audio_settings_button.custom_minimum_size = Vector2(92, 52)
	audio_settings_button.focus_mode = Control.FOCUS_NONE
	audio_settings_button.pressed.connect(_toggle_audio_settings)
	_bind_click_audio(audio_settings_button)
	_style_button(audio_settings_button, SURFACE_2)
	top_bar.add_child(audio_settings_button)

	var spacer_top := Control.new()
	spacer_top.custom_minimum_size = Vector2(0, 8)
	home_panel.add_child(spacer_top)

	# 品牌区只保留标题和目标数字，避免和玩法入口争夺注意力。
	var brand_panel := PanelContainer.new()
	brand_panel.custom_minimum_size = Vector2(0, 128)
	var brand_style := StyleBoxFlat.new()
	brand_style.bg_color = SURFACE.darkened(0.08)
	brand_style.border_width_left = 2
	brand_style.border_width_top = 2
	brand_style.border_width_right = 2
	brand_style.border_width_bottom = 2
	brand_style.border_color = GOLD.darkened(0.12)
	brand_style.corner_radius_top_left = 26
	brand_style.corner_radius_top_right = 26
	brand_style.corner_radius_bottom_left = 26
	brand_style.corner_radius_bottom_right = 26
	brand_style.content_margin_left = 18
	brand_style.content_margin_right = 18
	brand_style.content_margin_top = 10
	brand_style.content_margin_bottom = 10
	brand_style.shadow_color = Color(0, 0, 0, 0.22)
	brand_style.shadow_size = 8
	brand_style.shadow_offset = Vector2(0, 4)
	brand_panel.add_theme_stylebox_override("panel", brand_style)
	home_panel.add_child(brand_panel)
	var brand_content := HBoxContainer.new()
	brand_content.add_theme_constant_override("separation", 16)
	brand_content.alignment = BoxContainer.ALIGNMENT_CENTER
	brand_panel.add_child(brand_content)

	var emblem := PanelContainer.new()
	emblem.custom_minimum_size = Vector2(84, 84)
	var emblem_style := StyleBoxFlat.new()
	emblem_style.bg_color = ACCENT
	emblem_style.border_width_left = 3
	emblem_style.border_width_top = 3
	emblem_style.border_width_right = 3
	emblem_style.border_width_bottom = 3
	emblem_style.border_color = GOLD
	emblem_style.corner_radius_top_left = 24
	emblem_style.corner_radius_top_right = 24
	emblem_style.corner_radius_bottom_left = 24
	emblem_style.corner_radius_bottom_right = 24
	emblem_style.shadow_color = Color(0, 0, 0, 0.24)
	emblem_style.shadow_size = 6
	emblem_style.shadow_offset = Vector2(0, 3)
	emblem.add_theme_stylebox_override("panel", emblem_style)
	brand_content.add_child(emblem)
	var emblem_content := VBoxContainer.new()
	emblem_content.alignment = BoxContainer.ALIGNMENT_CENTER
	emblem_content.add_theme_constant_override("separation", 0)
	emblem.add_child(emblem_content)
	var logo_number := _label("24", 42, CARD_TEXT)
	emblem_content.add_child(logo_number)
	var emblem_caption := _label("目标数字", 12, CARD_TEXT)
	emblem_content.add_child(emblem_caption)

	var title_content := VBoxContainer.new()
	title_content.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	title_content.alignment = BoxContainer.ALIGNMENT_CENTER
	title_content.add_theme_constant_override("separation", 2)
	brand_content.add_child(title_content)
	var kicker := _label("数字脑力挑战", 13, ACCENT)
	kicker.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	title_content.add_child(kicker)
	var logo_title := _label("点挑战", 34, TEXT)
	logo_title.add_theme_color_override("font_outline_color", Color("#473ca1"))
	logo_title.add_theme_constant_override("outline_size", 7)
	logo_title.add_theme_color_override("font_shadow_color", Color(0, 0, 0, 0.24))
	logo_title.add_theme_constant_override("shadow_offset_x", 2)
	logo_title.add_theme_constant_override("shadow_offset_y", 3)
	logo_title.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	title_content.add_child(logo_title)
	var title_line := ColorRect.new()
	title_line.color = GOLD
	title_line.custom_minimum_size = Vector2(88, 3)
	title_line.size_flags_horizontal = Control.SIZE_SHRINK_BEGIN
	title_content.add_child(title_line)

	var spacer_middle := Control.new()
	spacer_middle.custom_minimum_size = Vector2(0, 8)
	home_panel.add_child(spacer_middle)
	var mode_hint := _label("选择你的挑战", 15, ACCENT)
	mode_hint.custom_minimum_size = Vector2(0, 20)
	mode_hint.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	home_panel.add_child(mode_hint)

	campaign_button = Button.new()
	campaign_button.text = "闯关模式  ·  进度 1-1"
	campaign_button.custom_minimum_size = Vector2(0, 76)
	campaign_button.add_theme_font_size_override("font_size", 25)
	campaign_button.focus_mode = Control.FOCUS_NONE
	campaign_button.pressed.connect(_open_campaign)
	_bind_click_audio(campaign_button)
	_style_home_button(campaign_button, Color("#5ecdf2"))
	home_panel.add_child(campaign_button)
	var friend_button := Button.new()
	friend_button.text = "好友对战"
	friend_button.custom_minimum_size = Vector2(0, 62)
	friend_button.add_theme_font_size_override("font_size", 23)
	friend_button.focus_mode = Control.FOCUS_NONE
	friend_button.pressed.connect(_show_friend_lobby)
	_bind_click_audio(friend_button)
	_style_home_button(friend_button, Color("#ff8ea8"))
	home_panel.add_child(friend_button)
	friend_mode_button = friend_button

	var mode_row := HBoxContainer.new()
	mode_row.add_theme_constant_override("separation", 10)
	home_panel.add_child(mode_row)
	var mode_info_row := HBoxContainer.new()
	mode_info_row.add_theme_constant_override("separation", 10)
	mode_info_row.visible = false
	home_panel.add_child(mode_info_row)
	var versus_button := Button.new()
	versus_button.text = "无尽模式"
	versus_button.custom_minimum_size = Vector2(0, 62)
	versus_button.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	versus_button.add_theme_font_size_override("font_size", 20)
	versus_button.focus_mode = Control.FOCUS_NONE
	versus_button.disabled = false
	versus_button.pressed.connect(_start_endless_mode)
	_bind_click_audio(versus_button)
	_style_home_button(versus_button, Color("#eea5df"))
	mode_row.add_child(versus_button)
	endless_button = versus_button
	var endless_info := _panel_box()
	endless_info.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	endless_info.custom_minimum_size = Vector2(0, 48)
	mode_info_row.add_child(endless_info)
	endless_status_label = _label("", 13, TEXT)
	endless_status_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	endless_status_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	endless_info.add_child(endless_status_label)

	daily_button = Button.new()
	daily_button.text = "每日一题"
	daily_button.custom_minimum_size = Vector2(0, 62)
	daily_button.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	daily_button.add_theme_font_size_override("font_size", 20)
	daily_button.focus_mode = Control.FOCUS_NONE
	daily_button.pressed.connect(_start_daily_challenge)
	_bind_click_audio(daily_button)
	_style_home_button(daily_button, Color("#ffe28b"))
	mode_row.add_child(daily_button)
	var daily_info := _panel_box()
	daily_info.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	daily_info.custom_minimum_size = Vector2(0, 48)
	mode_info_row.add_child(daily_info)
	daily_status_label = _label("", 13, TEXT)
	daily_status_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	daily_status_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	daily_info.add_child(daily_status_label)
	daily_tasks_button = Button.new()
	daily_tasks_button.text = "每日任务"
	daily_tasks_button.custom_minimum_size = Vector2(0, 56)
	daily_tasks_button.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	daily_tasks_button.add_theme_font_size_override("font_size", 19)
	daily_tasks_button.focus_mode = Control.FOCUS_NONE
	daily_tasks_button.pressed.connect(_toggle_daily_tasks)
	_bind_click_audio(daily_tasks_button)
	_style_home_button(daily_tasks_button, Color("#7dd9c1"))
	home_panel.add_child(daily_tasks_button)

	shop_button = Button.new()
	shop_button.text = "主题商店"
	shop_button.custom_minimum_size = Vector2(0, 60)
	shop_button.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	shop_button.add_theme_font_size_override("font_size", 20)
	shop_button.focus_mode = Control.FOCUS_NONE
	shop_button.pressed.connect(_show_shop)
	_bind_click_audio(shop_button)
	_style_home_button(shop_button, Color("#9beab8"))
	achievements_button = Button.new()
	achievements_button.text = "成就"
	achievements_button.custom_minimum_size = Vector2(0, 60)
	achievements_button.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	achievements_button.add_theme_font_size_override("font_size", 20)
	achievements_button.focus_mode = Control.FOCUS_NONE
	achievements_button.pressed.connect(_show_achievements)
	_bind_click_audio(achievements_button)
	_style_home_button(achievements_button, Color("#c69bff"))

	leaderboard_button = Button.new()
	leaderboard_button.text = "排行榜"
	leaderboard_button.custom_minimum_size = Vector2(0, 58)
	leaderboard_button.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	leaderboard_button.add_theme_font_size_override("font_size", 20)
	leaderboard_button.focus_mode = Control.FOCUS_NONE
	leaderboard_button.pressed.connect(_show_leaderboard)
	_bind_click_audio(leaderboard_button)
	_style_home_button(leaderboard_button, Color("#ffc775"))
	more_button = Button.new()
	more_button.text = "更多功能"
	more_button.custom_minimum_size = Vector2(0, 52)
	more_button.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	more_button.add_theme_font_size_override("font_size", 18)
	more_button.focus_mode = Control.FOCUS_NONE
	more_button.pressed.connect(_toggle_more_panel)
	_bind_click_audio(more_button)
	_style_button(more_button, SURFACE_2)
	home_panel.add_child(more_button)

	var footer := _label("程序验证有解 · 适配手机竖屏", 11, MUTED)
	footer.custom_minimum_size = Vector2(0, 20)
	footer.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	home_panel.add_child(footer)


func _build_menu() -> void:
	var header := HBoxContainer.new()
	header.add_theme_constant_override("separation", 8)
	menu_panel.add_child(header)
	var back_home := Button.new()
	back_home.text = "‹ 首页"
	back_home.custom_minimum_size = Vector2(92, 44)
	back_home.focus_mode = Control.FOCUS_NONE
	back_home.pressed.connect(_show_home)
	_bind_click_audio(back_home)
	_style_button(back_home, SURFACE_2)
	header.add_child(back_home)
	level_page_title = _label("选择关卡", 22, TEXT)
	level_page_title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	level_page_title.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	header.add_child(level_page_title)
	var page_hint := _label("闯关模式", 14, ACCENT)
	page_hint.custom_minimum_size = Vector2(92, 44)
	page_hint.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	header.add_child(page_hint)

	var intro := _panel_box()
	intro.custom_minimum_size = Vector2(0, 154)
	menu_panel.add_child(intro)
	# PanelContainer 只能自动布局一个子节点。此前把章节横幅和操作提示
	# 直接并列放进 PanelContainer，会让它们共享同一块矩形区域，导致文字重叠。
	var intro_content := VBoxContainer.new()
	intro_content.add_theme_constant_override("separation", 7)
	intro.add_child(intro_content)
	chapter_banner = PanelContainer.new()
	chapter_banner.custom_minimum_size = Vector2(0, 82)
	chapter_banner.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	intro_content.add_child(chapter_banner)
	var chapter_content := VBoxContainer.new()
	chapter_content.add_theme_constant_override("separation", 1)
	chapter_banner.add_child(chapter_content)
	chapter_title_label = _label("第 1 章 · 基础星球", 19, ACCENT)
	chapter_title_label.custom_minimum_size = Vector2(0, 26)
	chapter_title_label.clip_text = true
	chapter_content.add_child(chapter_title_label)
	chapter_subtitle_label = _label("先熟悉三步操作，稳稳算出 24", 13, TEXT)
	chapter_subtitle_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	chapter_subtitle_label.custom_minimum_size = Vector2(0, 22)
	chapter_content.add_child(chapter_subtitle_label)
	chapter_progress_label = _label("章节进度 1 / 20 · 整数与明显解法", 12, MUTED)
	chapter_progress_label.custom_minimum_size = Vector2(0, 20)
	chapter_progress_label.clip_text = true
	chapter_content.add_child(chapter_progress_label)
	var intro_text := _label("操作：数字 → 运算符 → 第二个数字\n每道题都由程序验证有解。", 15, TEXT)
	level_intro_instruction_label = intro_text
	intro_text.custom_minimum_size = Vector2(0, 48)
	intro_text.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	intro_text.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	intro_content.add_child(intro_text)

	var section := _label("选择关卡", 19, ACCENT)
	menu_panel.add_child(section)
	var level_grid_center := CenterContainer.new()
	level_grid_center.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	menu_panel.add_child(level_grid_center)
	level_grid = GridContainer.new()
	level_grid.columns = 4
	level_grid.add_theme_constant_override("h_separation", 10)
	level_grid.add_theme_constant_override("v_separation", 10)
	level_grid_center.add_child(level_grid)
	for index in range(LEVELS_PER_PAGE):
		var button := Button.new()
		# 固定卡片宽度，避免某个状态文字过长把单独一列撑宽。
		button.custom_minimum_size = Vector2(112, 76)
		button.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
		button.add_theme_font_size_override("font_size", 16)
		button.focus_mode = Control.FOCUS_NONE
		button.pressed.connect(_on_level_button_pressed.bind(index))
		_bind_click_audio(button)
		_style_button(button, SURFACE_2)
		level_grid.add_child(button)

	var page_controls := HBoxContainer.new()
	page_controls.add_theme_constant_override("separation", 10)
	menu_panel.add_child(page_controls)
	menu_prev_button = Button.new()
	menu_prev_button.text = "上一页"
	menu_prev_button.custom_minimum_size = Vector2(0, 48)
	menu_prev_button.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	menu_prev_button.focus_mode = Control.FOCUS_NONE
	menu_prev_button.pressed.connect(_previous_level_page)
	_bind_click_audio(menu_prev_button)
	_style_button(menu_prev_button, SURFACE_2)
	page_controls.add_child(menu_prev_button)
	menu_page_label = _label("第 1 / 10 页", 16, TEXT)
	menu_page_label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	menu_page_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	page_controls.add_child(menu_page_label)
	menu_next_button = Button.new()
	menu_next_button.text = "下一页"
	menu_next_button.custom_minimum_size = Vector2(0, 48)
	menu_next_button.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	menu_next_button.focus_mode = Control.FOCUS_NONE
	menu_next_button.pressed.connect(_next_level_page)
	_bind_click_audio(menu_next_button)
	_style_button(menu_next_button, BLUE)
	page_controls.add_child(menu_next_button)
	_refresh_level_buttons()

	var risk := _label("原型提示：Godot 暂不官方一键导出微信小游戏，后续需验证 Web/小游戏适配层。", 13, Color("#7890aa"))
	risk.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	menu_panel.add_child(risk)


func _build_game_panel() -> void:
	var header := HBoxContainer.new()
	game_header = header
	header.add_theme_constant_override("separation", 8)
	header.custom_minimum_size = Vector2(0, 62)
	game_panel.add_child(header)
	var level_chip := _make_stat_chip("当前关卡", "第 1 关", TEXT)
	level_label = level_chip["value_label"]
	header.add_child(level_chip["panel"])
	var score_chip := _make_stat_chip("本局得分", "得分 0", TEXT)
	score_label = score_chip["value_label"]
	header.add_child(score_chip["panel"])
	var combo_chip := _make_stat_chip("连续表现", "连击 0", ACCENT)
	combo_label = combo_chip["value_label"]
	header.add_child(combo_chip["panel"])
	friend_race_panel = _panel_box()
	friend_race_panel.custom_minimum_size = Vector2(0, 74)
	game_panel.add_child(friend_race_panel)
	friend_race_label = _label("好友对战将在这里显示双方进度", 14, MUTED)
	friend_race_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	friend_race_label.custom_minimum_size = Vector2(0, 46)
	friend_race_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	friend_race_panel.add_child(friend_race_label)
	friend_race_panel.visible = false
	timer_badge = TimerBadgeScript.new()
	timer_badge.custom_minimum_size = Vector2(148, 52)
	timer_badge.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	timer_badge.configure_palette(SURFACE, TEXT, MUTED, ACCENT, WARNING, DANGER)
	# 用左右弹性占位夹住计时牌，避免 CenterContainer 在窄布局下产生负偏移。
	var timer_row := HBoxContainer.new()
	game_timer_row = timer_row
	timer_row.custom_minimum_size = Vector2(0, 56)
	timer_row.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	timer_row.add_theme_constant_override("separation", 0)
	game_panel.add_child(timer_row)
	var timer_left_spacer := Control.new()
	timer_left_spacer.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	timer_row.add_child(timer_left_spacer)
	timer_row.add_child(timer_badge)
	var timer_right_spacer := Control.new()
	timer_right_spacer.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	timer_row.add_child(timer_right_spacer)

	var info_panel := _panel_box()
	game_info_panel = info_panel
	info_panel.custom_minimum_size = Vector2(0, 126)
	game_panel.add_child(info_panel)
	var info := VBoxContainer.new()
	game_info_box = info
	info.add_theme_constant_override("separation", 7)
	info_panel.add_child(info)
	var info_top := HBoxContainer.new()
	info_top.add_theme_constant_override("separation", 8)
	info.add_child(info_top)
	question_label = _label("第 1 / 5 题", 20, TEXT)
	question_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	question_label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	info_top.add_child(question_label)
	var target_chip := PanelContainer.new()
	target_chip_panel = target_chip
	target_chip.custom_minimum_size = Vector2(104, 38)
	var target_style := StyleBoxFlat.new()
	target_style.bg_color = ACCENT
	target_style.border_width_left = 2
	target_style.border_width_top = 2
	target_style.border_width_right = 2
	target_style.border_width_bottom = 2
	target_style.border_color = GOLD
	target_style.corner_radius_top_left = 19
	target_style.corner_radius_top_right = 19
	target_style.corner_radius_bottom_left = 19
	target_style.corner_radius_bottom_right = 19
	target_chip.add_theme_stylebox_override("panel", target_style)
	info_top.add_child(target_chip)
	question_target_label = _label("目标 24", 15, CARD_TEXT)
	question_target_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	target_chip.add_child(question_target_label)
	question_progress_bar = ProgressBar.new()
	question_progress_bar.min_value = 0.0
	question_progress_bar.max_value = 5.0
	question_progress_bar.value = 1.0
	question_progress_bar.show_percentage = false
	question_progress_bar.custom_minimum_size = Vector2(0, 10)
	var progress_bg := StyleBoxFlat.new()
	progress_bg.bg_color = SURFACE_2.darkened(0.1)
	progress_bg.corner_radius_top_left = 5
	progress_bg.corner_radius_top_right = 5
	progress_bg.corner_radius_bottom_left = 5
	progress_bg.corner_radius_bottom_right = 5
	var progress_fill := StyleBoxFlat.new()
	progress_fill.bg_color = ACCENT
	progress_fill.corner_radius_top_left = 5
	progress_fill.corner_radius_top_right = 5
	progress_fill.corner_radius_bottom_left = 5
	progress_fill.corner_radius_bottom_right = 5
	question_progress_bar.add_theme_stylebox_override("background", progress_bg)
	question_progress_bar.add_theme_stylebox_override("fill", progress_fill)
	info.add_child(question_progress_bar)
	var rule := _label("操作顺序：先点数字 → 再点运算符 → 最后点数字", 13, MUTED)
	game_info_instruction_label = rule
	rule.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	info.add_child(rule)
	rule_label = _label("", 14, ACCENT)
	rule_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	rule_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	info.add_child(rule_label)

	var status_panel := _panel_box()
	game_status_panel = status_panel
	status_panel.custom_minimum_size = Vector2(0, 60)
	game_panel.add_child(status_panel)
	status_label = _label("请选择第一个数字", 17, MUTED)
	status_label.custom_minimum_size = Vector2(0, 46)
	status_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	status_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	status_panel.add_child(status_label)

	var numbers_title := _label("第一步 · 选择数字", 15, ACCENT)
	numbers_title_label = numbers_title
	numbers_title.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	game_panel.add_child(numbers_title)
	numbers_row = GridContainer.new()
	numbers_row.columns = 2
	numbers_row.add_theme_constant_override("h_separation", 12)
	numbers_row.add_theme_constant_override("v_separation", 12)
	game_panel.add_child(numbers_row)

	var operator_title := _label("第二步 · 选择运算符", 15, ACCENT)
	operator_title_label = operator_title
	operator_title.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	game_panel.add_child(operator_title)
	operator_row = HBoxContainer.new()
	operator_row.add_theme_constant_override("separation", 10)
	game_panel.add_child(operator_row)
	for operator in ["+", "−", "×", "÷"]:
		var button := Button.new()
		button.text = operator
		button.custom_minimum_size = Vector2(0, 62)
		button.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		button.add_theme_font_size_override("font_size", 27)
		button.focus_mode = Control.FOCUS_NONE
		button.pressed.connect(_on_operator_pressed.bind(operator))
		button.button_down.connect(_on_button_down.bind(button))
		button.button_up.connect(_on_button_up.bind(button))
		_style_operator(button, BLUE)
		operator_row.add_child(button)
		operator_buttons.append(button)

	var actions_title := _label("第三步 · 点击第二个数字完成合成", 14, MUTED)
	actions_title_label = actions_title
	actions_title.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	game_panel.add_child(actions_title)

	var actions := HBoxContainer.new()
	game_actions = actions
	actions.add_theme_constant_override("separation", 10)
	game_panel.add_child(actions)
	undo_button = Button.new()
	undo_button.text = "撤销"
	undo_button.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	undo_button.custom_minimum_size = Vector2(0, 52)
	undo_button.focus_mode = Control.FOCUS_NONE
	undo_button.pressed.connect(_undo)
	_bind_click_audio(undo_button)
	_style_button(undo_button, SURFACE_2)
	actions.add_child(undo_button)
	hint_button = Button.new()
	hint_button.text = "提示"
	hint_button.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	hint_button.custom_minimum_size = Vector2(0, 52)
	hint_button.focus_mode = Control.FOCUS_NONE
	hint_button.pressed.connect(_use_hint)
	_bind_click_audio(hint_button)
	_style_button(hint_button, Color("#765b9e"))
	actions.add_child(hint_button)
	reset_button = Button.new()
	reset_button.text = "重置本题"
	reset_button.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	reset_button.custom_minimum_size = Vector2(0, 52)
	reset_button.focus_mode = Control.FOCUS_NONE
	reset_button.pressed.connect(_reset_puzzle)
	_bind_click_audio(reset_button)
	_style_button(reset_button, SURFACE_2)
	actions.add_child(reset_button)

	var bottom := HBoxContainer.new()
	game_bottom_row = bottom
	bottom.add_theme_constant_override("separation", 10)
	game_panel.add_child(bottom)
	var restart := Button.new()
	game_restart_button = restart
	restart.text = "重新开始本关"
	restart.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	restart.custom_minimum_size = Vector2(0, 48)
	restart.focus_mode = Control.FOCUS_NONE
	restart.pressed.connect(_restart_level)
	_bind_click_audio(restart)
	_style_button(restart, Color("#365f98"))
	bottom.add_child(restart)
	var back := Button.new()
	back.text = "返回关卡"
	back.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	back.custom_minimum_size = Vector2(0, 48)
	back.focus_mode = Control.FOCUS_NONE
	back.pressed.connect(_on_game_back_pressed)
	_bind_click_audio(back)
	_style_button(back, SURFACE_2)
	bottom.add_child(back)
	game_back_button = back


func _set_panel_padding(panel: PanelContainer, horizontal: int, vertical: int) -> void:
	if not is_instance_valid(panel):
		return
	var style := panel.get_theme_stylebox("panel") as StyleBoxFlat
	if style:
		style.content_margin_left = horizontal
		style.content_margin_right = horizontal
		style.content_margin_top = vertical
		style.content_margin_bottom = vertical


func _apply_friend_layout(active: bool) -> void:
	if not game_panel:
		return
	# 好友对战信息本来会和顶部统计、规则说明重复；这里改为一张紧凑的对战进度卡。
	game_panel.add_theme_constant_override("separation", 6 if active else 12)
	game_header.visible = not active
	friend_race_panel.visible = active
	if active:
		friend_race_panel.custom_minimum_size = Vector2(0, 58)
		_set_panel_padding(friend_race_panel, 12, 7)
		friend_race_label.custom_minimum_size = Vector2(0, 40)
		friend_race_label.add_theme_font_size_override("font_size", 13)
		game_timer_row.custom_minimum_size = Vector2(0, 46)
		timer_badge.custom_minimum_size = Vector2(136, 44)
		game_info_panel.custom_minimum_size = Vector2(0, 68)
		_set_panel_padding(game_info_panel, 12, 7)
		game_info_box.add_theme_constant_override("separation", 3)
		game_info_instruction_label.visible = false
		question_progress_bar.custom_minimum_size = Vector2(0, 7)
		target_chip_panel.visible = false
		target_chip_panel.custom_minimum_size = Vector2(92, 30)
		question_target_label.add_theme_font_size_override("font_size", 13)
		game_status_panel.custom_minimum_size = Vector2(0, 42)
		_set_panel_padding(game_status_panel, 12, 5)
		status_label.custom_minimum_size = Vector2(0, 32)
		status_label.add_theme_font_size_override("font_size", 15)
		numbers_title_label.add_theme_font_size_override("font_size", 13)
		operator_title_label.add_theme_font_size_override("font_size", 13)
		actions_title_label.visible = false
		hint_button.visible = false
		undo_button.custom_minimum_size = Vector2(0, 44)
		reset_button.custom_minimum_size = Vector2(0, 44)
		game_bottom_row.custom_minimum_size = Vector2(0, 44)
		game_restart_button.visible = false
		game_back_button.custom_minimum_size = Vector2(0, 44)
		game_back_button.text = "退出对战"
	else:
		friend_race_panel.custom_minimum_size = Vector2(0, 74)
		_set_panel_padding(friend_race_panel, 18, 14)
		friend_race_label.custom_minimum_size = Vector2(0, 46)
		friend_race_label.add_theme_font_size_override("font_size", 14)
		game_timer_row.custom_minimum_size = Vector2(0, 56)
		timer_badge.custom_minimum_size = Vector2(148, 52)
		game_info_panel.custom_minimum_size = Vector2(0, 126)
		_set_panel_padding(game_info_panel, 18, 14)
		game_info_box.add_theme_constant_override("separation", 7)
		game_info_instruction_label.visible = true
		question_progress_bar.custom_minimum_size = Vector2(0, 10)
		target_chip_panel.visible = true
		target_chip_panel.custom_minimum_size = Vector2(104, 38)
		question_target_label.add_theme_font_size_override("font_size", 15)
		game_status_panel.custom_minimum_size = Vector2(0, 60)
		_set_panel_padding(game_status_panel, 18, 14)
		status_label.custom_minimum_size = Vector2(0, 46)
		status_label.add_theme_font_size_override("font_size", 17)
		numbers_title_label.add_theme_font_size_override("font_size", 15)
		operator_title_label.add_theme_font_size_override("font_size", 15)
		actions_title_label.visible = true
		hint_button.visible = true
		undo_button.custom_minimum_size = Vector2(0, 52)
		reset_button.custom_minimum_size = Vector2(0, 52)
		game_bottom_row.custom_minimum_size = Vector2(0, 48)
		game_restart_button.visible = true
		game_back_button.custom_minimum_size = Vector2(0, 48)
	_update_action_buttons()


func _make_stat_chip(caption: String, value: String, value_color: Color) -> Dictionary:
	var panel := PanelContainer.new()
	panel.custom_minimum_size = Vector2(0, 58)
	panel.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	var style := StyleBoxFlat.new()
	style.bg_color = Color(1.0, 1.0, 1.0, 0.08)
	style.border_width_left = 2
	style.border_width_top = 2
	style.border_width_right = 2
	style.border_width_bottom = 2
	style.border_color = Color(1.0, 1.0, 1.0, 0.15)
	style.corner_radius_top_left = 16
	style.corner_radius_top_right = 16
	style.corner_radius_bottom_left = 16
	style.corner_radius_bottom_right = 16
	style.content_margin_left = 8
	style.content_margin_right = 8
	style.content_margin_top = 5
	style.content_margin_bottom = 5
	style.shadow_color = Color(0.0, 0.0, 0.0, 0.30)
	style.shadow_size = 12
	style.shadow_offset = Vector2(0, 4)
	panel.add_theme_stylebox_override("panel", style)
	var content := VBoxContainer.new()
	content.add_theme_constant_override("separation", 0)
	panel.add_child(content)
	var caption_label := _label(caption, 11, MUTED)
	caption_label.add_theme_color_override("font_outline_color", Color(0.03, 0.02, 0.12, 0.95))
	caption_label.add_theme_constant_override("outline_size", 2)
	caption_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	content.add_child(caption_label)
	var value_label := _label(value, 16, value_color)
	value_label.add_theme_color_override("font_outline_color", Color(0.03, 0.02, 0.12, 0.95))
	value_label.add_theme_constant_override("outline_size", 2)
	value_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	content.add_child(value_label)
	return {"panel": panel, "value_label": value_label}


func _build_result_panel() -> void:
	result_title = _label("本关结算", 30, TEXT)
	result_panel.add_child(result_title)
	result_stars = _label("☆☆☆", 44, WARNING)
	result_panel.add_child(result_stars)
	result_stats = _label("", 18, MUTED)
	result_stats.custom_minimum_size = Vector2(0, 110)
	result_stats.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	result_panel.add_child(result_stats)
	result_record_label = _label("", 14, ACCENT)
	result_record_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	result_record_label.custom_minimum_size = Vector2(0, 28)
	result_panel.add_child(result_record_label)
	result_detail = _label("", 15, MUTED)
	result_detail.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	result_panel.add_child(result_detail)
	result_reward_label = _label("", 16, ACCENT)
	result_reward_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	result_panel.add_child(result_reward_label)
	result_share_button = Button.new()
	result_share_button.text = "分享本局战绩"
	result_share_button.custom_minimum_size = Vector2(0, 46)
	result_share_button.focus_mode = Control.FOCUS_NONE
	result_share_button.pressed.connect(_share_match_result)
	_bind_click_audio(result_share_button)
	_style_button(result_share_button, Color("#6fcfdd"))
	result_share_button.visible = false
	result_panel.add_child(result_share_button)
	result_ad_button = Button.new()
	result_ad_button.text = "▶ 看广告领取额外奖励"
	result_ad_button.custom_minimum_size = Vector2(0, 48)
	result_ad_button.focus_mode = Control.FOCUS_NONE
	result_ad_button.pressed.connect(_watch_result_ad)
	_bind_click_audio(result_ad_button)
	_style_button(result_ad_button, Color("#6fcf97"))
	result_ad_button.visible = false
	result_panel.add_child(result_ad_button)

	var restart := Button.new()
	restart.text = "再来一局"
	restart.custom_minimum_size = Vector2(0, 56)
	restart.focus_mode = Control.FOCUS_NONE
	restart.pressed.connect(_restart_level)
	_bind_click_audio(restart)
	_style_button(restart, Color("#256c5c"))
	result_panel.add_child(restart)
	result_restart_button = restart
	next_button = Button.new()
	next_button.text = "下一关"
	next_button.custom_minimum_size = Vector2(0, 56)
	next_button.focus_mode = Control.FOCUS_NONE
	next_button.pressed.connect(_go_next_level)
	_bind_click_audio(next_button)
	_style_button(next_button, Color("#365f98"))
	result_panel.add_child(next_button)
	var back := Button.new()
	back.text = "返回关卡"
	back.custom_minimum_size = Vector2(0, 48)
	back.focus_mode = Control.FOCUS_NONE
	back.pressed.connect(_show_menu)
	_bind_click_audio(back)
	_style_button(back, SURFACE_2)
	result_panel.add_child(back)
	result_back_button = back


func _build_shop() -> void:
	var header := HBoxContainer.new()
	header.add_theme_constant_override("separation", 8)
	shop_panel.add_child(header)
	var back := Button.new()
	back.text = "‹ 首页"
	back.custom_minimum_size = Vector2(92, 44)
	back.focus_mode = Control.FOCUS_NONE
	back.pressed.connect(_show_home)
	_bind_click_audio(back)
	_style_button(back, SURFACE_2)
	header.add_child(back)
	var title := _label("主题商店", 24, TEXT)
	title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	title.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	header.add_child(title)
	shop_coins_label = _label("金币 0", 16, ACCENT)
	shop_coins_label.custom_minimum_size = Vector2(108, 44)
	shop_coins_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	header.add_child(shop_coins_label)

	var intro := _panel_box()
	shop_panel.add_child(intro)
	var intro_text := _label("用闯关和每日挑战获得金币，兑换喜欢的界面主题。\n皮肤只改变外观，不会改变题目难度。", 16, TEXT)
	intro_text.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	intro_text.custom_minimum_size = Vector2(0, 60)
	intro.add_child(intro_text)
	shop_status_label = _label("", 15, ACCENT)
	shop_status_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	shop_panel.add_child(shop_status_label)
	shop_list = VBoxContainer.new()
	shop_list.add_theme_constant_override("separation", 10)
	shop_panel.add_child(shop_list)


func _build_achievements() -> void:
	var header := HBoxContainer.new()
	header.add_theme_constant_override("separation", 8)
	achievements_panel.add_child(header)
	var back := Button.new()
	back.text = "‹ 首页"
	back.custom_minimum_size = Vector2(92, 44)
	back.focus_mode = Control.FOCUS_NONE
	back.pressed.connect(_show_home)
	_bind_click_audio(back)
	_style_button(back, SURFACE_2)
	header.add_child(back)
	var title := _label("成就徽章", 24, TEXT)
	title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	title.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	header.add_child(title)
	var hint := _label("收集金币", 13, ACCENT)
	hint.custom_minimum_size = Vector2(78, 44)
	hint.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	header.add_child(hint)
	var intro := _panel_box()
	achievements_panel.add_child(intro)
	achievements_summary_label = _label("", 17, TEXT)
	achievements_summary_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	achievements_summary_label.custom_minimum_size = Vector2(0, 62)
	achievements_summary_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	intro.add_child(achievements_summary_label)
	achievements_list = VBoxContainer.new()
	achievements_list.add_theme_constant_override("separation", 8)
	achievements_panel.add_child(achievements_list)


func _build_friend_match() -> void:
	var header := HBoxContainer.new()
	header.add_theme_constant_override("separation", 8)
	friend_panel.add_child(header)
	var back := Button.new()
	back.text = "‹ 首页"
	back.custom_minimum_size = Vector2(92, 44)
	back.focus_mode = Control.FOCUS_NONE
	back.pressed.connect(_show_home)
	_bind_click_audio(back)
	_style_button(back, SURFACE_2)
	header.add_child(back)
	var title := _label("好友对战", 24, TEXT)
	title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	title.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	header.add_child(title)
	var tag := _label("同题竞速", 13, ACCENT)
	tag.custom_minimum_size = Vector2(82, 44)
	tag.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	header.add_child(tag)

	var hero := _panel_box()
	hero.custom_minimum_size = Vector2(0, 138)
	friend_panel.add_child(hero)
	var hero_content := VBoxContainer.new()
	hero_content.alignment = BoxContainer.ALIGNMENT_CENTER
	hero_content.add_theme_constant_override("separation", 4)
	hero.add_child(hero_content)
	var hero_title := _label("约上好友，比比谁更快", 24, ACCENT)
	hero_title.add_theme_color_override("font_outline_color", Color("#473ca1"))
	hero_title.add_theme_constant_override("outline_size", 4)
	hero_content.add_child(hero_title)
	var hero_note := _label("同一套题目 · 8 道挑战 · 不显示对方答案", 14, TEXT)
	hero_note.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	hero_content.add_child(hero_note)

	var room_card := _panel_box()
	room_card.custom_minimum_size = Vector2(0, 154)
	friend_panel.add_child(room_card)
	var room_content := VBoxContainer.new()
	room_content.alignment = BoxContainer.ALIGNMENT_CENTER
	room_content.add_theme_constant_override("separation", 5)
	room_card.add_child(room_content)
	var room_caption := _label("房间口令", 13, MUTED)
	room_content.add_child(room_caption)
	friend_room_code_label = _label("------", 38, ACCENT)
	friend_room_code_label.add_theme_color_override("font_outline_color", Color("#473ca1"))
	friend_room_code_label.add_theme_constant_override("outline_size", 5)
	room_content.add_child(friend_room_code_label)
	friend_status_label = _label("正在创建房间…", 15, TEXT)
	friend_status_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	room_content.add_child(friend_status_label)
	friend_rule_label = _label("", 13, MUTED)
	friend_rule_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	room_content.add_child(friend_rule_label)

	friend_invite_button = Button.new()
	friend_invite_button.text = "复制邀请口令"
	friend_invite_button.custom_minimum_size = Vector2(0, 56)
	friend_invite_button.focus_mode = Control.FOCUS_NONE
	friend_invite_button.pressed.connect(_share_friend_room)
	_bind_click_audio(friend_invite_button)
	_style_home_button(friend_invite_button, Color("#62c8f2"))
	friend_panel.add_child(friend_invite_button)

	friend_join_button = Button.new()
	friend_join_button.text = "模拟好友加入（桌面体验）"
	friend_join_button.custom_minimum_size = Vector2(0, 52)
	friend_join_button.focus_mode = Control.FOCUS_NONE
	friend_join_button.pressed.connect(_simulate_friend_join)
	_bind_click_audio(friend_join_button)
	_style_button(friend_join_button, Color("#765b9e"))
	friend_panel.add_child(friend_join_button)

	friend_start_button = Button.new()
	friend_start_button.text = "开始同题竞速"
	friend_start_button.custom_minimum_size = Vector2(0, 64)
	friend_start_button.add_theme_font_size_override("font_size", 22)
	friend_start_button.focus_mode = Control.FOCUS_NONE
	friend_start_button.pressed.connect(_start_friend_match)
	_bind_click_audio(friend_start_button)
	_style_home_button(friend_start_button, Color("#ff8ea8"))
	friend_panel.add_child(friend_start_button)

	var footer := _label("正式微信版：点击邀请后调用 wx.shareAppMessage，好友从分享卡片进入同一房间。", 12, MUTED)
	footer.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	friend_panel.add_child(footer)


func _show_friend_lobby() -> void:
	phase = "friend_lobby"
	friend_mode = false
	friend_room = FriendMatchServiceScript.create_room(SaveServiceScript.today_seed() * 97 + Time.get_ticks_msec())
	_refresh_friend_lobby()
	_show_only(friend_panel)


func _refresh_friend_lobby() -> void:
	if not friend_room_code_label:
		return
	friend_room_code_label.text = str(friend_room.get("room_code", "------"))
	var players: Array = friend_room.get("players", [])
	var joined := players.size() >= 2
	friend_status_label.text = "好友已加入，可以开始！" if joined else "等待好友加入…"
	friend_status_label.add_theme_color_override("font_color", ACCENT if joined else TEXT)
	friend_rule_label.text = "8 道同题 · 总限时 120 秒 · 答错扣时间 · 禁用提示\n胜负顺序：答对题数 → 总分 → 用时"
	friend_start_button.disabled = not joined
	friend_join_button.disabled = joined


func _simulate_friend_join() -> void:
	if friend_room.is_empty():
		return
	friend_room = FriendMatchServiceScript.join_room(friend_room, "friend-local", "好友·小满")
	_refresh_friend_lobby()
	if celebration_fx:
		celebration_fx.toast("好友已加入房间！", ACCENT, 1.0)


func _share_friend_room() -> void:
	if friend_room.is_empty():
		return
	var payload := ShareServiceScript.create_friend_room_payload(friend_room)
	DisplayServer.clipboard_set(ShareServiceScript.build_invite_text(friend_room))
	friend_status_label.text = "邀请口令已复制：%s\n微信版将打开好友分享面板" % str(payload["room_code"])
	friend_status_label.add_theme_color_override("font_color", ACCENT)


func _start_friend_match() -> void:
	var players: Array = friend_room.get("players", [])
	if players.size() < 2:
		friend_status_label.text = "请先邀请好友加入房间"
		return
	friend_puzzles = FriendMatchServiceScript.generate_puzzles(generator, int(friend_room.get("room_seed", 0)))
	if friend_puzzles.size() < FriendMatchServiceScript.QUESTION_COUNT:
		friend_status_label.text = "房间题目生成失败，请重新创建房间"
		return
	friend_mode = true
	daily_mode = false
	endless_mode = false
	_apply_friend_layout(true)
	current_level = 0
	level_puzzles = friend_puzzles.duplicate(true)
	match_data = FriendMatchServiceScript.create_match(friend_room, friend_puzzles)
	friend_opponent_plan = FriendMatchServiceScript.build_opponent_plan(int(friend_room.get("room_seed", 0)), friend_puzzles.size())
	friend_opponent_progress = {}
	level_score = 0
	level_combo = 0
	max_combo = 0
	mistakes = 0
	hints_left = 0
	free_undo_available = true
	free_hint_available = true
	hint_used = false
	current_question = 0
	question_elapsed = 0.0
	level_time_left = FriendMatchServiceScript.TIME_LIMIT
	timer_max_time = FriendMatchServiceScript.TIME_LIMIT
	level_attempts.clear()
	session_achievement_unlocks.clear()
	phase = "playing"
	if audio_service:
		audio_service.start_countdown()
	_load_question()
	_show_only(game_panel)


func _show_achievements() -> void:
	phase = "achievements"
	_refresh_achievements()
	_show_only(achievements_panel)


func _refresh_achievements() -> void:
	if not is_instance_valid(achievements_panel) or not is_instance_valid(achievements_list):
		return
	for child in achievements_list.get_children():
		child.queue_free()
	var total := AchievementServiceScript.all().size()
	var unlocked_count := AchievementServiceScript.unlocked_count(progress)
	achievements_summary_label.text = "已收集 %d / %d 枚徽章\n每枚成就只奖励一次金币，继续挑战来解锁更多目标。" % [unlocked_count, total]
	for achievement in AchievementServiceScript.all():
		var achievement_id := str(achievement["id"])
		var unlocked := AchievementServiceScript.is_unlocked(progress, achievement_id)
		var row := PanelContainer.new()
		row.custom_minimum_size = Vector2(0, 74)
		var row_style := StyleBoxFlat.new()
		row_style.bg_color = Color(0.28, 0.22, 0.56, 0.9) if unlocked else Color(0.12, 0.1, 0.28, 0.9)
		row_style.border_width_left = 2
		row_style.border_width_top = 2
		row_style.border_width_right = 2
		row_style.border_width_bottom = 2
		row_style.border_color = Color(1.0, 0.83, 0.4, 0.9) if unlocked else Color(0.52, 0.58, 0.82, 0.5)
		row_style.corner_radius_top_left = 16
		row_style.corner_radius_top_right = 16
		row_style.corner_radius_bottom_left = 16
		row_style.corner_radius_bottom_right = 16
		row_style.content_margin_left = 12
		row_style.content_margin_right = 12
		row_style.content_margin_top = 8
		row_style.content_margin_bottom = 8
		row.add_theme_stylebox_override("panel", row_style)
		achievements_list.add_child(row)
		var content := HBoxContainer.new()
		content.add_theme_constant_override("separation", 10)
		row.add_child(content)
		var icon := _label(str(achievement["icon"]) if unlocked else "?", 28, ACCENT if unlocked else MUTED)
		icon.custom_minimum_size = Vector2(42, 52)
		icon.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
		content.add_child(icon)
		var text_box := VBoxContainer.new()
		text_box.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		text_box.alignment = BoxContainer.ALIGNMENT_CENTER
		content.add_child(text_box)
		var title := _label(str(achievement["title"]), 16, TEXT if unlocked else MUTED)
		title.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
		text_box.add_child(title)
		var description := _label(str(achievement["description"]), 12, MUTED)
		description.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
		description.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		text_box.add_child(description)
		var reward := _label("已获得" if unlocked else "+%d 金币" % int(achievement["reward"]), 13, ACCENT if unlocked else WARNING)
		reward.custom_minimum_size = Vector2(78, 44)
		reward.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
		reward.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT
		content.add_child(reward)


func _build_leaderboard() -> void:
	var header := HBoxContainer.new()
	header.add_theme_constant_override("separation", 8)
	leaderboard_panel.add_child(header)
	var back := Button.new()
	back.text = "‹ 首页"
	back.custom_minimum_size = Vector2(92, 44)
	back.focus_mode = Control.FOCUS_NONE
	back.pressed.connect(_show_home)
	_bind_click_audio(back)
	_style_button(back, SURFACE_2)
	header.add_child(back)
	var title := _label("排行榜", 24, TEXT)
	title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	title.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	header.add_child(title)
	var source_hint := _label("原型榜单", 13, ACCENT)
	source_hint.custom_minimum_size = Vector2(78, 44)
	source_hint.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	header.add_child(source_hint)

	var board_tabs := HBoxContainer.new()
	board_tabs.add_theme_constant_override("separation", 10)
	leaderboard_panel.add_child(board_tabs)
	leaderboard_board_friends_button = Button.new()
	leaderboard_board_friends_button.text = "微信好友榜"
	leaderboard_board_friends_button.custom_minimum_size = Vector2(0, 52)
	leaderboard_board_friends_button.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	leaderboard_board_friends_button.focus_mode = Control.FOCUS_NONE
	leaderboard_board_friends_button.pressed.connect(_select_leaderboard_board.bind(LeaderboardServiceScript.BOARD_FRIENDS))
	_bind_click_audio(leaderboard_board_friends_button)
	board_tabs.add_child(leaderboard_board_friends_button)
	leaderboard_board_global_button = Button.new()
	leaderboard_board_global_button.text = "游戏总榜"
	leaderboard_board_global_button.custom_minimum_size = Vector2(0, 52)
	leaderboard_board_global_button.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	leaderboard_board_global_button.focus_mode = Control.FOCUS_NONE
	leaderboard_board_global_button.pressed.connect(_select_leaderboard_board.bind(LeaderboardServiceScript.BOARD_GLOBAL))
	_bind_click_audio(leaderboard_board_global_button)
	board_tabs.add_child(leaderboard_board_global_button)

	var mode_tabs := HBoxContainer.new()
	mode_tabs.add_theme_constant_override("separation", 8)
	leaderboard_panel.add_child(mode_tabs)
	leaderboard_campaign_button = _make_leaderboard_mode_button("闯关", LeaderboardServiceScript.MODE_CAMPAIGN)
	mode_tabs.add_child(leaderboard_campaign_button)
	leaderboard_daily_button = _make_leaderboard_mode_button("每日", LeaderboardServiceScript.MODE_DAILY)
	mode_tabs.add_child(leaderboard_daily_button)
	leaderboard_endless_button = _make_leaderboard_mode_button("无尽", LeaderboardServiceScript.MODE_ENDLESS)
	mode_tabs.add_child(leaderboard_endless_button)

	var intro := _panel_box()
	leaderboard_panel.add_child(intro)
	leaderboard_summary_label = _label("", 16, TEXT)
	leaderboard_summary_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	leaderboard_summary_label.custom_minimum_size = Vector2(0, 52)
	leaderboard_summary_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	intro.add_child(leaderboard_summary_label)

	var scroll := ScrollContainer.new()
	scroll.custom_minimum_size = Vector2(0, 440)
	scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	leaderboard_panel.add_child(scroll)
	leaderboard_list = VBoxContainer.new()
	leaderboard_list.add_theme_constant_override("separation", 8)
	leaderboard_list.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	scroll.add_child(leaderboard_list)

	leaderboard_status_label = _label("", 13, MUTED)
	leaderboard_status_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	leaderboard_panel.add_child(leaderboard_status_label)
	_refresh_leaderboard()


func _build_audio_settings() -> void:
	audio_settings_panel = _panel_box()
	audio_settings_panel.name = "AudioSettingsPanel"
	audio_settings_panel.set_anchors_preset(Control.PRESET_TOP_RIGHT)
	audio_settings_panel.offset_left = -340.0
	audio_settings_panel.offset_top = 82.0
	audio_settings_panel.offset_right = -28.0
	audio_settings_panel.offset_bottom = 386.0
	audio_settings_panel.z_index = 20
	audio_settings_panel.visible = false
	add_child(audio_settings_panel)
	var content := VBoxContainer.new()
	content.add_theme_constant_override("separation", 7)
	audio_settings_panel.add_child(content)
	var header := HBoxContainer.new()
	content.add_child(header)
	var title := _label("声音设置", 19, TEXT)
	title.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	header.add_child(title)
	var close_button := Button.new()
	close_button.text = "×"
	close_button.custom_minimum_size = Vector2(38, 34)
	close_button.focus_mode = Control.FOCUS_NONE
	close_button.pressed.connect(_toggle_audio_settings)
	_bind_click_audio(close_button)
	_style_button(close_button, SURFACE_2)
	header.add_child(close_button)

	audio_music_toggle = Button.new()
	audio_music_toggle.custom_minimum_size = Vector2(0, 38)
	audio_music_toggle.focus_mode = Control.FOCUS_NONE
	audio_music_toggle.pressed.connect(_toggle_music_enabled)
	_bind_click_audio(audio_music_toggle)
	content.add_child(audio_music_toggle)
	_style_button(audio_music_toggle, BLUE)

	audio_music_volume_label = _label("背景音乐音量", 13, MUTED)
	audio_music_volume_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	content.add_child(audio_music_volume_label)
	audio_music_slider = HSlider.new()
	audio_music_slider.min_value = 0.0
	audio_music_slider.max_value = 1.0
	audio_music_slider.step = 0.05
	audio_music_slider.custom_minimum_size = Vector2(0, 24)
	audio_music_slider.value_changed.connect(_on_music_volume_changed)
	content.add_child(audio_music_slider)

	audio_track_button = Button.new()
	audio_track_button.custom_minimum_size = Vector2(0, 38)
	audio_track_button.focus_mode = Control.FOCUS_NONE
	audio_track_button.pressed.connect(_next_music_track)
	_bind_click_audio(audio_track_button)
	content.add_child(audio_track_button)
	_style_button(audio_track_button, SURFACE_2)

	audio_sfx_toggle = Button.new()
	audio_sfx_toggle.custom_minimum_size = Vector2(0, 38)
	audio_sfx_toggle.focus_mode = Control.FOCUS_NONE
	audio_sfx_toggle.pressed.connect(_toggle_sfx_enabled)
	_bind_click_audio(audio_sfx_toggle)
	content.add_child(audio_sfx_toggle)
	_style_button(audio_sfx_toggle, BLUE)

	audio_sfx_volume_label = _label("按键音效音量", 13, MUTED)
	audio_sfx_volume_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	content.add_child(audio_sfx_volume_label)
	audio_sfx_slider = HSlider.new()
	audio_sfx_slider.min_value = 0.0
	audio_sfx_slider.max_value = 1.0
	audio_sfx_slider.step = 0.05
	audio_sfx_slider.custom_minimum_size = Vector2(0, 24)
	audio_sfx_slider.value_changed.connect(_on_sfx_volume_changed)
	content.add_child(audio_sfx_slider)
	audio_settings_hint = _label("设置会自动保存；微信小游戏中也可继续复用这套配置。", 12, MUTED)
	audio_settings_hint.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	audio_settings_hint.custom_minimum_size = Vector2(0, 30)
	content.add_child(audio_settings_hint)
	_refresh_audio_settings_ui()


func _build_daily_tasks() -> void:
	daily_tasks_panel = _panel_box()
	daily_tasks_panel.name = "DailyTasksPanel"
	daily_tasks_panel.set_anchors_preset(Control.PRESET_TOP_RIGHT)
	daily_tasks_panel.offset_left = -380.0
	daily_tasks_panel.offset_top = 82.0
	daily_tasks_panel.offset_right = -28.0
	daily_tasks_panel.offset_bottom = 432.0
	daily_tasks_panel.z_index = 20
	daily_tasks_panel.visible = false
	add_child(daily_tasks_panel)
	var content := VBoxContainer.new()
	content.add_theme_constant_override("separation", 8)
	daily_tasks_panel.add_child(content)
	var header := HBoxContainer.new()
	content.add_child(header)
	var title := _label("✦ 每日任务", 19, TEXT)
	title.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	header.add_child(title)
	var close_button := Button.new()
	close_button.text = "×"
	close_button.custom_minimum_size = Vector2(38, 34)
	close_button.focus_mode = Control.FOCUS_NONE
	close_button.pressed.connect(_toggle_daily_tasks)
	_bind_click_audio(close_button)
	_style_button(close_button, SURFACE_2)
	header.add_child(close_button)
	var note := _label("完成任务自动领取金币 · 每天零点刷新", 12, MUTED)
	note.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	note.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	content.add_child(note)
	home_task_label = _label("", 14, TEXT)
	home_task_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	home_task_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	home_task_label.custom_minimum_size = Vector2(0, 112)
	content.add_child(home_task_label)
	var achievement_title := _label("🏅 成就进度", 14, ACCENT)
	achievement_title.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	content.add_child(achievement_title)
	home_achievement_label = _label("", 13, TEXT)
	home_achievement_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	home_achievement_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	home_achievement_label.custom_minimum_size = Vector2(0, 44)
	content.add_child(home_achievement_label)
	var weekly_title := _label("本周任务", 14, BLUE)
	weekly_title.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	content.add_child(weekly_title)
	weekly_task_label = _label("", 13, TEXT)
	weekly_task_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	weekly_task_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	weekly_task_label.custom_minimum_size = Vector2(0, 84)
	content.add_child(weekly_task_label)


func _build_more_panel() -> void:
	more_panel = _panel_box()
	more_panel.name = "MorePanel"
	more_panel.set_anchors_preset(Control.PRESET_TOP_RIGHT)
	more_panel.offset_left = -360.0
	more_panel.offset_top = 82.0
	more_panel.offset_right = -28.0
	more_panel.offset_bottom = 356.0
	more_panel.z_index = 20
	more_panel.visible = false
	add_child(more_panel)
	var content := VBoxContainer.new()
	content.add_theme_constant_override("separation", 8)
	more_panel.add_child(content)
	var header := HBoxContainer.new()
	content.add_child(header)
	var title := _label("⋯ 更多功能", 19, TEXT)
	title.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	header.add_child(title)
	var close_button := Button.new()
	close_button.text = "×"
	close_button.custom_minimum_size = Vector2(38, 34)
	close_button.focus_mode = Control.FOCUS_NONE
	close_button.pressed.connect(_toggle_more_panel)
	_bind_click_audio(close_button)
	_style_button(close_button, SURFACE_2)
	header.add_child(close_button)
	var note := _label("商店、成就和排行榜集中在这里", 12, MUTED)
	note.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	content.add_child(note)
	for button in [shop_button, achievements_button, leaderboard_button]:
		button.custom_minimum_size = Vector2(0, 48)
		button.add_theme_font_size_override("font_size", 17)
		content.add_child(button)
	stats_button = Button.new()
	stats_button.text = "挑战记录"
	stats_button.custom_minimum_size = Vector2(0, 48)
	stats_button.add_theme_font_size_override("font_size", 17)
	stats_button.focus_mode = Control.FOCUS_NONE
	stats_button.pressed.connect(_show_stats)
	_bind_click_audio(stats_button)
	_style_button(stats_button, Color("#8bb8ff"))
	content.add_child(stats_button)


func _build_stats_panel(shell_content: VBoxContainer) -> void:
	stats_panel = VBoxContainer.new()
	stats_panel.add_theme_constant_override("separation", 12)
	# 由 _build_shell 的 content 统一承载，保持与其他功能页相同的返回层级。
	shell_content.add_child(stats_panel)
	var header := HBoxContainer.new()
	header.add_theme_constant_override("separation", 8)
	stats_panel.add_child(header)
	var back := Button.new()
	back.text = "‹ 首页"
	back.custom_minimum_size = Vector2(92, 44)
	back.focus_mode = Control.FOCUS_NONE
	back.pressed.connect(_show_home)
	_bind_click_audio(back)
	_style_button(back, SURFACE_2)
	header.add_child(back)
	var title := _label("挑战记录", 24, TEXT)
	title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	title.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	header.add_child(title)
	var intro := _panel_box()
	stats_panel.add_child(intro)
	stats_summary_label = _label("", 18, ACCENT)
	stats_summary_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	stats_summary_label.custom_minimum_size = Vector2(0, 110)
	stats_summary_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	intro.add_child(stats_summary_label)
	stats_detail_label = _label("", 15, MUTED)
	stats_detail_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	stats_detail_label.custom_minimum_size = Vector2(0, 190)
	stats_detail_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	stats_panel.add_child(stats_detail_label)
	var note := _label("这些记录只统计已验证的正确答案，不影响关卡评分。", 12, MUTED)
	note.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	stats_panel.add_child(note)


func _make_leaderboard_mode_button(text: String, mode_id: String) -> Button:
	var button := Button.new()
	button.text = text
	button.custom_minimum_size = Vector2(0, 46)
	button.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	button.focus_mode = Control.FOCUS_NONE
	button.pressed.connect(_select_leaderboard_mode.bind(mode_id))
	_bind_click_audio(button)
	return button


func _start_level(index: int) -> void:
	if not SaveServiceScript.is_unlocked(progress, index):
		return
	current_level = clampi(index, 0, levels.size() - 1)
	daily_mode = false
	friend_mode = false
	_apply_friend_layout(false)
	daily_date_key = ""
	daily_config = {}
	daily_rule_id = ""
	endless_mode = false
	session_achievement_unlocks.clear()
	SaveServiceScript.save_last_level(progress, current_level)
	var config: Dictionary = levels[current_level]
	_apply_chapter_music(int(config.get("chapter_index", int(current_level / 20))))
	level_puzzles = generator.generate_level(config, current_level, int(config["question_count"]))
	if level_puzzles.is_empty():
		status_label.text = "题目生成失败，请重新启动原型"
		return
	match_data = MatchDataScript.create_match("solo-%d" % Time.get_ticks_msec(), level_puzzles, {
		"target": 24,
		"integer_intermediates": true,
		"same_puzzles_for_match": true,
	})
	phase = "playing"
	if audio_service:
		audio_service.start_countdown()
	current_question = 0
	question_elapsed = 0.0
	level_time_left = float(config["time_limit"])
	timer_max_time = level_time_left
	level_score = 0
	level_combo = 0
	max_combo = 0
	mistakes = 0
	hints_left = 0
	free_undo_available = true
	free_hint_available = true
	hint_used = false
	question_used_operators.clear()
	selected_index = -1
	selected_operator = ""
	level_attempts.clear()
	_load_question()
	_show_only(game_panel)


func _start_daily_challenge() -> void:
	endless_mode = false
	friend_mode = false
	_apply_friend_layout(false)
	session_achievement_unlocks.clear()
	daily_mode = true
	daily_date_key = SaveServiceScript.today_key()
	daily_config = DailyChallengeScript.build(generator, daily_date_key, SaveServiceScript.today_seed())
	if daily_config.is_empty():
		return
	level_puzzles = daily_config.get("puzzles", [])
	daily_rule_id = str(daily_config.get("rule_id", ""))
	current_level = 0
	level_time_left = 120.0 if bool(daily_config.get("time_bonus", false)) else float(daily_config.get("time_limit", 150.0))
	timer_max_time = level_time_left
	level_score = 0
	level_combo = 0
	max_combo = 0
	mistakes = 0
	hints_left = 0
	free_undo_available = true
	free_hint_available = true
	hint_used = false
	question_used_operators.clear()
	level_attempts.clear()
	selected_index = -1
	selected_operator = ""
	match_data = MatchDataScript.create_match("daily-%s" % daily_date_key, level_puzzles, {
		"target": 24,
		"daily": true,
		"rule_id": daily_rule_id,
		"rule_text": daily_config.get("rule_text", ""),
	})
	_apply_chapter_music(int(daily_config.get("rule_index", 0)) % 5)
	phase = "playing"
	if audio_service:
		audio_service.start_countdown()
	current_question = 0
	question_elapsed = 0.0
	_load_question()
	_show_only(game_panel)


func _start_endless_mode() -> void:
	daily_mode = false
	daily_date_key = ""
	daily_config = {}
	daily_rule_id = ""
	endless_mode = true
	_apply_chapter_music(2)
	friend_mode = false
	_apply_friend_layout(false)
	session_achievement_unlocks.clear()
	endless_seed = SaveServiceScript.today_seed() * 31 + Time.get_ticks_msec()
	endless_solved_questions = 0
	endless_used_keys.clear()
	endless_speed_ratio = 1.0
	endless_fast_streak = 0
	endless_score_multiplier = 1.0
	current_level = 0
	level_puzzles.clear()
	level_score = 0
	level_combo = 0
	max_combo = 0
	mistakes = 0
	hints_left = 0
	free_undo_available = true
	free_hint_available = true
	hint_used = false
	question_used_operators.clear()
	level_attempts.clear()
	match_data = MatchDataScript.create_match("endless-%d" % endless_seed, [], {
		"target": 24,
		"endless": true,
		"same_puzzles_for_match": true,
	})
	phase = "playing"
	if audio_service:
		audio_service.start_countdown()
	current_question = 0
	question_elapsed = 0.0
	endless_transitioning = false
	level_time_left = float(EndlessModeScript.config_for_question(0)["time_limit"])
	timer_max_time = level_time_left
	_load_endless_question()
	_show_only(game_panel)


func _load_endless_question() -> void:
	var puzzle := EndlessModeScript.generate_question(generator, current_question, endless_seed, endless_used_keys, {
		"speed_ratio": endless_speed_ratio,
		"fast_streak": endless_fast_streak,
	})
	if puzzle.is_empty():
		_finish_level(false, "题目生成失败，请再试一次")
		return
	current_puzzle = puzzle
	level_puzzles = [puzzle]
	match_data["puzzles"].append(puzzle.duplicate(true))
	original_cards.clear()
	for number in current_puzzle["numbers"]:
		original_cards.append({"value": int(number), "expression": str(number)})
	var config := EndlessModeScript.config_for_question(current_question, {
		"speed_ratio": endless_speed_ratio,
		"fast_streak": endless_fast_streak,
	})
	level_time_left = float(config["time_limit"])
	timer_max_time = level_time_left
	if audio_service:
		audio_service.start_countdown()
	hints_left = 0
	hint_used = false
	question_elapsed = 0.0
	selected_index = -1
	selected_operator = ""
	undo_stack.clear()
	question_used_operators.clear()
	endless_transitioning = false
	cards = original_cards.duplicate(true)
	status_label.text = "第 %d 题 · %s · 答错不结束，超时才结束" % [current_question + 1, str(config["stage_name"])]
	status_label.add_theme_color_override("font_color", ACCENT)
	_render_cards()
	_update_hud()


func _build_tutorial_overlay() -> void:
	tutorial_overlay = ColorRect.new()
	tutorial_overlay.color = Color(0.0, 0.12, 0.08, 0.82)
	tutorial_overlay.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	add_child(tutorial_overlay)
	var center := CenterContainer.new()
	center.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	tutorial_overlay.add_child(center)
	var panel := _panel_box()
	panel.custom_minimum_size = Vector2(480, 0)
	center.add_child(panel)
	var content := VBoxContainer.new()
	content.add_theme_constant_override("separation", 12)
	panel.add_child(content)
	var title := _label("新手提示", 28, ACCENT)
	content.add_child(title)
	var instructions := _label("跟着下面三步试一次，数字不会显示计算过程，画面更清爽。", 16, TEXT)
	instructions.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	content.add_child(instructions)
	var steps := HBoxContainer.new()
	steps.add_theme_constant_override("separation", 8)
	steps.custom_minimum_size = Vector2(0, 112)
	content.add_child(steps)
	var step_data: Array = [
		["1", "点数字", "先点一个数字", BLUE],
		["2", "选符号", "再点 + − × ÷", Color("#c69bff")],
		["3", "点数字", "最后点第二个数字", Color("#ff8ea8")],
	]
	for data in step_data:
		var step_panel := PanelContainer.new()
		step_panel.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		var step_style := StyleBoxFlat.new()
		step_style.bg_color = Color(data[3].r, data[3].g, data[3].b, 0.20)
		step_style.border_width_left = 2
		step_style.border_width_top = 2
		step_style.border_width_right = 2
		step_style.border_width_bottom = 2
		step_style.border_color = Color(data[3].r, data[3].g, data[3].b, 0.72)
		step_style.corner_radius_top_left = 14
		step_style.corner_radius_top_right = 14
		step_style.corner_radius_bottom_left = 14
		step_style.corner_radius_bottom_right = 14
		step_style.content_margin_left = 6
		step_style.content_margin_right = 6
		step_style.content_margin_top = 7
		step_style.content_margin_bottom = 7
		step_panel.add_theme_stylebox_override("panel", step_style)
		steps.add_child(step_panel)
		var step_box := VBoxContainer.new()
		step_box.alignment = BoxContainer.ALIGNMENT_CENTER
		step_box.add_theme_constant_override("separation", 2)
		step_panel.add_child(step_box)
		var number := _label(str(data[0]), 25, data[3])
		number.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		step_box.add_child(number)
		var step_title := _label(str(data[1]), 14, TEXT)
		step_title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		step_box.add_child(step_title)
		var note := _label(str(data[2]), 11, MUTED)
		note.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		note.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		step_box.add_child(note)
	var instructions_tail := _label("两个数字会合并，重复操作直到得到 24。", 15, MUTED)
	instructions_tail.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	content.add_child(instructions_tail)
	var got_it := Button.new()
	got_it.text = "知道了，开始挑战"
	got_it.custom_minimum_size = Vector2(0, 56)
	got_it.focus_mode = Control.FOCUS_NONE
	got_it.pressed.connect(_close_tutorial)
	_bind_click_audio(got_it)
	_style_home_button(got_it, Color("#43c96e"))
	content.add_child(got_it)
	tutorial_overlay.visible = false


func _close_tutorial() -> void:
	SaveServiceScript.mark_tutorial_seen(progress)
	tutorial_overlay.visible = false
	_start_level(0)


func _on_level_button_pressed(slot: int) -> void:
	var level_index := menu_page * LEVELS_PER_PAGE + slot
	if level_index < levels.size():
		_start_level(level_index)


func _load_question() -> void:
	if current_question >= level_puzzles.size():
		_finish_level(true, "全部题目完成")
		return
	current_puzzle = level_puzzles[current_question]
	original_cards.clear()
	for number in current_puzzle["numbers"]:
		original_cards.append({"value": int(number), "expression": str(number)})
	_reset_puzzle(false)
	if audio_service:
		audio_service.start_countdown()
	_update_hud()


func _reset_puzzle(add_mistake: bool = true) -> void:
	if phase != "playing":
		return
	if add_mistake:
		mistakes += 1
		level_combo = 0
		level_time_left = maxf(0.0, level_time_left - 2.0)
	cards = original_cards.duplicate(true)
	selected_index = -1
	selected_operator = ""
	undo_stack.clear()
	question_used_operators.clear()
	status_label.text = "本题已重置，重新找找规律吧"
	status_label.add_theme_color_override("font_color", WARNING if add_mistake else MUTED)
	_render_cards()


func _on_card_pressed(index: int) -> void:
	if phase != "playing" or index < 0 or index >= cards.size():
		return
	if audio_service:
		audio_service.play_card()
	if celebration_fx and index < numbers_row.get_child_count():
		celebration_fx.tap(numbers_row.get_child(index).get_global_rect().get_center(), CARD_SELECTED)
	if not selected_operator.is_empty():
		if index == selected_index:
			status_label.text = "第二个数字不能和第一个相同"
			status_label.add_theme_color_override("font_color", WARNING)
			return
		_apply_selected_operation(index)
		return
	if selected_index == index:
		selected_index = -1
		status_label.text = "请选择第一个数字"
		status_label.add_theme_color_override("font_color", MUTED)
	else:
		selected_index = index
		status_label.text = "已选第一个数字，请点击运算符"
		status_label.add_theme_color_override("font_color", ACCENT)
	_render_cards()


func _on_operator_pressed(operator: String) -> void:
	if phase != "playing":
		return
	if audio_service:
		audio_service.play_operator()
	if selected_index == -1:
		status_label.text = "先点击第一个数字"
		status_label.add_theme_color_override("font_color", WARNING)
		return
	if daily_mode and not _daily_operator_allowed(operator):
		status_label.text = "今日规则：不能使用 %s" % operator
		status_label.add_theme_color_override("font_color", WARNING)
		return
	selected_operator = operator
	status_label.text = "已选 %s，请点击第二个数字" % operator
	status_label.add_theme_color_override("font_color", ACCENT)
	_render_cards()


func _apply_selected_operation(second_index: int) -> void:
	var first_index := selected_index
	var operator := selected_operator
	var first: Dictionary = cards[first_index]
	var second: Dictionary = cards[second_index]
	var first_center: Vector2 = numbers_row.get_child(first_index).get_global_rect().get_center() if first_index < numbers_row.get_child_count() else Vector2.ZERO
	var second_center: Vector2 = numbers_row.get_child(second_index).get_global_rect().get_center() if second_index < numbers_row.get_child_count() else Vector2.ZERO
	if daily_mode and daily_rule_id == "no_division" and operator == "÷":
		status_label.text = "今日规则：不能使用除法"
		status_label.add_theme_color_override("font_color", WARNING)
		selected_index = -1
		selected_operator = ""
		_render_cards()
		return
	var calculation := _calculate(int(first["value"]), int(second["value"]), operator)
	if not calculation["valid"]:
		if audio_service:
			audio_service.play_error()
		if endless_mode:
			status_label.text = "%s，请继续尝试" % str(calculation["message"])
			status_label.add_theme_color_override("font_color", WARNING)
		else:
			mistakes += 1
			level_combo = 0
			level_time_left = maxf(0.0, level_time_left - 2.0)
			if friend_mode:
				level_time_left = maxf(0.0, level_time_left - 3.0)
			status_label.text = str(calculation["message"])
			status_label.add_theme_color_override("font_color", DANGER)
		selected_index = -1
		selected_operator = ""
		_render_cards()
		return

	undo_stack.append(cards.duplicate(true))
	question_used_operators.append(operator)
	var next_cards: Array = []
	for index in range(cards.size()):
		if index != first_index and index != second_index:
			next_cards.append(cards[index])
	next_cards.append({
			"value": int(calculation["value"]),
			"expression": "(%s %s %s)" % [first["expression"], operator, second["expression"]],
		})
	cards = next_cards
	selected_index = -1
	selected_operator = ""
	status_label.text = "合并成功，还剩 %d 个数字" % cards.size()
	status_label.add_theme_color_override("font_color", ACCENT)
	_render_cards()
	_play_merge_feedback()
	if celebration_fx:
		celebration_fx.merge(first_center, second_center)
		_pulse_control(numbers_row, 1.035, 0.16)
	if audio_service:
		audio_service.play_merge()
	if cards.size() == 1:
		_check_answer()


func _calculate(first: int, second: int, operator: String) -> Dictionary:
	match operator:
		"+":
			return {"valid": true, "value": first + second, "message": ""}
		"−", "-":
			return {"valid": true, "value": first - second, "message": ""}
		"×":
			return {"valid": true, "value": first * second, "message": ""}
		"÷":
			if second == 0:
				return {"valid": false, "value": 0, "message": "除数不能为 0"}
			if first % second != 0:
				return {"valid": false, "value": 0, "message": "第一版只允许整数结果"}
			return {"valid": true, "value": int(first / second), "message": ""}
	return {"valid": false, "value": 0, "message": "未知运算符"}


func _check_answer() -> void:
	var answer := int(cards[0]["value"])
	if answer == 24:
		if daily_mode and not _daily_requirement_met():
			status_label.text = "今日规则：这道题还没有满足指定运算条件"
			status_label.add_theme_color_override("font_color", WARNING)
			level_combo = 0
			mistakes += 1
			return
		var bonus := maxi(0, int(level_time_left) * 2)
		var earned := 100 + bonus + level_combo * 15
		level_combo += 1
		if endless_mode:
			endless_score_multiplier = EndlessModeScript.score_multiplier(level_combo, endless_fast_streak)
			earned = int(round(float(earned) * endless_score_multiplier))
		level_score += earned
		max_combo = maxi(max_combo, level_combo)
		var stats_mode := "endless" if endless_mode else ("daily" if daily_mode else "campaign")
		PlayerStatsScript.record_solve(progress, stats_mode, int(question_elapsed * 1000.0), earned, level_combo, question_used_operators, -1 if (endless_mode or friend_mode) else current_level)
		TaskServiceScript.record_max(progress, "combo", max_combo, SaveServiceScript.today_key())
		var combo_achievements: Array = []
		if max_combo >= 5:
			combo_achievements.append("combo_5")
		if max_combo >= 10:
			combo_achievements.append("combo_10")
		_unlock_achievements(combo_achievements)
		level_attempts.append(MatchDataScript.create_attempt(str(current_puzzle["puzzle_id"]), int(question_elapsed * 1000.0), true, mistakes, earned))
		if endless_mode:
			var endless_config := EndlessModeScript.config_for_question(current_question, {
				"speed_ratio": endless_speed_ratio,
				"fast_streak": endless_fast_streak,
			})
			endless_speed_ratio = clampf(question_elapsed / maxf(0.1, float(endless_config["time_limit"])), 0.0, 1.0)
			if endless_speed_ratio <= 0.62:
				endless_fast_streak += 1
			else:
				endless_fast_streak = 0
			endless_solved_questions += 1
			TaskServiceScript.record_weekly(progress, "weekly_endless", 1, SaveServiceScript.today_key())
			var endless_achievements: Array = []
			if endless_solved_questions >= 5:
				endless_achievements.append("endless_5")
			if endless_solved_questions >= 10:
				endless_achievements.append("endless_10")
			if endless_solved_questions >= 30:
				endless_achievements.append("endless_30")
			_unlock_achievements(endless_achievements)
			TaskServiceScript.record(progress, "endless_questions", 1, SaveServiceScript.today_key())
			SaveServiceScript.save_progress(progress)
			var milestone := EndlessModeScript.milestone_for_questions(endless_solved_questions)
			if not milestone.is_empty():
				status_label.text = "%s\n+%d 分 · %.2fx 倍率" % [str(milestone["title"]), earned, endless_score_multiplier]
				_play_merge_feedback()
			else:
				status_label.text = "答对了！+%d 分 · 连击 %d · %.2fx 倍率" % [earned, level_combo, endless_score_multiplier]
			status_label.add_theme_color_override("font_color", ACCENT)
			if audio_service:
				audio_service.play_success()
			current_question += 1
			await get_tree().create_timer(0.32).timeout
			if phase == "playing":
				_load_endless_question()
			return
		status_label.text = "答对了！+%d 分" % earned
		status_label.add_theme_color_override("font_color", ACCENT)
		_pulse_control(status_label, 1.035, 0.18)
		if audio_service:
			audio_service.play_success()
		if celebration_fx:
			celebration_fx.success(game_panel.get_global_rect().get_center())
			_pulse_control(game_panel, 1.012, 0.22)
		current_question += 1
		if current_question >= level_puzzles.size():
			_finish_level(true, "全部题目完成")
		else:
			await get_tree().create_timer(0.45).timeout
			if phase == "playing":
				_load_question()
	else:
		if friend_mode:
			cards = original_cards.duplicate(true)
			level_combo = 0
			selected_index = -1
			selected_operator = ""
			undo_stack.clear()
			question_used_operators.clear()
			status_label.text = "%d 不是 24，扣 5 秒，请继续尝试" % answer
			status_label.add_theme_color_override("font_color", WARNING)
			_render_cards()
			if audio_service:
				audio_service.play_error()
		elif endless_mode:
			cards = original_cards.duplicate(true)
			level_combo = 0
			endless_fast_streak = 0
			endless_score_multiplier = 1.0
			selected_index = -1
			selected_operator = ""
			undo_stack.clear()
			question_used_operators.clear()
			status_label.text = "%d 不是 24，已恢复本题，请继续尝试" % answer
			status_label.add_theme_color_override("font_color", WARNING)
			_render_cards()
			if audio_service:
				audio_service.play_error()
		else:
			status_label.text = "%d 不是 24，点击“重置本题”再试" % answer
			status_label.add_theme_color_override("font_color", WARNING)
			_pulse_control(status_label, 1.02, 0.14)
			level_combo = 0
			mistakes += 1
			level_attempts.append(MatchDataScript.create_attempt(str(current_puzzle["puzzle_id"]), int(question_elapsed * 1000.0), false, mistakes, 0))


func _endless_miss(reason: String) -> void:
	if not endless_mode or phase != "playing" or endless_transitioning:
		return
	endless_transitioning = true
	mistakes += 1
	level_combo = 0
	status_label.text = "%s，本局结束" % reason
	status_label.add_theme_color_override("font_color", DANGER)
	_finish_level(false, reason)


func _daily_operator_allowed(operator: String) -> bool:
	var forbidden := str(daily_config.get("forbidden_operator", ""))
	if forbidden == operator:
		return false
	if forbidden == "-" and operator == "−":
		return false
	return true


func _daily_requirement_met() -> bool:
	var required := str(daily_config.get("required_operator", ""))
	if required.is_empty():
		return true
	if required == "−":
		return question_used_operators.has("−") or question_used_operators.has("-")
	return question_used_operators.has(required)


func _undo() -> void:
	if phase != "playing":
		return
	if friend_mode and not free_undo_available:
		status_label.text = "好友对战每局只能免费撤销一次"
		status_label.add_theme_color_override("font_color", WARNING)
		return
	if daily_mode and daily_rule_id == "no_undo":
		status_label.text = "今日规则：不能撤销，每一步都要想清楚"
		status_label.add_theme_color_override("font_color", WARNING)
		return
	if undo_stack.is_empty():
		status_label.text = "还没有可以撤销的步骤"
		return
	if not free_undo_available:
		if not _consume_rewarded_ad("undo"):
			status_label.text = "今日广告次数已用完，暂时不能撤销"
			status_label.add_theme_color_override("font_color", WARNING)
			return
		status_label.text = "已观看广告，获得一次撤销"
		status_label.add_theme_color_override("font_color", ACCENT)
	else:
		free_undo_available = false
	cards = undo_stack.pop_back()
	selected_index = -1
	selected_operator = ""
	status_label.text = "已撤销上一步"
	status_label.add_theme_color_override("font_color", ACCENT)
	_render_cards()


func _use_hint() -> void:
	if phase != "playing":
		return
	var hint_allowed := _hint_rule_allowed()
	if not hint_allowed:
		status_label.text = "本关不允许使用提示"
		return
	if not free_hint_available:
		if not _consume_rewarded_ad("hint"):
			status_label.text = "今日广告次数已用完，暂时不能获得提示"
			return
		status_label.text = "已观看广告，获得一次提示"
		status_label.add_theme_color_override("font_color", ACCENT)
	else:
		free_hint_available = false
	hint_used = true
	var hint_step := _extract_first_step(str(current_puzzle["solution"]))
	if hint_step.is_empty():
		status_label.text = "提示：先任选两个数字开始尝试"
	else:
		status_label.text = "提示：第一步，先点 %s，再点 %s，最后点 %s" % [hint_step["first"], hint_step["operator"], hint_step["second"]]
	status_label.add_theme_color_override("font_color", WARNING)
	_update_hud()


func _hint_rule_allowed() -> bool:
	if friend_mode:
		return false
	if daily_mode:
		return true
	if endless_mode:
		return bool(EndlessModeScript.config_for_question(current_question, {
			"speed_ratio": endless_speed_ratio,
			"fast_streak": endless_fast_streak,
		}).get("allow_hint", false))
	if current_level < 0 or current_level >= levels.size():
		return false
	return bool(levels[current_level].get("allow_hint", false))


func _consume_rewarded_ad(reward_type: String) -> bool:
	if friend_mode or not ad_service or not ad_service.show_rewarded(reward_type):
		return false
	progress["ads"] = ad_service.usage()
	SaveServiceScript.save_progress(progress)
	return true


func _prepare_result_ad(mode: String, reward_coins: int = 0) -> void:
	result_ad_mode = mode
	result_ad_reward_coins = maxi(0, reward_coins)
	result_ad_claimed = false
	if not result_ad_button:
		return
	result_ad_button.visible = not friend_mode and not mode.is_empty() and (mode == "continue" or result_ad_reward_coins > 0) and ad_service.is_available()
	result_ad_button.disabled = not result_ad_button.visible
	if mode == "continue":
		result_ad_button.text = "▶ 看广告继续本题（+20秒）"
	else:
		result_ad_button.text = "▶ 看广告领取额外 +%d 金币" % result_ad_reward_coins


func _watch_result_ad() -> void:
	if result_ad_claimed or result_ad_mode.is_empty() or friend_mode:
		return
	if not _consume_rewarded_ad(result_ad_mode):
		result_ad_button.text = "今日广告奖励已用完"
		result_ad_button.disabled = true
		return
	result_ad_claimed = true
	result_ad_button.visible = false
	if result_ad_mode == "continue":
		phase = "playing"
		level_time_left = 20.0
		question_elapsed = 0.0
		status_label.text = "广告奖励已生效，再给你 20 秒！"
		status_label.add_theme_color_override("font_color", ACCENT)
		if audio_service:
			audio_service.start_countdown()
		_show_only(game_panel)
		_update_hud()
		_render_cards()
		return
	var bonus := RewardServiceScript.claim_ad_coin_bonus(progress, result_ad_reward_coins)
	SaveServiceScript.save_progress(progress)
	result_detail.text += "\n广告奖励：+%d 金币" % bonus


func _extract_first_step(solution: String) -> Dictionary:
	var open_positions: Array = []
	for index in range(solution.length()):
		var character := solution[index]
		if character == "(":
			open_positions.append(index)
		elif character == ")" and not open_positions.is_empty():
			var start: int = int(open_positions.pop_back())
			var inside := solution.substr(start + 1, index - start - 1).strip_edges()
			for operator in ["+", "-", "×", "÷"]:
				var operator_index := inside.find(operator)
				if operator_index > 0:
					var display_operator: String = "−" if operator == "-" else operator
					return {
						"first": inside.substr(0, operator_index).strip_edges(),
						"operator": display_operator,
						"second": inside.substr(operator_index + operator.length()).strip_edges(),
					}
	return {}


func _finish_level(passed: bool, reason: String) -> void:
	if phase != "playing":
		return
	phase = "result"
	if result_record_label:
		result_record_label.text = ""
	if result_reward_label:
		result_reward_label.text = ""
	if result_share_button:
		result_share_button.visible = false
	if audio_service:
		audio_service.stop_countdown()
	if endless_mode:
		var endless_reward := RewardServiceScript.claim_endless_reward(progress, level_score, endless_solved_questions, max_combo, SaveServiceScript.today_key())
		var endless_config := EndlessModeScript.config_for_question(maxi(0, endless_solved_questions - 1))
		var reached_stage := int(endless_config.get("stage", 0))
		SaveServiceScript.save_endless_result(progress, level_score, endless_solved_questions, max_combo, reached_stage)
		LeaderboardServiceScript.submit_score(progress, LeaderboardServiceScript.MODE_ENDLESS, level_score, {
			"questions": endless_solved_questions,
			"max_combo": max_combo,
			"stage": reached_stage,
		})
		SaveServiceScript.save_progress(progress)
		result_title.text = "无尽模式结束"
		result_title.add_theme_color_override("font_color", ACCENT)
		result_stars.text = "无限"
		var endless: Dictionary = progress.get("endless", {})
		var record_text := "刷新个人纪录！" if bool(endless_reward["new_questions_record"]) or bool(endless_reward["new_score_record"]) else "继续挑战，冲击个人纪录"
		result_stats.text = "本局得分：%d\n连续答对：%d 题    难度阶段：%s\n最高倍率：%.2fx" % [level_score, endless_solved_questions, str(endless_config.get("stage_name", "热身")), endless_score_multiplier]
		result_detail.text = "%s\n%s\n本局奖励：+%d 金币（答题 %d · 难度 %d · 里程碑 %d · 连击 %d）\n今日无尽奖励：%d / %d 金币\n历史最高：%d 分 · %d 题 · %d 连击" % [str(reason), record_text, int(endless_reward["coins"]), int(endless_reward["base_coins"]), int(endless_reward["stage_coins"]), int(endless_reward["milestone_coins"]), int(endless_reward["combo_coins"]), int(endless_reward["daily_earned"]), int(endless_reward["daily_cap"]), int(endless.get("best_score", 0)), int(endless.get("best_questions", 0)), int(endless.get("best_combo", 0))]
		result_reward_label.text = "奖励明细：答题、难度、里程碑和连击都会逐步增加，但每日有上限。"
		result_record_label.text = "新纪录！" if record_text.contains("刷新") else "再答几题，就能刷新自己的纪录"
		_append_achievement_notice()
		_prepare_result_ad("coins", int(endless_reward["coins"]))
		result_restart_button.visible = true
		next_button.visible = true
		next_button.text = "返回首页"
		result_back_button.visible = false
		_show_only(result_panel)
		return
	if friend_mode:
		var opponent := FriendMatchServiceScript.opponent_snapshot(friend_opponent_plan, FriendMatchServiceScript.TIME_LIMIT - level_time_left, friend_puzzles.size())
		var friend_result := FriendMatchServiceScript.calculate_result(current_question, level_score, mistakes, FriendMatchServiceScript.TIME_LIMIT - level_time_left, opponent)
		var friend_reward := RewardServiceScript.claim_friend_match_reward(progress, friend_result, SaveServiceScript.today_key())
		if str(friend_result.get("outcome", "")) == "win":
			TaskServiceScript.record_weekly(progress, "weekly_friend", 1, SaveServiceScript.today_key())
		last_friend_result = friend_result.duplicate(true)
		SaveServiceScript.save_progress(progress)
		var outcome := str(friend_result.get("outcome", "draw"))
		result_title.text = "好友对战胜利！" if outcome == "win" else ("平局！" if outcome == "draw" else "好友更快！")
		result_title.add_theme_color_override("font_color", ACCENT if outcome == "win" else WARNING)
		result_stars.text = "胜利" if outcome == "win" else ("平局" if outcome == "draw" else "惜败")
		result_stats.text = "我：答对 %d 题 · %d 分 · %.1f 秒\n好友：答对 %d 题 · %d 分 · %.1f 秒" % [int(friend_result["player_solved"]), int(friend_result["player_score"]), float(friend_result["player_elapsed"]), int(friend_result["opponent_solved"]), int(friend_result["opponent_score"]), float(friend_result["opponent_elapsed"])]
		result_detail.text = "%s\n%s\n胜负规则：答对题数优先，其次比较分数。" % [str(reason), str(friend_reward.get("message", "对战奖励已记录"))]
		result_reward_label.text = "好友对战奖励：每日最多领取 3 局，胜利奖励更高。"
		result_share_button.visible = true
		_prepare_result_ad("", 0)
		result_restart_button.visible = true
		next_button.visible = true
		next_button.text = "返回首页"
		result_back_button.visible = false
		_show_only(result_panel)
		return
	var config: Dictionary = levels[current_level]
	var stars := 0
	if daily_mode:
		if passed:
			SaveServiceScript.save_daily_result(progress, daily_date_key, level_score)
			var daily: Dictionary = progress.get("daily", {})
			var daily_achievements: Array = []
			if int(daily.get("streak", 0)) >= 3:
				daily_achievements.append("daily_3")
			if int(daily.get("streak", 0)) >= 7:
				daily_achievements.append("daily_7")
			_unlock_achievements(daily_achievements)
			var reward := RewardServiceScript.claim_daily_reward(progress, daily_date_key, level_score, level_puzzles.size(), hint_used)
			TaskServiceScript.record_weekly(progress, "weekly_daily", 1, SaveServiceScript.today_key())
			LeaderboardServiceScript.submit_score(progress, LeaderboardServiceScript.MODE_DAILY, level_score, {
				"date": daily_date_key,
				"questions": level_puzzles.size(),
				"hint_used": hint_used,
			})
			SaveServiceScript.save_progress(progress)
			result_title.text = "每日挑战完成"
			result_title.add_theme_color_override("font_color", ACCENT)
			result_stars.text = "✦✦✦"
			result_detail.text = "%s\n今日规则：%s\n连续挑战：%d 天    每日最高分：%d" % [str(reward.get("message", "奖励已记录")), str(daily_config.get("rule_title", "")), int(daily.get("streak", 0)), int(daily.get("best_score", 0))]
			result_reward_label.text = "连续挑战第 3 / 7 天会获得额外金币。"
			_append_achievement_notice()
			_prepare_result_ad("coins", int(reward.get("coins", 0)))
		else:
			result_title.text = "每日挑战失败"
			result_title.add_theme_color_override("font_color", WARNING)
			result_stars.text = "☆"
			result_detail.text = "%s。明天仍可继续挑战。" % reason
			_prepare_result_ad("continue" if _is_timeout_reason(reason) else "", 0)
		result_stats.text = "得分：%d\n剩余时间：%.1f 秒" % [level_score, level_time_left]
		next_button.text = "返回首页"
		result_back_button.visible = false
		_show_only(result_panel)
		return
	if passed:
		var old_level_record: Dictionary = progress.get("levels", {}).get(str(current_level), {})
		var new_level_clear := int(old_level_record.get("best_score", 0)) <= 0
		stars = 1
		if level_score >= int(config["target_score"]):
			stars += 1
		if max_combo >= int(config["target_combo"]) and mistakes == 0 and not hint_used:
			stars += 1
		var campaign_achievements: Array = ["first_clear"]
		if stars >= 3:
			campaign_achievements.append("three_star")
		if mistakes == 0 and not hint_used:
			campaign_achievements.append("perfect_clear")
		_unlock_achievements(campaign_achievements)
		TaskServiceScript.record(progress, "campaign_clear", 1, SaveServiceScript.today_key())
		TaskServiceScript.record_weekly(progress, "weekly_campaign", 1, SaveServiceScript.today_key())
		var level_coins := RewardServiceScript.claim_level_reward(progress, current_level, stars)
		var chapter_index := int(config.get("chapter_index", int(current_level / 20)))
		var chapter_complete := current_level % 20 == 19
		var milestone_reward := RewardServiceScript.claim_campaign_bonus(progress, current_level, stars, chapter_index, chapter_complete, new_level_clear)
		SaveServiceScript.save_level(progress, current_level, stars, level_score)
		var campaign_total_score := LeaderboardServiceScript.campaign_score(progress)
		LeaderboardServiceScript.submit_score(progress, LeaderboardServiceScript.MODE_CAMPAIGN, campaign_total_score, {
			"level": current_level + 1,
			"stars": stars,
			"level_score": level_score,
			"max_combo": max_combo,
		})
		SaveServiceScript.save_progress(progress)
		result_title.text = "第 %d 关完成" % (current_level + 1)
		result_title.add_theme_color_override("font_color", ACCENT)
		result_stars.text = "★".repeat(stars) + "☆".repeat(3 - stars)
		result_detail.text = "评价条件：完成 + 达到 %d 分 + 连击达到 %d 且无错误无提示\n本关奖励：+%d 金币\n章节：第 %d 章 · %s" % [int(config["target_score"]), int(config["target_combo"]), level_coins, chapter_index + 1, str(config.get("chapter_name", ""))]
		result_reward_label.text = "额外里程碑：%s" % ("、".join(milestone_reward["labels"]) if not milestone_reward["labels"].is_empty() else "本次没有重复奖励")
		result_record_label.text = "三星达成！" if stars >= 3 else "再挑战一次，争取点亮全部三星"
		_append_achievement_notice()
		_prepare_result_ad("coins", level_coins)
	else:
		result_title.text = "挑战失败"
		result_title.add_theme_color_override("font_color", WARNING)
		result_stars.text = "☆☆☆"
		result_detail.text = "%s。别急，题目会重新生成并且保证有解。" % reason
		_prepare_result_ad("continue" if _is_timeout_reason(reason) else "", 0)
	var used_time := maxf(0.0, timer_max_time - level_time_left)
	result_stats.text = "得分：%d\n最高连击：%d    错误：%d\n已完成题目：%d / %d    用时：%.1f 秒" % [level_score, max_combo, mistakes, current_question, level_puzzles.size(), used_time]
	next_button.text = "下一关" if current_level < levels.size() - 1 else "返回关卡"
	result_back_button.visible = true
	_show_only(result_panel)
	_refresh_level_buttons()


func _is_timeout_reason(reason: String) -> bool:
	return reason.contains("时间") or reason.contains("超时")


func _restart_level() -> void:
	if friend_mode:
		_start_friend_match()
	elif endless_mode:
		_start_endless_mode()
	elif daily_mode:
		_start_daily_challenge()
	else:
		_start_level(current_level)


func _unlock_achievements(achievement_ids: Array) -> void:
	if achievement_ids.is_empty():
		return
	var new_unlocks := AchievementServiceScript.unlock_many(progress, achievement_ids)
	if new_unlocks.is_empty():
		return
	for achievement in new_unlocks:
		session_achievement_unlocks.append(achievement)
	SaveServiceScript.save_progress(progress)
	if status_label and phase == "playing":
		status_label.text = "解锁成就：%s  +%d 金币" % [str(new_unlocks[0].get("title", "新成就")), int(new_unlocks[0].get("reward", 0))]
		status_label.add_theme_color_override("font_color", ACCENT)


func _append_achievement_notice() -> void:
	var notice := AchievementServiceScript.format_unlocks(session_achievement_unlocks)
	if not notice.is_empty():
		result_detail.text += "\n\n" + notice


func _pulse_control(control: Control, peak_scale: float, duration: float) -> void:
	if not is_instance_valid(control):
		return
	control.pivot_offset = control.size * 0.5
	var tween := create_tween()
	tween.set_trans(Tween.TRANS_BACK).set_ease(Tween.EASE_OUT)
	tween.tween_property(control, "scale", Vector2(peak_scale, peak_scale), duration * 0.42)
	tween.tween_property(control, "scale", Vector2.ONE, duration * 0.58)


func _on_game_back_pressed() -> void:
	if daily_mode or endless_mode or friend_mode:
		_show_home()
	else:
		_show_menu()


func _go_next_level() -> void:
	if endless_mode or friend_mode:
		_show_home()
		return
	if daily_mode:
		_show_home()
		return
	if current_level < levels.size() - 1 and SaveServiceScript.is_unlocked(progress, current_level + 1):
		_start_level(current_level + 1)
	else:
		_show_menu()


func _open_campaign() -> void:
	_show_menu()
	if not bool(progress.get("tutorial_seen", false)):
		tutorial_overlay.visible = true


func _show_home() -> void:
	phase = "home"
	TaskServiceScript.ensure_day(progress, SaveServiceScript.today_key())
	AchievementServiceScript.ensure_progress(progress)
	var unlocked_level := mini(levels.size(), int(progress.get("unlocked_level", 0)) + 1)
	if campaign_button:
		campaign_button.text = "闯关模式  ·  进度 1-%d" % unlocked_level
	if home_coins_label:
		home_coins_label.text = "金币  %d" % int(progress.get("coins", 0))
	if endless_status_label:
		var endless: Dictionary = progress.get("endless", {})
		endless_status_label.text = "最高：%d 分 · %d 题 · %d 连击" % [int(endless.get("best_score", 0)), int(endless.get("best_questions", 0)), int(endless.get("best_combo", 0))]
	if daily_button and daily_status_label:
		var date_key := SaveServiceScript.today_key()
		var daily: Dictionary = progress.get("daily", {})
		var streak := int(daily.get("streak", 0))
		if SaveServiceScript.is_daily_completed(progress, date_key):
			daily_button.text = "✓  今日挑战已完成"
			daily_status_label.text = "连续挑战 %d 天 · 明天再来领取新规则" % streak
		else:
			daily_button.text = "✦  每日三题挑战"
			daily_status_label.text = "固定题目 + 每日规则 · 完成可得金币"
	_refresh_home_tasks()
	_refresh_home_achievements()
	_show_only(home_panel)
	_refresh_audio_settings_ui()
	home_requested.emit()


func _toggle_audio_settings() -> void:
	if daily_tasks_panel:
		daily_tasks_panel.visible = false
	if more_panel:
		more_panel.visible = false
	if audio_settings_panel:
		audio_settings_panel.visible = not audio_settings_panel.visible
		if audio_settings_panel.visible:
			audio_service.resume_music()
			_refresh_audio_settings_ui()


func _toggle_daily_tasks() -> void:
	if audio_settings_panel:
		audio_settings_panel.visible = false
	if more_panel:
		more_panel.visible = false
	if daily_tasks_panel:
		daily_tasks_panel.visible = not daily_tasks_panel.visible
		if daily_tasks_panel.visible:
			_refresh_home_tasks()
			_refresh_home_achievements()
			audio_service.resume_music()


func _toggle_more_panel() -> void:
	if audio_settings_panel:
		audio_settings_panel.visible = false
	if daily_tasks_panel:
		daily_tasks_panel.visible = false
	if more_panel:
		more_panel.visible = not more_panel.visible
		if more_panel.visible:
			audio_service.resume_music()


func _toggle_music_enabled() -> void:
	var enabled := not bool(progress.get("audio", {}).get("music_enabled", true))
	var audio: Dictionary = progress.get("audio", {})
	audio["music_enabled"] = enabled
	progress["audio"] = audio
	audio_service.set_music_enabled(enabled)
	SaveServiceScript.save_progress(progress)
	_refresh_audio_settings_ui()


func _toggle_sfx_enabled() -> void:
	var enabled := not bool(progress.get("audio", {}).get("sfx_enabled", true))
	var audio: Dictionary = progress.get("audio", {})
	audio["sfx_enabled"] = enabled
	progress["audio"] = audio
	audio_service.set_sfx_enabled(enabled)
	SaveServiceScript.save_progress(progress)
	_refresh_audio_settings_ui()
	if enabled:
		audio_service.play_click()


func _on_music_volume_changed(value: float) -> void:
	var volume_db := lerpf(-36.0, -6.0, value)
	var audio: Dictionary = progress.get("audio", {})
	audio["music_volume_db"] = volume_db
	progress["audio"] = audio
	audio_service.set_music_volume(volume_db)
	SaveServiceScript.save_progress(progress)
	if audio_music_volume_label:
		audio_music_volume_label.text = "背景音乐音量  %d%%" % int(round(value * 100.0))


func _on_sfx_volume_changed(value: float) -> void:
	var volume_db := lerpf(-24.0, 0.0, value)
	var audio: Dictionary = progress.get("audio", {})
	audio["sfx_volume_db"] = volume_db
	progress["audio"] = audio
	audio_service.set_sfx_volume(volume_db)
	SaveServiceScript.save_progress(progress)
	if audio_sfx_volume_label:
		audio_sfx_volume_label.text = "按键音效音量  %d%%" % int(round(value * 100.0))


func _next_music_track() -> void:
	var next_track: int = (audio_service.get_music_track() + 1) % maxi(1, audio_service.get_music_track_count())
	var audio: Dictionary = progress.get("audio", {})
	audio["music_track"] = next_track
	progress["audio"] = audio
	audio_service.set_music_track(next_track)
	SaveServiceScript.save_progress(progress)
	_refresh_audio_settings_ui()


func _refresh_audio_settings_ui() -> void:
	if not audio_settings_panel or not audio_music_toggle:
		return
	var audio: Dictionary = progress.get("audio", {})
	var music_enabled := bool(audio.get("music_enabled", true))
	var sfx_enabled := bool(audio.get("sfx_enabled", true))
	var music_volume := clampf(inverse_lerp(-36.0, -6.0, float(audio.get("music_volume_db", -25.0))), 0.0, 1.0)
	var sfx_volume := clampf(inverse_lerp(-24.0, 0.0, float(audio.get("sfx_volume_db", -9.0))), 0.0, 1.0)
	audio_music_toggle.text = "♫ 背景音乐：开启" if music_enabled else "♫ 背景音乐：关闭"
	audio_sfx_toggle.text = "♪ 按键音效：开启" if sfx_enabled else "♪ 按键音效：关闭"
	audio_track_button.text = "更换音乐：%s  ›" % audio_service.get_music_track_name()
	audio_music_volume_label.text = "背景音乐音量  %d%%" % int(round(music_volume * 100.0))
	audio_sfx_volume_label.text = "按键音效音量  %d%%" % int(round(sfx_volume * 100.0))
	if not is_equal_approx(audio_music_slider.value, music_volume):
		audio_music_slider.set_value_no_signal(music_volume)
	if not is_equal_approx(audio_sfx_slider.value, sfx_volume):
		audio_sfx_slider.set_value_no_signal(sfx_volume)
	_style_button(audio_music_toggle, BLUE if music_enabled else SURFACE_2)
	_style_button(audio_sfx_toggle, BLUE if sfx_enabled else SURFACE_2)


func _refresh_home_tasks() -> void:
	if not home_task_label:
		return
	var snapshot: Dictionary = TaskServiceScript.snapshot(progress, SaveServiceScript.today_key())
	var parts: Array[String] = []
	for task_id in ["campaign_clear", "endless_questions", "combo"]:
		var task: Dictionary = snapshot[task_id]
		var mark := "✓" if bool(task["claimed"]) else "%d/%d" % [int(task["value"]), int(task["target"])]
		parts.append("%s  %s  ·  +%d 金币" % [mark, str(task["title"]), int(task["reward"])])
	home_task_label.text = "\n".join(parts)
	if weekly_task_label:
		var weekly := TaskServiceScript.weekly_snapshot(progress, SaveServiceScript.today_key())
		var weekly_parts: Array[String] = []
		for task_id in ["weekly_campaign", "weekly_daily", "weekly_endless", "weekly_friend"]:
			var task: Dictionary = weekly[task_id]
			var mark := "✓" if bool(task["claimed"]) else "%d/%d" % [int(task["value"]), int(task["target"])]
			weekly_parts.append("%s  %s · +%d" % [mark, str(task["title"]), int(task["reward"])])
		weekly_task_label.text = "\n".join(weekly_parts)


func _refresh_home_achievements() -> void:
	if not home_achievement_label:
		return
	var unlocked_count := AchievementServiceScript.unlocked_count(progress)
	var total_count := AchievementServiceScript.all().size()
	var next_achievement := AchievementServiceScript.next_hint(progress)
	if next_achievement.is_empty():
		home_achievement_label.text = "✅ 全部成就已解锁！继续保持神奇表现"
	else:
		home_achievement_label.text = "徽章 %d/%d　·　下个目标：%s（+%d 金币）" % [unlocked_count, total_count, str(next_achievement.get("title", "新成就")), int(next_achievement.get("reward", 0))]


func _show_stats() -> void:
	phase = "stats"
	var summary := PlayerStatsScript.summary(progress)
	stats_summary_label.text = "已答对 %d 题\n最高连击 %d · 最快 %.1f 秒\n最高关卡 第 %d 关 · 第 %d 章" % [int(summary["total_solved"]), int(summary["best_combo"]), float(summary["fastest_ms"]) / 1000.0 if int(summary["fastest_ms"]) > 0 else 0.0, int(summary["best_level"]), int(summary["best_chapter"])]
	var mode_questions: Dictionary = summary["mode_questions"]
	stats_detail_label.text = "累计得分：%d\n最常使用：%s（%d 次）\n闯关答题：%d 题\n每日答题：%d 题\n无尽答题：%d 题\n\n继续挑战，刷新自己的速度和连击记录。" % [int(summary["total_score"]), str(summary["favorite_operator"]), int(summary["favorite_count"]), int(mode_questions.get("campaign", 0)), int(mode_questions.get("daily", 0)), int(mode_questions.get("endless", 0))]
	_show_only(stats_panel)


func _share_match_result() -> void:
	if last_friend_result.is_empty():
		return
	var payload := ShareServiceScript.create_match_result_payload(last_friend_result, friend_room)
	var card_text := ShareServiceScript.build_result_card_text(last_friend_result)
	if platform_adapter and platform_adapter.share(payload):
		DisplayServer.clipboard_set(card_text)
		result_detail.text += "\n战绩卡片已生成，微信版将打开分享面板。"
		if celebration_fx:
			celebration_fx.toast("战绩卡片已复制", ACCENT, 1.0)


func _apply_chapter_music(chapter_index: int) -> void:
	var audio: Dictionary = progress.get("audio", {})
	if not bool(audio.get("chapter_music_auto", true)) or not audio_service:
		return
	var track_count := maxi(1, audio_service.get_music_track_count())
	var track_index := posmod(chapter_index, track_count)
	audio["music_track"] = track_index
	progress["audio"] = audio
	audio_service.set_music_track(track_index)
	SaveServiceScript.save_progress(progress)



func _show_shop() -> void:
	phase = "shop"
	_refresh_shop()
	_show_only(shop_panel)


func _show_leaderboard() -> void:
	phase = "leaderboard"
	LeaderboardServiceScript.ensure_progress(progress)
	_refresh_leaderboard()
	_show_only(leaderboard_panel)


func _select_leaderboard_board(board_id: String) -> void:
	leaderboard_board_id = board_id
	_refresh_leaderboard()


func _select_leaderboard_mode(mode_id: String) -> void:
	leaderboard_mode_id = mode_id
	_refresh_leaderboard()


func _refresh_leaderboard() -> void:
	if not is_instance_valid(leaderboard_panel) or not is_instance_valid(leaderboard_list):
		return
	_refresh_leaderboard_tab_styles()
	for child in leaderboard_list.get_children():
		child.queue_free()
	var entries: Array = LeaderboardServiceScript.get_entries(progress, leaderboard_board_id, leaderboard_mode_id)
	var player_score := LeaderboardServiceScript.player_score(progress, leaderboard_mode_id)
	var player_rank := 0
	for entry in entries:
		if bool(entry.get("is_player", false)):
			player_rank = int(entry.get("rank", 0))
			break
	leaderboard_summary_label.text = "%s · %s\n我的成绩：%d 分    当前排名：第 %d 名" % [LeaderboardServiceScript.board_name(leaderboard_board_id), LeaderboardServiceScript.mode_name(leaderboard_mode_id), player_score, player_rank]
	for entry in entries:
		var row := PanelContainer.new()
		row.custom_minimum_size = Vector2(0, 58)
		var row_style := StyleBoxFlat.new()
		row_style.bg_color = Color(0.42, 0.28, 0.62, 0.94) if bool(entry.get("is_player", false)) else Color(0.14, 0.11, 0.32, 0.9)
		row_style.border_width_left = 1
		row_style.border_width_top = 1
		row_style.border_width_right = 1
		row_style.border_width_bottom = 1
		row_style.border_color = Color(1.0, 0.83, 0.4, 0.95) if bool(entry.get("is_player", false)) else Color(0.52, 0.66, 0.98, 0.5)
		row_style.corner_radius_top_left = 14
		row_style.corner_radius_top_right = 14
		row_style.corner_radius_bottom_left = 14
		row_style.corner_radius_bottom_right = 14
		row_style.content_margin_left = 14
		row_style.content_margin_right = 14
		row_style.content_margin_top = 8
		row_style.content_margin_bottom = 8
		row.add_theme_stylebox_override("panel", row_style)
		leaderboard_list.add_child(row)
		var row_content := HBoxContainer.new()
		row_content.add_theme_constant_override("separation", 10)
		row.add_child(row_content)
		var rank_label := _label("%d" % int(entry["rank"]), 20, ACCENT if int(entry["rank"]) <= 3 else MUTED)
		rank_label.custom_minimum_size = Vector2(36, 40)
		rank_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
		row_content.add_child(rank_label)
		var name_label := _label("%s\n%s" % [str(entry["name"]), str(entry["subtitle"])], 15, TEXT)
		name_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
		name_label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		name_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
		row_content.add_child(name_label)
		var score_label_row := _label("%d 分" % int(entry["score"]), 17, ACCENT if bool(entry.get("is_player", false)) else TEXT)
		score_label_row.custom_minimum_size = Vector2(104, 40)
		score_label_row.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT
		score_label_row.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
		row_content.add_child(score_label_row)
	if leaderboard_status_label:
		leaderboard_status_label.text = "排行榜按各模式最高分排序。当前为本地原型数据；正式接入微信后，好友榜将读取微信好友开放数据域。"


func _refresh_leaderboard_tab_styles() -> void:
	if not is_instance_valid(leaderboard_board_friends_button):
		return
	_style_button(leaderboard_board_friends_button, BLUE if leaderboard_board_id == LeaderboardServiceScript.BOARD_FRIENDS else SURFACE_2)
	_style_button(leaderboard_board_global_button, BLUE if leaderboard_board_id == LeaderboardServiceScript.BOARD_GLOBAL else SURFACE_2)
	_style_button(leaderboard_campaign_button, GOLD if leaderboard_mode_id == LeaderboardServiceScript.MODE_CAMPAIGN else SURFACE_2)
	_style_button(leaderboard_daily_button, GOLD if leaderboard_mode_id == LeaderboardServiceScript.MODE_DAILY else SURFACE_2)
	_style_button(leaderboard_endless_button, GOLD if leaderboard_mode_id == LeaderboardServiceScript.MODE_ENDLESS else SURFACE_2)


func _refresh_shop() -> void:
	if not shop_list:
		return
	shop_coins_label.text = "金币 %d" % int(progress.get("coins", 0))
	for child in shop_list.get_children():
		child.queue_free()
	for skin in SkinCatalogScript.all():
		var skin_id := str(skin["id"])
		var owned: Array = progress.get("owned_skins", ["classic"])
		var is_owned := owned.has(skin_id)
		var is_equipped := str(progress.get("equipped_skin", "classic")) == skin_id
		var unlock_status: Dictionary = SkinCatalogScript.unlock_status(skin_id, progress)
		var row := _panel_box()
		var row_content := HBoxContainer.new()
		row_content.add_theme_constant_override("separation", 10)
		row.add_child(row_content)
		var status_text := "已拥有" if is_owned else str(unlock_status.get("reason", ""))
		var name_label := _label("%s\n%s\n%s" % [str(skin["name"]), str(skin["description"]), status_text] , 15, TEXT)
		name_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
		name_label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		name_label.custom_minimum_size = Vector2(0, 72)
		name_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
		row_content.add_child(name_label)
		var action := Button.new()
		action.custom_minimum_size = Vector2(128, 52)
		action.focus_mode = Control.FOCUS_NONE
		if is_equipped:
			action.text = "已装备"
			action.disabled = true
			_style_button(action, SURFACE_2)
		elif is_owned:
			action.text = "装备"
			action.pressed.connect(_equip_skin.bind(skin_id))
			_bind_click_audio(action)
			_style_button(action, BLUE)
		else:
			action.text = "%d 金币" % int(skin["price"])
			action.disabled = not bool(unlock_status.get("unlocked", false)) or int(progress.get("coins", 0)) < int(skin["price"])
			action.pressed.connect(_buy_skin.bind(skin_id))
			_bind_click_audio(action)
			_style_button(action, GOLD)
		row_content.add_child(action)
		shop_list.add_child(row)
	shop_status_label.text = "当前主题：%s · 皮肤只改变外观" % str(SkinCatalogScript.get_skin(str(progress.get("equipped_skin", "classic")))["name"])


func _buy_skin(skin_id: String) -> void:
	var skin := SkinCatalogScript.get_skin(skin_id)
	var price := int(skin.get("price", 0))
	var unlock_status: Dictionary = SkinCatalogScript.unlock_status(skin_id, progress)
	if not bool(unlock_status.get("unlocked", false)):
		shop_status_label.text = str(unlock_status.get("reason", "还未满足兑换条件"))
		return
	if int(progress.get("coins", 0)) < price:
		shop_status_label.text = "金币还不够，先去完成关卡或每日挑战吧"
		return
	var owned: Array = progress.get("owned_skins", ["classic"])
	if not owned.has(skin_id):
		owned.append(skin_id)
		progress["owned_skins"] = owned
		progress["coins"] = int(progress.get("coins", 0)) - price
		_unlock_achievements(["skin_unlock"])
		SaveServiceScript.save_progress(progress)
		shop_status_label.text = "已兑换「%s」，现在可以装备了" % str(skin["name"])
	_refresh_shop()


func _equip_skin(skin_id: String) -> void:
	var owned: Array = progress.get("owned_skins", ["classic"])
	if not owned.has(skin_id):
		return
	progress["equipped_skin"] = skin_id
	SaveServiceScript.save_progress(progress)
	_apply_skin(skin_id)
	_refresh_shop()
	_show_home()


func _apply_skin(skin_id: String) -> void:
	var theme: Dictionary = SkinCatalogScript.theme_colors(skin_id)
	BG = Color(str(theme.get("bg", "#064633")))
	SURFACE = Color(str(theme.get("surface", "#0b5a42")))
	SURFACE_2 = Color(str(theme.get("surface_2", "#126b4d")))
	CARD = Color(str(theme.get("card", "#fff4d1")))
	CARD_SELECTED = CARD.lightened(0.12)
	CARD_TEXT = Color(str(theme.get("card_text", "#145438")))
	TEXT = Color(str(theme.get("text", "#fff9e9")))
	MUTED = Color(str(theme.get("muted", "#b9d8c5")))
	ACCENT = Color(str(theme.get("accent", "#ffd34d")))
	BLUE = Color(str(theme.get("blue", "#2eaa68")))
	GOLD = Color(str(theme.get("gold", "#e8b544")))
	if background_gradient:
		background_gradient.colors = PackedColorArray([BG.lightened(0.08), BG.darkened(0.18)])
	if background_texture:
		background_texture.queue_redraw()
	if is_instance_valid(home_panel):
		_refresh_theme_controls()
	if is_instance_valid(cartoon_decor):
		cartoon_decor.configure(BG, SURFACE, MUTED, ACCENT, GOLD)
	if is_instance_valid(timer_badge):
		timer_badge.configure_palette(SURFACE, TEXT, MUTED, ACCENT, WARNING, DANGER)
	if is_instance_valid(celebration_fx):
		celebration_fx.configure_palette(ACCENT, BLUE, Color("#f39adf"), Color("#7ee6a8"), TEXT)


func _refresh_theme_controls() -> void:
	if home_coins_panel:
		_style_coin_badge(home_coins_panel)
	if campaign_button:
		_style_home_button(campaign_button, Color("#5ecdf2"))
	if daily_button:
		_style_home_button(daily_button, Color("#ffe28b"))
	if shop_button:
		_style_home_button(shop_button, Color("#9beab8"))
	if achievements_button:
		_style_home_button(achievements_button, Color("#c69bff"))
	if leaderboard_button:
		_style_home_button(leaderboard_button, Color("#ffc775"))
	if endless_button:
		_style_home_button(endless_button, Color("#eea5df"))
	if menu_prev_button:
		_style_button(menu_prev_button, SURFACE_2)
	if menu_next_button:
		_style_button(menu_next_button, BLUE)
	if undo_button:
		_style_button(undo_button, SURFACE_2)
	if hint_button:
		_style_button(hint_button, BLUE.darkened(0.18))
	if reset_button:
		_style_button(reset_button, SURFACE_2)
	if level_grid:
		_refresh_level_buttons()
	for button in operator_buttons:
		_style_operator(button, BLUE)
	_refresh_leaderboard_tab_styles()


func _style_coin_badge(panel: PanelContainer) -> void:
	var style := StyleBoxFlat.new()
	style.bg_color = Color(ACCENT.r, ACCENT.g, ACCENT.b, 0.18)
	style.border_width_left = 2
	style.border_width_top = 2
	style.border_width_right = 2
	style.border_width_bottom = 2
	style.border_color = Color(ACCENT.r, ACCENT.g, ACCENT.b, 0.35)
	style.corner_radius_top_left = 24
	style.corner_radius_top_right = 24
	style.corner_radius_bottom_left = 24
	style.corner_radius_bottom_right = 24
	style.shadow_color = Color(0, 0, 0, 0.2)
	style.shadow_size = 5
	style.shadow_offset = Vector2(0, 2)
	style.content_margin_left = 8
	style.content_margin_right = 8
	style.content_margin_top = 5
	style.content_margin_bottom = 5
	panel.add_theme_stylebox_override("panel", style)


func _show_menu() -> void:
	phase = "level_select"
	# 根据已解锁进度自动定位：解锁到第 21 关时，打开第 2 页。
	var unlocked_level := maxi(int(progress.get("unlocked_level", 0)), int(progress.get("last_level", 0)))
	menu_page = clampi(int(unlocked_level / LEVELS_PER_PAGE), 0, maxi(0, _page_count() - 1))
	_show_only(menu_panel)
	_refresh_chapter_banner()
	_refresh_level_buttons()


func _page_count() -> int:
	return ceili(float(levels.size()) / float(LEVELS_PER_PAGE))


func _next_level_page() -> void:
	if menu_page < _page_count() - 1:
		menu_page += 1
		_refresh_level_buttons()


func _previous_level_page() -> void:
	if menu_page > 0:
		menu_page -= 1
		_refresh_level_buttons()


func _show_only(panel: Control) -> void:
	if audio_settings_panel:
		audio_settings_panel.visible = false
	if daily_tasks_panel:
		daily_tasks_panel.visible = false
	if more_panel:
		more_panel.visible = false
	if more_panel:
		more_panel.visible = false
	home_panel.visible = panel == home_panel
	menu_panel.visible = panel == menu_panel
	game_panel.visible = panel == game_panel
	result_panel.visible = panel == result_panel
	shop_panel.visible = panel == shop_panel
	achievements_panel.visible = panel == achievements_panel
	friend_panel.visible = panel == friend_panel
	leaderboard_panel.visible = panel == leaderboard_panel
	if stats_panel:
		stats_panel.visible = panel == stats_panel
	var visible_panel := panel
	visible_panel.modulate = Color(1.0, 1.0, 1.0, 0.0)
	var fade := create_tween()
	fade.set_trans(Tween.TRANS_QUAD).set_ease(Tween.EASE_OUT)
	fade.tween_property(visible_panel, "modulate", Color.WHITE, 0.18)


func _play_merge_feedback() -> void:
	if feedback_tween != null:
		feedback_tween.kill()
	status_label.modulate = Color(1.0, 1.0, 1.0, 0.72)
	feedback_tween = create_tween()
	feedback_tween.tween_property(status_label, "modulate", Color.WHITE, 0.18).set_trans(Tween.TRANS_QUAD).set_ease(Tween.EASE_OUT)


func _bind_click_audio(button: Button) -> void:
	if not is_instance_valid(button):
		return
	button.pressed.connect(func() -> void:
		if audio_service:
			audio_service.resume_music()
			audio_service.play_click()
	)


func _on_button_down(button: Button) -> void:
	if not is_instance_valid(button):
		return
	button.pivot_offset = button.size * 0.5
	button.scale = Vector2(0.97, 0.97)
	button.modulate = Color(1.0, 0.9, 0.72, 1.0)


func _on_button_up(button: Button) -> void:
	if not is_instance_valid(button):
		return
	button.scale = Vector2.ONE
	button.modulate = Color.WHITE


func _render_cards() -> void:
	for child in numbers_row.get_children():
		child.queue_free()
	for index in range(cards.size()):
		var card: Dictionary = cards[index]
		var button := Button.new()
		button.text = str(card["value"])
		button.custom_minimum_size = Vector2(0, 104 if friend_mode else 126)
		button.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		button.add_theme_font_size_override("font_size", 38)
		button.focus_mode = Control.FOCUS_NONE
		button.pressed.connect(_on_card_pressed.bind(index))
		button.button_down.connect(_on_button_down.bind(button))
		button.button_up.connect(_on_button_up.bind(button))
		if selected_index == index:
			_style_card(button, CARD_SELECTED, true)
		else:
			_style_card(button, CARD, false)
		numbers_row.add_child(button)
	for index in range(operator_buttons.size()):
		var operator_button: Button = operator_buttons[index]
		operator_button.custom_minimum_size = Vector2(0, 52 if friend_mode else 62)
		var operator: String = ["+", "−", "×", "÷"][index]
		_style_operator(operator_button, BLUE if selected_operator == operator else SURFACE_2)
	_update_action_buttons()


func _update_action_buttons() -> void:
	if not undo_button:
		return
	var undo_forbidden := daily_mode and daily_rule_id == "no_undo"
	var friend_undo_used := friend_mode and not free_undo_available
	undo_button.disabled = undo_stack.is_empty() or undo_forbidden or friend_undo_used
	undo_button.text = "不可撤销" if undo_forbidden else ("本局仅限一次撤销" if friend_undo_used else ("撤销（免费）" if free_undo_available else "撤销（广告）"))
	reset_button.disabled = cards.is_empty()
	hint_button.text = "提示（免费）" if free_hint_available else "提示（广告）"
	hint_button.disabled = friend_mode or not _hint_rule_allowed()
	hint_button.visible = not friend_mode


func _update_hud() -> void:
	if not level_label:
		return
	_update_navigation_buttons()
	var config: Dictionary = levels[current_level]
	if endless_mode:
		var endless_config := EndlessModeScript.config_for_question(current_question, {
			"speed_ratio": endless_speed_ratio,
			"fast_streak": endless_fast_streak,
		})
		level_label.text = "无尽 · 第 %d 题" % (current_question + 1)
		score_label.text = "得分 %d" % level_score
		combo_label.text = "连击 %d · %.2fx · %s" % [level_combo, endless_score_multiplier, str(endless_config["stage_name"])]
		var endless_record := int(progress.get("endless", {}).get("best_questions", 0))
		var record_hint := "已突破个人纪录" if endless_solved_questions >= endless_record and endless_solved_questions > 0 else "距离纪录还差 %d 题" % maxi(0, endless_record - endless_solved_questions)
		question_label.text = "连续答对：%d 题    %s" % [endless_solved_questions, record_hint]
		if question_progress_bar:
			question_progress_bar.max_value = 5.0
			question_progress_bar.value = float((endless_solved_questions % 5) + 1)
		if question_target_label:
			question_target_label.text = "目标 24"
		if rule_label:
			rule_label.text = "无尽规则：答对继续，答错可重试，只有超时才结束"
		_update_action_buttons()
		if timer_badge:
			timer_badge.set_timer(level_time_left, timer_max_time, true)
		return
	if friend_mode:
		level_label.text = "好友对战 · 第 %d 题" % (current_question + 1)
		score_label.text = "得分 %d" % level_score
		combo_label.text = "连击 %d" % level_combo
		combo_label.add_theme_color_override("font_color", ACCENT if level_combo >= 2 else MUTED)
		question_label.text = "我已答对：%d / %d    目标：24" % [current_question, friend_puzzles.size()]
		if question_progress_bar:
			question_progress_bar.max_value = maxf(1.0, float(friend_puzzles.size()))
			question_progress_bar.value = float(current_question + 1)
		if question_target_label:
			question_target_label.text = "同题竞速"
		if rule_label:
			rule_label.text = "好友对战：同一套题 · 不允许提示 · 答错扣 5 秒"
		_update_action_buttons()
		if timer_badge:
			timer_badge.set_timer(level_time_left, timer_max_time, false)
		return
	level_label.text = "每日挑战" if daily_mode else "第 %d 关" % (current_level + 1)
	score_label.text = "得分 %d" % level_score
	combo_label.text = "连击 %d" % level_combo
	combo_label.add_theme_color_override("font_color", ACCENT if level_combo >= 2 else MUTED)
	question_label.text = "第 %d / %d 题    目标：24" % [current_question + 1, level_puzzles.size()]
	if question_progress_bar:
		question_progress_bar.max_value = maxf(1.0, float(level_puzzles.size()))
		question_progress_bar.value = float(current_question + 1)
	if question_target_label:
		question_target_label.text = "目标 24"
	if rule_label:
		rule_label.text = ("今日规则：%s" % str(daily_config.get("rule_text", ""))) if daily_mode else ""
	_update_action_buttons()
	if timer_badge:
		timer_badge.set_timer(level_time_left, timer_max_time, false)


func _update_friend_race() -> void:
	if not friend_mode or not is_instance_valid(friend_race_label):
		return
	var elapsed := FriendMatchServiceScript.TIME_LIMIT - level_time_left
	friend_opponent_progress = FriendMatchServiceScript.opponent_snapshot(friend_opponent_plan, elapsed, friend_puzzles.size())
	var opponent_solved := int(friend_opponent_progress.get("solved", 0))
	var opponent_score := int(friend_opponent_progress.get("score", 0))
	var lead_text := "你领先" if current_question > opponent_solved else ("好友领先" if current_question < opponent_solved else "暂时平手")
	friend_race_label.text = "我　%d / %d 题　·　%d 分\n好友·小满　%d / %d 题　·　%d 分　　%s" % [current_question, friend_puzzles.size(), level_score, opponent_solved, friend_puzzles.size(), opponent_score, lead_text]
	friend_race_label.add_theme_color_override("font_color", ACCENT if current_question >= opponent_solved else WARNING)


func _update_navigation_buttons() -> void:
	if game_back_button:
		game_back_button.text = "返回首页" if daily_mode or endless_mode or friend_mode else "返回关卡"


func _refresh_level_buttons() -> void:
	if not level_grid:
		return
	var first_level := menu_page * LEVELS_PER_PAGE
	_refresh_chapter_banner()
	for slot in range(level_grid.get_child_count()):
		var button: Button = level_grid.get_child(slot)
		var level_index := first_level + slot
		if level_index >= levels.size():
			button.visible = false
			continue
		button.visible = true
		var config: Dictionary = levels[level_index]
		var record: Dictionary = progress.get("levels", {}).get(str(level_index), {})
		var stars := int(record.get("stars", 0))
		var unlocked := SaveServiceScript.is_unlocked(progress, level_index)
		var current := level_index == int(progress.get("last_level", 0)) and unlocked
		button.disabled = not unlocked
		var stars_text := "★".repeat(stars) + "☆".repeat(3 - stars)
		# 当前关卡使用边框和发光区分，文字保持与其他卡片完全一致。
		var challenge_mark := " · 挑战" if bool(config.get("is_challenge", false)) else ""
		button.text = "第 %d 关%s\n%s\n%s" % [level_index + 1, challenge_mark, config["title"], stars_text] if unlocked else "第 %d 关%s\n%s\n锁定" % [level_index + 1, challenge_mark, config["title"]]
		_style_level_button(button, unlocked, current, stars)
	if menu_page_label:
		menu_page_label.text = "第 %d / %d 页" % [menu_page + 1, _page_count()]
	if menu_prev_button:
		menu_prev_button.disabled = menu_page <= 0
	if menu_next_button:
		menu_next_button.disabled = menu_page >= _page_count() - 1


func _refresh_chapter_banner() -> void:
	if not chapter_banner or levels.is_empty():
		return
	var first_level := clampi(menu_page * LEVELS_PER_PAGE, 0, levels.size() - 1)
	var config: Dictionary = levels[first_level]
	var chapter_index := int(config.get("chapter_index", int(first_level / 20)))
	var chapter_name := str(config.get("chapter_name", "第 %d 章" % (chapter_index + 1)))
	var chapter_subtitle := str(config.get("chapter_subtitle", "继续挑战"))
	var chapter_goal := str(config.get("chapter_goal", "完成关卡"))
	var chapter_start := chapter_index * 20
	var chapter_end := mini(chapter_start + 20, levels.size())
	var unlocked_in_chapter := clampi(int(progress.get("unlocked_level", 0)) - chapter_start, 0, chapter_end - chapter_start)
	chapter_title_label.text = "第 %d 章 · %s" % [chapter_index + 1, chapter_name]
	chapter_subtitle_label.text = chapter_subtitle
	chapter_progress_label.text = "章节进度 %d / %d · %s" % [unlocked_in_chapter, chapter_end - chapter_start, chapter_goal]
	var accent := Color(str(config.get("chapter_color", "#22d3ee")))
	chapter_title_label.add_theme_color_override("font_color", accent)
	var style := chapter_banner.get_theme_stylebox("panel") as StyleBoxFlat
	if style:
		style.border_color = Color(accent.r, accent.g, accent.b, 0.62)
		style.shadow_color = Color(accent.r, accent.g, accent.b, 0.2)


func _style_level_button(button: Button, unlocked: bool, current: bool, stars: int) -> void:
	var style := StyleBoxFlat.new()
	style.bg_color = Color(BLUE.r, BLUE.g, BLUE.b, 0.28) if current else (Color(1.0, 1.0, 1.0, 0.08) if unlocked else Color(0.03, 0.02, 0.12, 0.22))
	style.border_width_left = 2 if not current else 3
	style.border_width_top = style.border_width_left
	style.border_width_right = style.border_width_left
	style.border_width_bottom = style.border_width_left
	style.border_color = Color(ACCENT.r, ACCENT.g, ACCENT.b, 0.98) if current else (Color(1.0, 1.0, 1.0, 0.15) if unlocked else Color(0.55, 0.58, 0.78, 0.18))
	style.corner_radius_top_left = 18
	style.corner_radius_top_right = 18
	style.corner_radius_bottom_left = 18
	style.corner_radius_bottom_right = 18
	style.shadow_color = Color(BLUE.r, BLUE.g, BLUE.b, 0.32) if current else Color(0.0, 0.0, 0.0, 0.22)
	style.shadow_size = 12 if current else 6
	style.shadow_offset = Vector2(0, 3)
	button.add_theme_stylebox_override("normal", style)
	var hover := style.duplicate()
	hover.bg_color = Color(BLUE.r, BLUE.g, BLUE.b, 0.42) if unlocked else style.bg_color
	hover.border_color = Color(BLUE.r, BLUE.g, BLUE.b, 1.0) if unlocked else style.border_color
	button.add_theme_stylebox_override("hover", hover)
	var pressed := hover.duplicate()
	pressed.bg_color = Color(BLUE.r, BLUE.g, BLUE.b, 0.58) if unlocked else style.bg_color
	button.add_theme_stylebox_override("pressed", pressed)
	button.add_theme_stylebox_override("focus", hover)
	button.add_theme_color_override("font_color", TEXT if unlocked else MUTED)
	button.add_theme_color_override("font_hover_color", Color.WHITE)
	button.add_theme_color_override("font_pressed_color", Color.WHITE)
	button.add_theme_color_override("font_outline_color", Color(0.04, 0.02, 0.14, 0.95))
	button.add_theme_constant_override("outline_size", 2)


func _panel_box() -> PanelContainer:
	var panel := PanelContainer.new()
	var style := StyleBoxFlat.new()
	# 统一采用首页的深蓝紫玻璃卡片，内容页也保持同一套视觉语言。
	style.bg_color = Color(1.0, 1.0, 1.0, 0.08)
	style.border_width_left = 2
	style.border_width_top = 2
	style.border_width_right = 2
	style.border_width_bottom = 2
	style.border_color = Color(1.0, 1.0, 1.0, 0.15)
	style.corner_radius_top_left = 22
	style.corner_radius_top_right = 22
	style.corner_radius_bottom_left = 22
	style.corner_radius_bottom_right = 22
	style.content_margin_left = 18
	style.content_margin_right = 18
	style.content_margin_top = 14
	style.content_margin_bottom = 14
	style.shadow_color = Color(0.0, 0.0, 0.0, 0.35)
	style.shadow_size = 16
	style.shadow_offset = Vector2(0, 4)
	panel.add_theme_stylebox_override("panel", style)
	return panel


func _label(text: String, font_size: int, color: Color) -> Label:
	var label := Label.new()
	label.text = text
	label.add_theme_font_size_override("font_size", font_size)
	label.add_theme_color_override("font_color", color)
	# 深色玻璃背景上的白字统一加深色描边，避免文字融入背景。
	label.add_theme_color_override("font_outline_color", Color(0.05, 0.03, 0.16, 0.9))
	label.add_theme_constant_override("outline_size", 2)
	label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	label.clip_text = true
	return label


func _style_button(button: Button, color: Color) -> void:
	var normal := StyleBoxFlat.new()
	var glass_color := Color(color.r, color.g, color.b, 0.42)
	normal.bg_color = glass_color
	normal.border_width_left = 2
	normal.border_width_top = 2
	normal.border_width_right = 2
	normal.border_width_bottom = 2
	normal.border_color = Color(1.0, 1.0, 1.0, 0.15)
	normal.corner_radius_top_left = 20
	normal.corner_radius_top_right = 20
	normal.corner_radius_bottom_left = 20
	normal.corner_radius_bottom_right = 20
	normal.content_margin_left = 10
	normal.content_margin_right = 10
	normal.content_margin_top = 8
	normal.content_margin_bottom = 8
	normal.shadow_color = Color(color.r, color.g, color.b, 0.24)
	normal.shadow_size = 10
	normal.shadow_offset = Vector2(0, 3)
	var hover = normal.duplicate()
	hover.bg_color = Color(color.lightened(0.12).r, color.lightened(0.12).g, color.lightened(0.12).b, 0.62)
	hover.border_color = Color(0.13, 0.83, 0.93, 0.95)
	hover.shadow_size = 12
	var pressed = normal.duplicate()
	pressed.bg_color = Color(color.lightened(0.2).r, color.lightened(0.2).g, color.lightened(0.2).b, 0.72)
	pressed.border_color = Color(0.13, 0.83, 0.93, 1.0)
	button.add_theme_stylebox_override("normal", normal)
	button.add_theme_stylebox_override("hover", hover)
	button.add_theme_stylebox_override("pressed", pressed)
	button.add_theme_stylebox_override("focus", hover)
	button.add_theme_color_override("font_color", TEXT)
	button.add_theme_color_override("font_hover_color", Color.WHITE)
	button.add_theme_color_override("font_pressed_color", Color.WHITE)
	button.add_theme_color_override("font_outline_color", Color(0.05, 0.03, 0.16, 0.9))
	button.add_theme_constant_override("outline_size", 2)


func _style_home_button(button: Button, color: Color) -> void:
	var normal := StyleBoxFlat.new()
	normal.bg_color = Color(color.r, color.g, color.b, 0.48)
	normal.border_width_left = 3
	normal.border_width_top = 3
	normal.border_width_right = 3
	normal.border_width_bottom = 3
	normal.border_color = Color(1.0, 1.0, 1.0, 0.22)
	normal.corner_radius_top_left = 34
	normal.corner_radius_top_right = 34
	normal.corner_radius_bottom_left = 34
	normal.corner_radius_bottom_right = 34
	normal.content_margin_left = 16
	normal.content_margin_right = 16
	normal.content_margin_top = 12
	normal.content_margin_bottom = 12
	normal.shadow_color = Color(color.r, color.g, color.b, 0.32)
	normal.shadow_size = 12
	normal.shadow_offset = Vector2(0, 4)
	var hover = normal.duplicate()
	hover.bg_color = Color(color.lightened(0.1).r, color.lightened(0.1).g, color.lightened(0.1).b, 0.68)
	hover.border_color = Color(0.13, 0.83, 0.93, 1.0)
	hover.shadow_size = 14
	var pressed = normal.duplicate()
	pressed.bg_color = Color(color.lightened(0.18).r, color.lightened(0.18).g, color.lightened(0.18).b, 0.78)
	button.add_theme_stylebox_override("normal", normal)
	button.add_theme_stylebox_override("hover", hover)
	button.add_theme_stylebox_override("pressed", pressed)
	button.add_theme_stylebox_override("focus", hover)
	button.add_theme_color_override("font_color", TEXT)
	button.add_theme_color_override("font_hover_color", Color.WHITE)
	button.add_theme_color_override("font_pressed_color", Color.WHITE)
	button.add_theme_color_override("font_outline_color", Color(0.05, 0.03, 0.16, 0.94))
	button.add_theme_constant_override("outline_size", 2)


func _style_operator(button: Button, color: Color) -> void:
	_style_button(button, color)
	for state in ["normal", "hover", "pressed", "focus"]:
		var style = button.get_theme_stylebox(state)
		if style is StyleBoxFlat:
			style.corner_radius_top_left = 24
			style.corner_radius_top_right = 24
			style.corner_radius_bottom_left = 24
			style.corner_radius_bottom_right = 24


func _style_card(button: Button, color: Color, selected: bool = false) -> void:
	var normal := StyleBoxFlat.new()
	# 数字卡保持轻玻璃；选中时用青绿色强调，而不是整块变成厚重的黄卡。
	normal.bg_color = Color(1.0, 1.0, 1.0, 0.10) if not selected else Color(BLUE.r, BLUE.g, BLUE.b, 0.72)
	normal.border_width_left = 3
	normal.border_width_top = 3
	normal.border_width_right = 3
	normal.border_width_bottom = 3
	normal.border_color = Color(BLUE.r, BLUE.g, BLUE.b, 1.0) if selected else Color(1.0, 1.0, 1.0, 0.15)
	normal.corner_radius_top_left = 20
	normal.corner_radius_top_right = 20
	normal.corner_radius_bottom_left = 20
	normal.corner_radius_bottom_right = 20
	normal.content_margin_left = 10
	normal.content_margin_right = 10
	normal.content_margin_top = 8
	normal.content_margin_bottom = 8
	normal.shadow_color = Color(BLUE.r, BLUE.g, BLUE.b, 0.34) if selected else Color(0.0, 0.0, 0.0, 0.28)
	normal.shadow_size = 12 if selected else 8
	normal.shadow_offset = Vector2(0, 3)
	var hover = normal.duplicate()
	hover.bg_color = Color(BLUE.r, BLUE.g, BLUE.b, 0.34)
	hover.border_color = Color(BLUE.r, BLUE.g, BLUE.b, 0.92)
	var pressed = normal.duplicate()
	pressed.bg_color = Color(BLUE.r, BLUE.g, BLUE.b, 0.56)
	pressed.border_color = Color.WHITE
	button.add_theme_stylebox_override("normal", normal)
	button.add_theme_stylebox_override("hover", hover)
	button.add_theme_stylebox_override("pressed", pressed)
	button.add_theme_stylebox_override("focus", hover)
	button.add_theme_color_override("font_color", Color.WHITE)
	button.add_theme_color_override("font_hover_color", Color.WHITE)
	button.add_theme_color_override("font_pressed_color", Color.WHITE)
	button.add_theme_color_override("font_outline_color", Color(0.05, 0.03, 0.16, 0.94))
	button.add_theme_constant_override("outline_size", 2)
