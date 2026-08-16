class_name TaskService
extends RefCounted

## 每日目标服务。
## 目标数据独立于界面，未来可把 progress 读写替换成微信云存档。

const TASKS := {
	"campaign_clear": {"title": "完成 1 个闯关关卡", "target": 1, "reward": 15},
	"endless_questions": {"title": "无尽模式答对 5 题", "target": 5, "reward": 25},
	"combo": {"title": "单局连击达到 5", "target": 5, "reward": 20},
}

const WEEKLY_TASKS := {
	"weekly_campaign": {"title": "本周完成 5 个闯关关卡", "target": 5, "reward": 45},
	"weekly_daily": {"title": "本周完成 3 次每日挑战", "target": 3, "reward": 35},
	"weekly_endless": {"title": "本周无尽模式答对 20 题", "target": 20, "reward": 50},
	"weekly_friend": {"title": "本周好友对战获胜 2 次", "target": 2, "reward": 40},
}


static func ensure_day(progress: Dictionary, date_key: String) -> void:
	var tasks: Dictionary = progress.get("tasks", {})
	if str(tasks.get("date", "")) != date_key:
		tasks = {"date": date_key, "values": {}, "claimed": {}}
	progress["tasks"] = tasks
	ensure_week(progress, date_key)


static func ensure_week(progress: Dictionary, date_key: String) -> void:
	var week: Dictionary = progress.get("weekly_tasks", {})
	var week_key := week_key_for_date(date_key)
	if str(week.get("week", "")) != week_key:
		week = {"week": week_key, "values": {}, "claimed": {}}
	progress["weekly_tasks"] = week


static func week_key_for_date(date_key: String) -> String:
	var timestamp := Time.get_unix_time_from_datetime_string(date_key + "T00:00:00")
	return str(int(timestamp / 604800.0))


static func record(progress: Dictionary, task_id: String, amount: int, date_key: String) -> Dictionary:
	return _record_internal(progress, task_id, amount, date_key, false)


static func record_max(progress: Dictionary, task_id: String, amount: int, date_key: String) -> Dictionary:
	return _record_internal(progress, task_id, amount, date_key, true)


static func snapshot(progress: Dictionary, date_key: String) -> Dictionary:
	ensure_day(progress, date_key)
	var tasks: Dictionary = progress.get("tasks", {})
	var values: Dictionary = tasks.get("values", {})
	var result: Dictionary = {}
	for task_id in TASKS.keys():
		var config: Dictionary = TASKS[task_id]
		result[task_id] = {
			"title": str(config["title"]),
			"target": int(config["target"]),
			"value": mini(int(config["target"]), int(values.get(task_id, 0))),
			"reward": int(config["reward"]),
			"claimed": bool(tasks.get("claimed", {}).get(task_id, false)),
		}
	return result


static func record_weekly(progress: Dictionary, task_id: String, amount: int, date_key: String) -> Dictionary:
	if not WEEKLY_TASKS.has(task_id):
		return {"reward": 0, "completed": false}
	ensure_week(progress, date_key)
	var week: Dictionary = progress.get("weekly_tasks", {})
	var values: Dictionary = week.get("values", {})
	var claimed: Dictionary = week.get("claimed", {})
	var config: Dictionary = WEEKLY_TASKS[task_id]
	var next_value := mini(int(config["target"]), int(values.get(task_id, 0)) + amount)
	values[task_id] = next_value
	var completed := next_value >= int(config["target"])
	var reward := 0
	if completed and not bool(claimed.get(task_id, false)):
		claimed[task_id] = true
		reward = int(config["reward"])
		progress["coins"] = maxi(0, int(progress.get("coins", 0)) + reward)
	week["values"] = values
	week["claimed"] = claimed
	progress["weekly_tasks"] = week
	return {"task_id": task_id, "title": str(config["title"]), "value": next_value, "target": int(config["target"]), "reward": reward, "completed": completed}


static func weekly_snapshot(progress: Dictionary, date_key: String) -> Dictionary:
	ensure_week(progress, date_key)
	var week: Dictionary = progress.get("weekly_tasks", {})
	var values: Dictionary = week.get("values", {})
	var result: Dictionary = {}
	for task_id in WEEKLY_TASKS.keys():
		var config: Dictionary = WEEKLY_TASKS[task_id]
		result[task_id] = {
			"title": str(config["title"]),
			"target": int(config["target"]),
			"value": mini(int(config["target"]), int(values.get(task_id, 0))),
			"reward": int(config["reward"]),
			"claimed": bool(week.get("claimed", {}).get(task_id, false)),
		}
	return result


static func _record_internal(progress: Dictionary, task_id: String, amount: int, date_key: String, use_max: bool) -> Dictionary:
	if not TASKS.has(task_id):
		return {"reward": 0, "completed": false}
	ensure_day(progress, date_key)
	var tasks: Dictionary = progress.get("tasks", {})
	var values: Dictionary = tasks.get("values", {})
	var claimed: Dictionary = tasks.get("claimed", {})
	var config: Dictionary = TASKS[task_id]
	var old_value := int(values.get(task_id, 0))
	var next_value := maxi(old_value, amount) if use_max else old_value + amount
	values[task_id] = mini(int(config["target"]), next_value)
	var completed := int(values[task_id]) >= int(config["target"])
	var reward := 0
	if completed and not bool(claimed.get(task_id, false)):
		claimed[task_id] = true
		reward = int(config["reward"])
		progress["coins"] = maxi(0, int(progress.get("coins", 0)) + reward)
	tasks["values"] = values
	tasks["claimed"] = claimed
	progress["tasks"] = tasks
	return {
		"task_id": task_id,
		"title": str(config["title"]),
		"value": int(values[task_id]),
		"target": int(config["target"]),
		"reward": reward,
		"completed": completed,
	}
