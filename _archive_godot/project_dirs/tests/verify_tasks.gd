extends SceneTree

const TaskServiceScript = preload("res://services/task_service.gd")

func _init() -> void:
	var progress := {"coins": 0}
	TaskServiceScript.ensure_day(progress, "2026-08-13")
	var first := TaskServiceScript.record(progress, "campaign_clear", 1, "2026-08-13")
	assert(bool(first["completed"]), "完成闯关任务应达成")
	assert(int(first["reward"]) == 15, "闯关任务奖励错误")
	assert(int(progress["coins"]) == 15, "闯关任务金币错误")
	var repeat := TaskServiceScript.record(progress, "campaign_clear", 1, "2026-08-13")
	assert(int(repeat["reward"]) == 0, "同一任务不能重复领取")
	var endless := TaskServiceScript.record(progress, "endless_questions", 5, "2026-08-13")
	assert(int(endless["reward"]) == 25, "无尽任务奖励错误")
	var combo := TaskServiceScript.record_max(progress, "combo", 5, "2026-08-13")
	assert(int(combo["reward"]) == 20, "连击任务奖励错误")
	assert(int(progress["coins"]) == 60, "任务奖励累计错误")
	var next_day := TaskServiceScript.snapshot(progress, "2026-08-14")
	assert(int(next_day["campaign_clear"]["value"]) == 0, "新的一天应重置任务")
	print("Task verification passed: daily reset, progress and one-time rewards")
	quit()
