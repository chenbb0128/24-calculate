class_name SaveService
extends RefCounted

const SAVE_PATH := "user://twenty_four_progress.json"


static func _default_progress() -> Dictionary:
	return {
		"version": 9,
		"unlocked_level": 0,
		"last_level": 0,
		"tutorial_seen": false,
		"levels": {},
		"level_rewards": {},
		"milestones": {
			"first_clear": false,
			"three_star": false,
			"chapters": {},
		},
		"login": {
			"last_date": "",
			"streak": 0,
			"last_reward": 0,
		},
		"player_stats": {
			"total_solved": 0,
			"total_score": 0,
			"fastest_ms": 0,
			"best_combo": 0,
			"best_level": 0,
			"best_chapter": 0,
			"operator_counts": {},
			"mode_questions": {},
			"last_solve": {},
		},
		"endless": {
			"best_score": 0,
			"best_questions": 0,
			"best_combo": 0,
			"best_stage": 0,
			"last_score": 0,
			"daily_reward_date": "",
			"daily_reward_coins": 0,
		},
		"coins": 0,
		"owned_skins": ["classic"],
		"equipped_skin": "classic",
		"tasks": {
			"date": "",
			"values": {},
			"claimed": {},
		},
		"weekly_tasks": {
			"week": "",
			"values": {},
			"claimed": {},
		},
		"audio": {
			"music_enabled": true,
			"sfx_enabled": true,
			"music_track": 0,
			"music_volume_db": -25.0,
			"sfx_volume_db": -9.0,
			"chapter_music_auto": true,
		},
		"daily": {
			"last_date": "",
			"streak": 0,
			"best_score": 0,
			"completed": {},
			"reward_claimed": {},
			"last_reward": {},
		},
		"leaderboards": {},
		"achievements": {
			"unlocked": {},
			"claimed": {},
		},
		"friend_matches": {
			"date": "",
			"played": 0,
			"wins": 0,
			"best_score": 0,
		},
		"ads": {
			"date": "",
			"rewarded_used": 0,
		},
	}


