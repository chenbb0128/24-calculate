class_name RewardService
extends RefCounted

## 奖励计算与领取记录。
## 领取记录按日期/关卡保存，避免重复进入页面时重复发金币。

static func claim_login_reward(progress: Dictionary, date_key: String) -> Dictionary:
	var login: Dictionary = progress.get("login", {"last_date": "", "streak": 0, "last_reward": 0})
	var last_date := str(login.get("last_date", ""))
	if last_date == date_key:
		return {"coins": 0, "streak": int(login.get("streak", 0)), "already_claimed": true, "message": "今日登录奖励已领取"}
	var streak := 1
	if _is_previous_day(last_date, date_key):
		streak = int(login.get("streak", 0)) + 1
	var day_in_cycle := posmod(streak - 1, 7) + 1
	var rewards: Array[int] = [5, 8, 10, 12, 15, 18, 30]
	var coins: int = rewards[day_in_cycle - 1]
	login["last_date"] = date_key
	login["streak"] = streak
	login["last_reward"] = coins
	progress["login"] = login
	_add_coins(progress, coins)
	return {"coins": coins, "streak": streak, "already_claimed": false, "message": "登录奖励 +%d 金币" % coins, "day_in_cycle": day_in_cycle}

static func claim_ad_coin_bonus(progress: Dictionary, requested_coins: int) -> int:
	# 广告奖励只做小额补充，不改变正常关卡、每日和无尽奖励。
	var coins := clampi(requested_coins, 0, 60)
	_add_coins(progress, coins)
	return coins

static func claim_daily_reward(progress: Dictionary, date_key: String, score: int, question_count: int, hint_used: bool) -> Dictionary:
	var daily: Dictionary = progress.get("daily", {})
	var claimed: Dictionary = daily.get("reward_claimed", {})
	if bool(claimed.get(date_key, false)):
		return {"coins": 0, "already_claimed": true, "message": "今日奖励已经领取"}
	# 每日挑战是稳定收入，但不应该成为最快的刷币方式。
	var coins := 30
	var full_clear := question_count >= 3
	if full_clear:
		coins += 10
	if not hint_used:
		coins += 10
	var streak := int(daily.get("streak", 0))
	if streak > 0 and streak % 3 == 0:
		coins += 20
	if streak > 0 and streak % 7 == 0:
		coins += 40
	claimed[date_key] = true
	daily["reward_claimed"] = claimed
	daily["last_reward"] = {
		"date": date_key,
		"coins": coins,
		"score": score,
		"full_clear": full_clear,
	}
	progress["daily"] = daily
	_add_coins(progress, coins)
	return {
		"coins": coins,
		"already_claimed": false,
		"message": "每日挑战奖励 +%d 金币" % coins,
		"streak": streak,
	}


static func claim_level_reward(progress: Dictionary, level_index: int, stars: int) -> int:
	var rewards: Dictionary = progress.get("level_rewards", {})
	var key := str(level_index)
	if bool(rewards.get(key, false)):
		return 0
	var coins := 8 + stars * 3
	rewards[key] = true
	progress["level_rewards"] = rewards
	_add_coins(progress, coins)
	return coins


static func claim_campaign_bonus(progress: Dictionary, level_index: int, stars: int, chapter_index: int, chapter_complete: bool, new_level_clear: bool) -> Dictionary:
	var milestones: Dictionary = progress.get("milestones", {
		"first_clear": false,
		"three_star": false,
		"chapters": {},
	})
	var chapters: Dictionary = milestones.get("chapters", {})
	var coins := 0
	var labels: Array[String] = []
	var first_clear := false
	var three_star := false
	var chapter_clear := false
	if new_level_clear and not bool(milestones.get("first_clear", false)):
		milestones["first_clear"] = true
		coins += 12
		labels.append("首次通关 +12")
		first_clear = true
	if stars >= 3 and not bool(milestones.get("three_star", false)):
		milestones["three_star"] = true
		coins += 8
		labels.append("首次三星 +8")
		three_star = true
	var chapter_key := str(chapter_index)
	if chapter_complete and new_level_clear and not bool(chapters.get(chapter_key, false)):
		chapters[chapter_key] = true
		coins += 20
		labels.append("章节完成 +20")
		chapter_clear = true
	milestones["chapters"] = chapters
	progress["milestones"] = milestones
	_add_coins(progress, coins)
	return {
		"coins": coins,
		"labels": labels,
		"first_clear": first_clear,
		"three_star": three_star,
		"chapter_clear": chapter_clear,
		"level": level_index + 1,
		"chapter": chapter_index + 1,
	}


