extends SceneTree

const PuzzleGeneratorScript = preload("res://core/puzzle_generator.gd")
const DailyChallengeScript = preload("res://core/daily_challenge.gd")
const RewardServiceScript = preload("res://services/reward_service.gd")
const TaskServiceScript = preload("res://services/task_service.gd")
const PlayerStatsScript = preload("res://services/player_stats.gd")
const ShareServiceScript = preload("res://services/share_service.gd")


func _init() -> void:
	_verify_all_daily_rules()
	_verify_login_rewards()
	_verify_weekly_tasks()
	_verify_player_stats()
	_verify_share_payload()
	print("Advanced feature verification passed: daily rules, login rewards, weekly tasks, stats and share")
	quit()


func _verify_all_daily_rules() -> void:
	var generator := PuzzleGeneratorScript.new()
	for rule_index in range(DailyChallengeScript.RULE_COUNT):
		var date_seed := rule_index
		var daily: Dictionary = DailyChallengeScript.build(generator, "2026-08-%02d" % (13 + rule_index), date_seed)
		assert(not daily.is_empty(), "每日规则 %d 应能生成" % rule_index)
		assert(daily["puzzles"].size() == 3, "每日规则 %d 应生成 3 道题" % rule_index)
		for puzzle in daily["puzzles"]:
			var solution := str(puzzle["solution"])
			var forbidden := str(daily.get("forbidden_operator", ""))
			var required := str(daily.get("required_operator", ""))
			if forbidden == "×":
				assert(not solution.contains("×"), "禁用乘法规则生成了乘法解")
			if forbidden == "+":
				assert(not solution.contains("+"), "禁用加法规则生成了加法解")
			if forbidden == "÷":
				assert(not solution.contains("÷"), "禁用除法规则生成了除法解")
			if required == "−":
				assert(solution.contains("-"), "必须减法规则没有减法解")
			if required == "×":
				assert(solution.contains("×"), "必须乘法规则没有乘法解")
			if str(daily["rule_id"]) == "big_digits":
				var has_big_digit := false
				for number in puzzle["numbers"]:
					has_big_digit = has_big_digit or int(number) >= 10
				assert(has_big_digit, "进阶数字规则没有出现 10～13")


func _verify_login_rewards() -> void:
	var progress := {"coins": 0}
	var first: Dictionary = RewardServiceScript.claim_login_reward(progress, "2026-08-13")
	var duplicate: Dictionary = RewardServiceScript.claim_login_reward(progress, "2026-08-13")
	var second: Dictionary = RewardServiceScript.claim_login_reward(progress, "2026-08-14")
	var skipped: Dictionary = RewardServiceScript.claim_login_reward(progress, "2026-08-16")
	assert(int(first["coins"]) == 5, "登录首日奖励错误")
	assert(bool(duplicate["already_claimed"]) and int(duplicate["coins"]) == 0, "登录奖励同一天不能重复领取")
	assert(int(second["coins"]) == 8 and int(second["streak"]) == 2, "连续登录奖励没有递增")
	assert(int(skipped["streak"]) == 1, "断签后登录连续天数应重置")
	assert(int(progress["coins"]) == 18, "登录奖励金币累计错误")


func _verify_weekly_tasks() -> void:
	var progress := {"coins": 0}
	for _i in range(4):
		var pending: Dictionary = TaskServiceScript.record_weekly(progress, "weekly_campaign", 1, "2026-08-13")
		assert(int(pending["reward"]) == 0, "每周任务未完成前不应发奖励")
	var completed: Dictionary = TaskServiceScript.record_weekly(progress, "weekly_campaign", 1, "2026-08-13")
	var duplicate: Dictionary = TaskServiceScript.record_weekly(progress, "weekly_campaign", 1, "2026-08-13")
	assert(bool(completed["completed"]) and int(completed["reward"]) == 45, "每周任务完成奖励错误")
	assert(int(duplicate["reward"]) == 0, "每周任务奖励不能重复领取")
	assert(int(progress["coins"]) == 45, "每周任务金币累计错误")
	var next_week: Dictionary = TaskServiceScript.weekly_snapshot(progress, "2026-08-20")
	assert(int(next_week["weekly_campaign"]["value"]) == 0, "跨周后任务进度应刷新")
	assert(not bool(next_week["weekly_campaign"]["claimed"]), "跨周后任务领取状态应刷新")


func _verify_player_stats() -> void:
	var progress := {}
	PlayerStatsScript.record_solve(progress, "campaign", 3200, 100, 3, ["+", "×"], 20)
	PlayerStatsScript.record_solve(progress, "endless", 1800, 80, 5, ["×", "×"], -1)
	var summary: Dictionary = PlayerStatsScript.summary(progress)
	assert(int(summary["total_solved"]) == 2, "挑战记录答题数错误")
	assert(int(summary["total_score"]) == 180, "挑战记录累计分数错误")
	assert(int(summary["fastest_ms"]) == 1800, "挑战记录最快时间错误")
	assert(int(summary["best_combo"]) == 5, "挑战记录最高连击错误")
	assert(int(summary["best_level"]) == 21, "挑战记录最高关卡错误")
	assert(str(summary["favorite_operator"]) == "×", "最常使用运算符错误")
	assert(int(summary["mode_questions"]["campaign"]) == 1 and int(summary["mode_questions"]["endless"]) == 1, "模式答题统计错误")


func _verify_share_payload() -> void:
	var room := {"room_code": "A7K2", "room_seed": 20260813}
	var room_payload: Dictionary = ShareServiceScript.create_friend_room_payload(room)
	assert(str(room_payload["query"]).contains("room=A7K2"), "好友房间分享缺少房间号")
	assert(str(room_payload["query"]).contains("seed=20260813"), "好友房间分享缺少题目种子")
	var result := {"outcome": "win", "player_solved": 7, "player_score": 560, "player_elapsed": 42.5}
	var result_payload: Dictionary = ShareServiceScript.create_match_result_payload(result, room)
	var card := ShareServiceScript.build_result_card_text(result)
	assert(str(result_payload["path"]).contains("room=A7K2"), "战绩分享缺少房间号")
	assert(card.contains("击败好友") and card.contains("560 分"), "战绩卡文字错误")