static func load_progress() -> Dictionary:
	if not FileAccess.file_exists(SAVE_PATH):
		return _default_progress()
	var file := FileAccess.open(SAVE_PATH, FileAccess.READ)
	var parsed = JSON.parse_string(file.get_as_text())
	if parsed is Dictionary:
		parsed["version"] = maxi(int(parsed.get("version", 0)), 9)
		if not parsed.has("unlocked_level"):
			parsed["unlocked_level"] = 0
		if not parsed.has("last_level"):
			parsed["last_level"] = 0
		if not parsed.has("levels"):
			parsed["levels"] = {}
		if not parsed.has("tutorial_seen"):
			parsed["tutorial_seen"] = false
		if not parsed.has("level_rewards"):
			parsed["level_rewards"] = {}
		if not parsed.has("milestones") or not (parsed["milestones"] is Dictionary):
			parsed["milestones"] = _default_progress()["milestones"]
		else:
			var milestones: Dictionary = parsed["milestones"]
			for key in _default_progress()["milestones"].keys():
				if not milestones.has(key):
					milestones[key] = _default_progress()["milestones"][key]
			parsed["milestones"] = milestones
		if not parsed.has("login") or not (parsed["login"] is Dictionary):
			parsed["login"] = _default_progress()["login"]
		else:
			var login: Dictionary = parsed["login"]
			for key in _default_progress()["login"].keys():
				if not login.has(key):
					login[key] = _default_progress()["login"][key]
			parsed["login"] = login
		if not parsed.has("player_stats") or not (parsed["player_stats"] is Dictionary):
			parsed["player_stats"] = _default_progress()["player_stats"]
		else:
			var player_stats: Dictionary = parsed["player_stats"]
			for key in _default_progress()["player_stats"].keys():
				if not player_stats.has(key):
					player_stats[key] = _default_progress()["player_stats"][key]
			parsed["player_stats"] = player_stats
		if not parsed.has("endless"):
			parsed["endless"] = _default_progress()["endless"]
		else:
			var endless: Dictionary = parsed["endless"]
			for key in _default_progress()["endless"].keys():
				if not endless.has(key):
					endless[key] = _default_progress()["endless"][key]
			parsed["endless"] = endless
		if not parsed.has("coins"):
			parsed["coins"] = 0
		if not parsed.has("owned_skins"):
			parsed["owned_skins"] = ["classic"]
		if not parsed.has("equipped_skin"):
			parsed["equipped_skin"] = "classic"
		if not parsed.has("tasks") or not (parsed["tasks"] is Dictionary):
			parsed["tasks"] = _default_progress()["tasks"]
		else:
			var tasks: Dictionary = parsed["tasks"]
			if not tasks.has("date"):
				tasks["date"] = ""
			if not tasks.has("values") or not (tasks["values"] is Dictionary):
				tasks["values"] = {}
			if not tasks.has("claimed") or not (tasks["claimed"] is Dictionary):
				tasks["claimed"] = {}
			parsed["tasks"] = tasks
		if not parsed.has("weekly_tasks") or not (parsed["weekly_tasks"] is Dictionary):
			parsed["weekly_tasks"] = _default_progress()["weekly_tasks"]
		else:
			var weekly_tasks: Dictionary = parsed["weekly_tasks"]
			for key in _default_progress()["weekly_tasks"].keys():
				if not weekly_tasks.has(key):
					weekly_tasks[key] = _default_progress()["weekly_tasks"][key]
			parsed["weekly_tasks"] = weekly_tasks
		if not parsed.has("audio") or not (parsed["audio"] is Dictionary):
			parsed["audio"] = _default_progress()["audio"]
		else:
			var audio: Dictionary = parsed["audio"]
			for key in _default_progress()["audio"].keys():
				if not audio.has(key):
					audio[key] = _default_progress()["audio"][key]
			parsed["audio"] = audio
		if not parsed.has("daily"):
			parsed["daily"] = _default_progress()["daily"]
		else:
			var daily: Dictionary = parsed["daily"]
			if not daily.has("reward_claimed"):
				daily["reward_claimed"] = {}
			if not daily.has("last_reward"):
				daily["last_reward"] = {}
			parsed["daily"] = daily
		if not parsed.has("leaderboards") or not (parsed["leaderboards"] is Dictionary):
			parsed["leaderboards"] = {}
		if not parsed.has("achievements") or not (parsed["achievements"] is Dictionary):
			parsed["achievements"] = _default_progress()["achievements"]
		else:
			var achievements: Dictionary = parsed["achievements"]
			if not achievements.has("unlocked") or not (achievements["unlocked"] is Dictionary):
				achievements["unlocked"] = {}
			if not achievements.has("claimed") or not (achievements["claimed"] is Dictionary):
				achievements["claimed"] = {}
			parsed["achievements"] = achievements
		if not parsed.has("friend_matches") or not (parsed["friend_matches"] is Dictionary):
			parsed["friend_matches"] = _default_progress()["friend_matches"]
		else:
			var friend_matches: Dictionary = parsed["friend_matches"]
			for key in _default_progress()["friend_matches"].keys():
				if not friend_matches.has(key):
					friend_matches[key] = _default_progress()["friend_matches"][key]
			parsed["friend_matches"] = friend_matches
		if not parsed.has("ads") or not (parsed["ads"] is Dictionary):
			parsed["ads"] = _default_progress()["ads"]
		else:
			var ads: Dictionary = parsed["ads"]
			for key in _default_progress()["ads"].keys():
				if not ads.has(key):
					ads[key] = _default_progress()["ads"][key]
			parsed["ads"] = ads
		return parsed
	return _default_progress()


static func _migrate_for_test(parsed: Dictionary) -> Dictionary:
	# 只给自动测试使用的纯内存迁移入口，不读写真实用户存档。
	var copy: Dictionary = parsed.duplicate(true)
	if not copy.has("audio") or not (copy["audio"] is Dictionary):
		copy["audio"] = _default_progress()["audio"]
	else:
		var audio: Dictionary = copy["audio"]
		for key in _default_progress()["audio"].keys():
			if not audio.has(key):
				audio[key] = _default_progress()["audio"][key]
		copy["audio"] = audio
	if not copy.has("tasks") or not (copy["tasks"] is Dictionary):
		copy["tasks"] = _default_progress()["tasks"]
	if not copy.has("weekly_tasks") or not (copy["weekly_tasks"] is Dictionary):
		copy["weekly_tasks"] = _default_progress()["weekly_tasks"]
	if not copy.has("achievements") or not (copy["achievements"] is Dictionary):
		copy["achievements"] = _default_progress()["achievements"]
	if not copy.has("milestones") or not (copy["milestones"] is Dictionary):
		copy["milestones"] = _default_progress()["milestones"]
	if not copy.has("login") or not (copy["login"] is Dictionary):
		copy["login"] = _default_progress()["login"]
	if not copy.has("player_stats") or not (copy["player_stats"] is Dictionary):
		copy["player_stats"] = _default_progress()["player_stats"]
	if not copy.has("friend_matches") or not (copy["friend_matches"] is Dictionary):
		copy["friend_matches"] = _default_progress()["friend_matches"]
	if not copy.has("ads") or not (copy["ads"] is Dictionary):
		copy["ads"] = _default_progress()["ads"]
	else:
		var ads: Dictionary = copy["ads"]
		for key in _default_progress()["ads"].keys():
			if not ads.has(key):
				ads[key] = _default_progress()["ads"][key]
		copy["ads"] = ads
	return copy