static func claim_endless_reward(progress: Dictionary, score: int, questions: int, max_combo: int, date_key: String = "") -> Dictionary:
	const DAILY_ENDLESS_CAP := 120
	var endless: Dictionary = progress.get("endless", {})
	var old_score := int(endless.get("best_score", 0))
	var old_questions := int(endless.get("best_questions", 0))
	var old_combo := int(endless.get("best_combo", 0))
	var stage := int(maxi(0, questions - 1) / 3)
	var base_coins := questions * 2
	var stage_coins := stage * 4
	var milestone_coins := int(questions / 5) * 8
	var combo_coins := int(max_combo / 5) * 5
	var requested_coins := mini(90, base_coins + stage_coins + milestone_coins + combo_coins)
	if score > old_score:
		requested_coins += 15
	var reward_date := date_key
	if reward_date.is_empty():
		var date := Time.get_date_dict_from_system()
		reward_date = "%04d-%02d-%02d" % [int(date["year"]), int(date["month"]), int(date["day"])]
	var reward_date_saved := str(endless.get("daily_reward_date", ""))
	var earned_today := int(endless.get("daily_reward_coins", 0)) if reward_date_saved == reward_date else 0
	var coins := mini(requested_coins, maxi(0, DAILY_ENDLESS_CAP - earned_today))
	endless["daily_reward_date"] = reward_date
	endless["daily_reward_coins"] = earned_today + coins
	progress["endless"] = endless
	_add_coins(progress, coins)
	return {
		"coins": coins,
		"requested_coins": requested_coins,
		"daily_cap": DAILY_ENDLESS_CAP,
		"daily_earned": earned_today + coins,
		"daily_cap_reached": earned_today + coins >= DAILY_ENDLESS_CAP,
		"daily_cap_remaining": maxi(0, DAILY_ENDLESS_CAP - earned_today - coins),
		"base_coins": base_coins,
		"stage_coins": stage_coins,
		"milestone_coins": milestone_coins,
		"combo_coins": combo_coins,
		"new_score_record": score > old_score,
		"new_questions_record": questions > old_questions,
		"new_combo_record": max_combo > old_combo,
	}


static func claim_friend_match_reward(progress: Dictionary, result: Dictionary, date_key: String = "") -> Dictionary:
	var friend_matches: Dictionary = progress.get("friend_matches", {})
	var reward_date := date_key
	if reward_date.is_empty():
		reward_date = Time.get_date_string_from_system()
	if str(friend_matches.get("date", "")) != reward_date:
		friend_matches["date"] = reward_date
		friend_matches["played"] = 0
		friend_matches["wins"] = 0
	var played := int(friend_matches.get("played", 0))
	if played >= 3:
		return {"coins": 0, "played": played, "daily_limit": 3, "limit_reached": true, "message": "今日好友对战奖励已领完"}
	var outcome := str(result.get("outcome", "draw"))
	var coins := 8
	if outcome == "win":
		coins = 20
	elif outcome == "draw":
		coins = 12
	friend_matches["played"] = played + 1
	if outcome == "win":
		friend_matches["wins"] = int(friend_matches.get("wins", 0)) + 1
	friend_matches["best_score"] = maxi(int(friend_matches.get("best_score", 0)), int(result.get("player_score", 0)))
	progress["friend_matches"] = friend_matches
	_add_coins(progress, coins)
	return {"coins": coins, "played": int(friend_matches["played"]), "daily_limit": 3, "limit_reached": false, "message": "好友对战胜利 +%d 金币" % coins if outcome == "win" else "参与奖励 +%d 金币" % coins}


static func _add_coins(progress: Dictionary, amount: int) -> void:
	progress["coins"] = maxi(0, int(progress.get("coins", 0)) + amount)


static func _is_previous_day(previous_key: String, current_key: String) -> bool:
	if previous_key.is_empty():
		return false
	var previous := Time.get_unix_time_from_datetime_string(previous_key + "T00:00:00")
	var current := Time.get_unix_time_from_datetime_string(current_key + "T00:00:00")
	return int(current - previous) == 86400
