class_name LeaderboardService
extends RefCounted

## 排行榜服务层。
## 当前 Godot 原型使用稳定的本地模拟数据，正式接入微信小游戏时，
## 只需要把 get_entries/submit_score 替换为微信开放数据域或服务端请求。

const BOARD_FRIENDS := "friends"
const BOARD_GLOBAL := "global"
const MODE_CAMPAIGN := "campaign"
const MODE_DAILY := "daily"
const MODE_ENDLESS := "endless"


static func mode_ids() -> Array:
	return [MODE_CAMPAIGN, MODE_DAILY, MODE_ENDLESS]


static func mode_name(mode_id: String) -> String:
	match mode_id:
		MODE_CAMPAIGN:
			return "闯关模式"
		MODE_DAILY:
			return "每日挑战"
		MODE_ENDLESS:
			return "无尽模式"
	return "排行榜"


static func board_name(board_id: String) -> String:
	return "微信好友榜" if board_id == BOARD_FRIENDS else "游戏总榜"


static func ensure_progress(progress: Dictionary) -> void:
	if not progress.has("leaderboards") or not (progress["leaderboards"] is Dictionary):
		progress["leaderboards"] = {}
	var leaderboards: Dictionary = progress["leaderboards"]
	for mode_id in mode_ids():
		if not leaderboards.has(mode_id) or not (leaderboards[mode_id] is Dictionary):
			leaderboards[mode_id] = _empty_record()
		else:
			var record: Dictionary = leaderboards[mode_id]
			for key in _empty_record().keys():
				if not record.has(key):
					record[key] = _empty_record()[key]
			leaderboards[mode_id] = record
	progress["leaderboards"] = leaderboards


static func submit_score(progress: Dictionary, mode_id: String, score: int, detail: Dictionary = {}) -> Dictionary:
	ensure_progress(progress)
	if not mode_ids().has(mode_id):
		return {"accepted": false, "new_record": false, "best_score": 0}
	var leaderboards: Dictionary = progress["leaderboards"]
	var record: Dictionary = leaderboards[mode_id]
	var old_score := int(record.get("best_score", 0))
	var accepted := score > 0
	var new_record := accepted and score > old_score
	if accepted:
		record["last_score"] = score
		record["last_detail"] = detail.duplicate(true)
		record["last_submitted_at"] = Time.get_unix_time_from_system()
		if new_record:
			record["best_score"] = score
			record["best_detail"] = detail.duplicate(true)
	leaderboards[mode_id] = record
	progress["leaderboards"] = leaderboards
	return {
		"accepted": accepted,
		"new_record": new_record,
		"best_score": int(record.get("best_score", 0)),
		"mode_id": mode_id,
	}


static func campaign_score(progress: Dictionary) -> int:
	var total := 0
	var levels: Dictionary = progress.get("levels", {})
	for record_value in levels.values():
		if record_value is Dictionary:
			total += int(record_value.get("best_score", 0))
	return total


static func player_score(progress: Dictionary, mode_id: String) -> int:
	ensure_progress(progress)
	if mode_id == MODE_CAMPAIGN:
		return campaign_score(progress)
	var leaderboards: Dictionary = progress.get("leaderboards", {})
	var record: Dictionary = leaderboards.get(mode_id, {})
	var leaderboard_score := int(record.get("best_score", 0))
	if mode_id == MODE_ENDLESS:
		return maxi(leaderboard_score, int(progress.get("endless", {}).get("best_score", 0)))
	if mode_id == MODE_DAILY:
		return maxi(leaderboard_score, int(progress.get("daily", {}).get("best_score", 0)))
	return leaderboard_score


static func get_entries(progress: Dictionary, board_id: String, mode_id: String) -> Array:
	ensure_progress(progress)
	var entries: Array = []
	var names: Array = _names(board_id)
	var scores: Array = _scores(board_id, mode_id)
	for index in range(names.size()):
		entries.append({
			"name": str(names[index]),
			"score": int(scores[index]),
			"is_player": false,
			"subtitle": "好友" if board_id == BOARD_FRIENDS else "挑战者",
		})
	entries.append({
		"name": "我",
		"score": player_score(progress, mode_id),
		"is_player": true,
		"subtitle": "当前玩家",
	})
	entries.sort_custom(func(a: Dictionary, b: Dictionary) -> bool:
		if int(a["score"]) == int(b["score"]):
			return bool(a["is_player"]) and not bool(b["is_player"])
		return int(a["score"]) > int(b["score"])
	)
	for index in range(entries.size()):
		entries[index]["rank"] = index + 1
	return entries


static func _empty_record() -> Dictionary:
	return {
		"best_score": 0,
		"last_score": 0,
		"best_detail": {},
		"last_detail": {},
		"last_submitted_at": 0,
	}


static func _names(board_id: String) -> Array:
	if board_id == BOARD_FRIENDS:
		return ["好友·小数", "好友·24号", "好友·小满", "好友·阿算", "好友·小棋", "好友·答题王"]
	return ["24研究所", "算术小天才", "脑力冲刺", "每日满星", "极速运算", "四则玩家", "数字探险家", "挑战不可能"]


static func _scores(board_id: String, mode_id: String) -> Array:
	if board_id == BOARD_FRIENDS:
		match mode_id:
			MODE_CAMPAIGN:
				return [6800, 6200, 5400, 4800, 3900, 2800]
			MODE_DAILY:
				return [520, 460, 410, 350, 280, 220]
			MODE_ENDLESS:
				return [8600, 7200, 6100, 4900, 3600, 2500]
	else:
		match mode_id:
			MODE_CAMPAIGN:
				return [24500, 19800, 16600, 14200, 11800, 9600, 7600, 5200]
			MODE_DAILY:
				return [980, 860, 760, 680, 580, 490, 390, 300]
			MODE_ENDLESS:
				return [52000, 41600, 33800, 27600, 21800, 16600, 12000, 8000]
	return []
