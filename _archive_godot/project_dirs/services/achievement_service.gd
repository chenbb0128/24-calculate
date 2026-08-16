class_name AchievementService
extends RefCounted

## 成就数据与解锁规则独立于界面，后续接入微信云存档时只需替换存档读写层。
const ACHIEVEMENTS := [
	{"id": "first_clear", "title": "迈出第一步", "description": "完成第一个闯关关卡", "icon": "🚩", "reward": 30},
	{"id": "three_star", "title": "三星闪耀", "description": "首次获得三星评价", "icon": "🌟", "reward": 50},
	{"id": "perfect_clear", "title": "完美解题", "description": "无错误、无提示完成关卡", "icon": "💎", "reward": 80},
	{"id": "combo_5", "title": "连击小能手", "description": "单局连击达到 5", "icon": "⚡", "reward": 30},
	{"id": "combo_10", "title": "连击大师", "description": "单局连击达到 10", "icon": "🔥", "reward": 80},
	{"id": "endless_5", "title": "无尽热身", "description": "无尽模式连续答对 5 题", "icon": "♾", "reward": 30},
	{"id": "endless_10", "title": "无尽进阶", "description": "无尽模式连续答对 10 题", "icon": "🚀", "reward": 60},
	{"id": "endless_30", "title": "无尽传说", "description": "无尽模式连续答对 30 题", "icon": "👑", "reward": 150},
	{"id": "daily_3", "title": "三日坚持", "description": "连续完成每日挑战 3 天", "icon": "🌱", "reward": 60},
	{"id": "daily_7", "title": "一周坚持", "description": "连续完成每日挑战 7 天", "icon": "🏆", "reward": 120},
	{"id": "skin_unlock", "title": "换个心情", "description": "兑换第一个主题皮肤", "icon": "🎨", "reward": 40},
]


static func all() -> Array:
	return ACHIEVEMENTS.duplicate(true)


static func ensure_progress(progress: Dictionary) -> void:
	if not progress.has("achievements") or not (progress["achievements"] is Dictionary):
		progress["achievements"] = {"unlocked": {}, "claimed": {}}
		return
	var achievements: Dictionary = progress["achievements"]
	if not achievements.has("unlocked") or not (achievements["unlocked"] is Dictionary):
		achievements["unlocked"] = {}
	if not achievements.has("claimed") or not (achievements["claimed"] is Dictionary):
		achievements["claimed"] = {}
	progress["achievements"] = achievements


static func get_achievement(achievement_id: String) -> Dictionary:
	for achievement in ACHIEVEMENTS:
		if str(achievement["id"]) == achievement_id:
			return achievement.duplicate(true)
	return {}


static func is_unlocked(progress: Dictionary, achievement_id: String) -> bool:
	ensure_progress(progress)
	var achievements: Dictionary = progress["achievements"]
	var unlocked: Dictionary = achievements["unlocked"]
	return bool(unlocked.get(achievement_id, false))


static func unlock(progress: Dictionary, achievement_id: String) -> Dictionary:
	var achievement := get_achievement(achievement_id)
	if achievement.is_empty():
		return {}
	ensure_progress(progress)
	var achievements: Dictionary = progress["achievements"]
	var unlocked: Dictionary = achievements["unlocked"]
	if bool(unlocked.get(achievement_id, false)):
		return {}
	var claimed: Dictionary = achievements["claimed"]
	unlocked[achievement_id] = true
	claimed[achievement_id] = true
	achievements["unlocked"] = unlocked
	achievements["claimed"] = claimed
	progress["achievements"] = achievements
	var reward := int(achievement.get("reward", 0))
	progress["coins"] = maxi(0, int(progress.get("coins", 0)) + reward)
	var result := achievement.duplicate(true)
	result["newly_unlocked"] = true
	result["reward"] = reward
	return result


static func unlock_many(progress: Dictionary, achievement_ids: Array) -> Array:
	var new_unlocks: Array = []
	for achievement_id in achievement_ids:
		var unlocked := unlock(progress, str(achievement_id))
		if not unlocked.is_empty():
			new_unlocks.append(unlocked)
	return new_unlocks


static func unlocked_count(progress: Dictionary) -> int:
	ensure_progress(progress)
	var unlocked: Dictionary = progress["achievements"]["unlocked"]
	var count := 0
	for achievement in ACHIEVEMENTS:
		if bool(unlocked.get(str(achievement["id"]), false)):
			count += 1
	return count


static func next_hint(progress: Dictionary) -> Dictionary:
	ensure_progress(progress)
	var unlocked: Dictionary = progress["achievements"]["unlocked"]
	for achievement in ACHIEVEMENTS:
		if not bool(unlocked.get(str(achievement["id"]), false)):
			return achievement.duplicate(true)
	return {}


static func format_unlocks(new_unlocks: Array) -> String:
	if new_unlocks.is_empty():
		return ""
	var lines: Array[String] = ["🏅 新成就解锁！"]
	for achievement in new_unlocks:
		lines.append("%s %s  +%d 金币" % [str(achievement.get("icon", "🏅")), str(achievement.get("title", "新成就")), int(achievement.get("reward", 0))])
	return "\n".join(lines)