static func save_level(progress: Dictionary, level_index: int, stars: int, score: int) -> void:
	var levels: Dictionary = progress.get("levels", {})
	var key := str(level_index)
	var old: Dictionary = levels.get(key, {})
	levels[key] = {
		"stars": maxi(int(old.get("stars", 0)), stars),
		"best_score": maxi(int(old.get("best_score", 0)), score),
	}
	progress["levels"] = levels
	progress["unlocked_level"] = maxi(int(progress.get("unlocked_level", 0)), level_index + 1)
	_flush(progress)


static func is_unlocked(progress: Dictionary, level_index: int) -> bool:
	return level_index <= int(progress.get("unlocked_level", 0))


static func save_last_level(progress: Dictionary, level_index: int) -> void:
	progress["last_level"] = level_index
	_flush(progress)


static func mark_tutorial_seen(progress: Dictionary) -> void:
	progress["tutorial_seen"] = true
	_flush(progress)


static func save_progress(progress: Dictionary) -> void:
	_flush(progress)


static func save_endless_result(progress: Dictionary, score: int, questions: int, combo: int, stage: int = 0) -> void:
	var endless: Dictionary = progress.get("endless", _default_progress()["endless"])
	endless["best_score"] = maxi(int(endless.get("best_score", 0)), score)
	endless["best_questions"] = maxi(int(endless.get("best_questions", 0)), questions)
	endless["best_combo"] = maxi(int(endless.get("best_combo", 0)), combo)
	endless["best_stage"] = maxi(int(endless.get("best_stage", 0)), stage)
	endless["last_score"] = score
	progress["endless"] = endless
	_flush(progress)


static func today_key() -> String:
	var date := Time.get_date_dict_from_system()
	return "%04d-%02d-%02d" % [int(date["year"]), int(date["month"]), int(date["day"])]


static func today_seed() -> int:
	var date := Time.get_date_dict_from_system()
	return int(date["year"]) * 10000 + int(date["month"]) * 100 + int(date["day"])


static func is_daily_completed(progress: Dictionary, date_key: String) -> bool:
	var daily: Dictionary = progress.get("daily", {})
	var completed: Dictionary = daily.get("completed", {})
	return bool(completed.get(date_key, false))


static func save_daily_result(progress: Dictionary, date_key: String, score: int) -> void:
	var daily: Dictionary = progress.get("daily", {})
	var completed: Dictionary = daily.get("completed", {})
	var last_date := str(daily.get("last_date", ""))
	var streak := int(daily.get("streak", 0))
	if not bool(completed.get(date_key, false)):
		if _is_previous_day(last_date, date_key):
			streak += 1
		else:
			streak = 1
	completed[date_key] = true
	daily["completed"] = completed
	daily["last_date"] = date_key
	daily["streak"] = streak
	daily["best_score"] = maxi(int(daily.get("best_score", 0)), score)
	progress["daily"] = daily
	_flush(progress)


static func _is_previous_day(previous_key: String, current_key: String) -> bool:
	if previous_key.is_empty():
		return false
	var previous := Time.get_unix_time_from_datetime_string(previous_key + "T00:00:00")
	var current := Time.get_unix_time_from_datetime_string(current_key + "T00:00:00")
	return int(current - previous) == 86400


static func _flush(progress: Dictionary) -> void:
	var file := FileAccess.open(SAVE_PATH, FileAccess.WRITE)
	file.store_string(JSON.stringify(progress))
