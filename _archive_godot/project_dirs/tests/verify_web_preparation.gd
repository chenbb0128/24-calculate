extends SceneTree

const MainMenuScene = preload("res://scenes/MainMenu.tscn")


func _init() -> void:
	assert(int(ProjectSettings.get_setting("display/window/size/viewport_width", 0)) == 720, "Web 预览应保持 720 宽度")
	assert(int(ProjectSettings.get_setting("display/window/size/viewport_height", 0)) == 1280, "Web 预览应保持 1280 高度")
	assert(str(ProjectSettings.get_setting("display/window/stretch/mode", "")) == "canvas_items", "Web 预览应使用 canvas_items 拉伸")
	assert(int(ProjectSettings.get_setting("display/window/handheld/orientation", 0)) == 1, "项目应保持竖屏方向")
	assert(bool(ProjectSettings.get_setting("input_devices/pointing/emulate_touch_from_mouse", false)), "桌面应开启鼠标模拟触摸")
	var menu := MainMenuScene.instantiate()
	root.add_child(menu)
	await process_frame
	var controls := [
		menu.get_node("Content/LevelBtn"),
		menu.get_node("Content/FriendBtn"),
		menu.get_node("Content/EndlessBtn"),
		menu.get_node("Content/ModeRow/DailyBtn"),
		menu.get_node("Content/ModeRow/TaskBtn"),
		menu.get_node("Content/ModeRow/MoreBtn"),
		menu.get_node("Content/TopBar/SettingBtn"),
	]
	for control in controls:
		assert(control is Button, "首页入口必须使用可触摸 Button")
		assert(control.mouse_filter == Control.MOUSE_FILTER_STOP, "首页按钮不能穿透触摸事件")
	print("Web preparation verification passed: portrait, stretch, touch simulation and menu controls")
	quit()
