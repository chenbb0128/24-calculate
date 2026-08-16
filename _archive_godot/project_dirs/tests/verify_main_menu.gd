extends SceneTree

const MainMenuScene = preload("res://scenes/MainMenu.tscn")


func _init() -> void:
	var menu := MainMenuScene.instantiate()
	root.add_child(menu)
	await process_frame

	assert(menu.get_node("StarsContainer").get_child_count() == 60, "新版主菜单应生成 60 颗星星")
	assert(menu.get_node("FloatNums").get_child_count() == 6, "新版主菜单应生成 6 个漂浮数字")
	assert(menu.get_node("Content").visible, "启动时应显示新版主菜单")
	assert(not menu.get_node("GameController").visible, "启动时不应显示旧游戏控制器")
	assert(menu.get_node("Content/LevelBtn/LevelTitle").text == "闯关模式", "闯关按钮标题未加载")
	assert(menu.get_node("Content/TopBar/CoinPanel/CoinHBox/CoinNum").text.is_valid_int(), "金币未从存档更新")
	var viewport_size := Vector2(720, 1280)
	var content := menu.get_node("Content") as Control
	for node_path in ["LevelBtn", "FriendBtn", "EndlessBtn", "ModeRow/DailyBtn", "ModeRow/TaskBtn", "ModeRow/MoreBtn", "ProgressPanel"]:
		var control := content.get_node(node_path) as Control
		assert(control.position.x >= 0.0 and control.position.y >= 0.0, "%s 位置不能为负" % node_path)
		assert(control.position.x + control.size.x <= content.size.x + 1.0, "%s 横向超出内容区域" % node_path)
		assert(control.position.y + control.size.y <= content.size.y + 1.0, "%s 纵向超出内容区域" % node_path)
	var daily_button := content.get_node("ModeRow/DailyBtn") as Control
	var daily_title := content.get_node("ModeRow/DailyBtn/DailyTitle") as Control
	var daily_label := content.get_node("ModeRow/DailyBtn/DailyStatus") as Control
	assert(not daily_title.get_global_rect().intersects(daily_label.get_global_rect()), "每日挑战标题和状态文字发生重叠")
	assert(daily_title.get_global_rect().encloses(Rect2(daily_title.get_global_rect().position, daily_title.get_global_rect().size)), "每日挑战标题布局无效")

	menu._on_level_pressed()
	await process_frame
	assert(not menu.get_node("Content").visible, "进入闯关模式后主菜单应隐藏")
	assert(menu.get_node("GameController").visible, "进入闯关模式后游戏控制器应显示")
	assert(menu.get_node("GameController").phase == "level_select", "闯关按钮未跳转到关卡选择")
	assert(menu.get_node("GameController")._page_count() == 10, "闯关模式应有 10 页关卡")
	var level_grid: GridContainer = menu.get_node("GameController").level_grid
	var game_controller := menu.get_node("GameController")
	assert(not game_controller.chapter_title_label.get_global_rect().intersects(game_controller.chapter_subtitle_label.get_global_rect()), "章节标题和副标题发生重叠")
	assert(not game_controller.chapter_banner.get_global_rect().intersects(game_controller.level_intro_instruction_label.get_global_rect()), "章节说明和操作提示发生重叠")
	var first_card := level_grid.get_child(0) as Control
	for card_index in range(level_grid.get_child_count()):
		var card := level_grid.get_child(card_index) as Control
		assert(absf(card.size.x - first_card.size.x) < 1.0, "关卡卡片宽度不一致，网格比例被撑开")
	assert(not str(first_card.text).contains("正在挑战"), "当前关卡状态文字不应撑大卡片")

	menu.get_node("GameController")._show_home()
	await process_frame
	assert(menu.get_node("Content").visible, "返回首页后新版主菜单应显示")
	assert(not menu.get_node("GameController").visible, "返回首页后游戏控制器应隐藏")

	menu._on_setting_pressed()
	assert(menu.settings_popup.visible, "设置按钮未打开新版设置弹窗")
	assert(not menu.get_node("GameController/AudioSettingsPanel").visible, "设置按钮不应打开旧版设置面板")
	menu._on_setting_pressed()
	assert(not menu.settings_popup.visible, "设置弹窗无法关闭")

	menu._on_task_pressed()
	assert(menu.tasks_popup.visible, "每日任务按钮未打开新版任务弹窗")
	assert(not menu.get_node("GameController/DailyTasksPanel").visible, "每日任务按钮不应打开旧版任务面板")
	var viewport_center: Vector2 = menu.get_viewport_rect().size * 0.5
	assert(menu.tasks_popup.get_global_rect().get_center().distance_to(viewport_center) < 2.0, "每日任务弹窗没有居中")
	menu._on_task_pressed()
	assert(not menu.tasks_popup.visible, "任务弹窗无法关闭")

	menu._on_more_pressed()
	assert(not menu.get_node("Content/ModeRow/MoreBtn").disabled, "更多功能按钮不应禁用")
	assert(menu.more_popup.visible, "更多功能按钮未打开新版功能弹窗")
	assert(menu.more_popup.get_global_rect().get_center().distance_to(viewport_center) < 2.0, "更多功能弹窗没有居中")
	menu._close_all_menu_popups()

	menu.daily_done = true
	menu._on_daily_pressed()
	assert(menu.toast_panel.visible, "今日挑战已完成时应给出明确提示")

	print("Main menu verification passed: stars, floating numbers, save data, navigation and return")
	quit()
