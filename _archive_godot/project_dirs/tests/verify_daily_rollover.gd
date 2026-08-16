extends SceneTree

const SaveServiceScript = preload("res://services/save_service.gd")
const TaskServiceScript = preload("res://services/task_service.gd")

func _init() -> void:
	var progress := {"coins": 0, "tasks": {"date": "2026-08-12", "values": {"campaign_clear": 1}, "claimed": {"campaign_clear": true}}}
	TaskServiceScript.ensure_day(progress, "2026-08-13")
	assert(str(progress["tasks"]["date"]) == "2026-08-13", "跨日后任务日期应更新")
	assert(int(progress["tasks"]["values"].get("campaign_clear", 0)) == 0, "跨日后每日任务应清零")
	assert(not SaveServiceScript.is_daily_completed({"daily": {"completed": {"2026-08-12": true}}}, "2026-08-13"), "旧日期完成记录不能算作今日完成")
	assert(SaveServiceScript.today_key().length() == 10, "今日日期应使用 YYYY-MM-DD")
	assert(SaveServiceScript.today_seed() > 20000000, "今日种子应来自本地日期")
	print("Daily rollover verification passed: local date, midnight task reset and daily completion isolation")
	quit()
