extends SceneTree

const PuzzleGeneratorScript = preload("res://core/puzzle_generator.gd")
const LevelCatalogScript = preload("res://core/level_catalog.gd")
const DailyChallengeScript = preload("res://core/daily_challenge.gd")
const EndlessModeScript = preload("res://core/endless_mode.gd")
const RewardServiceScript = preload("res://services/reward_service.gd")


func _init() -> void:
	var generator := PuzzleGeneratorScript.new()
	var levels: Array = LevelCatalogScript.all()
	assert(levels.size() == 200, "应有 200 个关卡")
	assert(ceili(float(levels.size()) / 20.0) == 10, "应有 10 页关卡")
	var sample_progress := {"unlocked_level": 20, "last_level": 20, "levels": {}}
	assert(int(sample_progress["unlocked_level"] / 20) == 1, "第 21 关应位于第 2 页")
	assert(int(levels[0]["question_count"]) == 3, "普通关应为 3 题")
	assert(int(levels[4]["question_count"]) == 5, "每 5 关应为挑战关")
	assert(bool(levels[4]["is_challenge"]), "第 5 关应标记为挑战关")
	var total_puzzles := 0
	for index in range(levels.size()):
		# 每关独立生成，模拟玩家进入关卡时的懒加载，避免把所有关卡题目同时压入内存。
		print("checking level %d" % (index + 1))
		var puzzles: Array = generator.generate_level(levels[index], index, int(levels[index]["question_count"]))
		assert(puzzles.size() == int(levels[index]["question_count"]), "第 %d 关题目数量不足" % (index + 1))
		for puzzle in puzzles:
			total_puzzles += 1
			assert(puzzle["numbers"].size() == 4, "题目必须有 4 个数字")
			assert(int(puzzle["target"]) == 24, "目标必须是 24")
			assert(not str(puzzle["solution"]).is_empty(), "必须记录至少一种解法")
			assert(int(puzzle["solution_count"]) > 0, "解法数量必须大于 0")
	assert(generator.solve([1, 2, 3, 7]).size() > 0, "经典题目应有解")
	assert(generator.solve([1, 1, 1, 1]).is_empty(), "无解题目必须被识别")
	var daily_config := {"min_digit": 1, "max_digit": 13, "min_solutions": 1, "max_solutions": 999999}
	var daily_a: Array = generator.generate_level(daily_config, 0, 1, 20260813)
	var daily_b: Array = generator.generate_level(daily_config, 0, 1, 20260813)
	assert(daily_a[0]["numbers"] == daily_b[0]["numbers"], "每日题固定种子应生成相同题目")
	for seed in [20260813, 20260814, 20260815, 20260816]:
		var daily: Dictionary = DailyChallengeScript.build(generator, str(seed), seed)
		assert(daily.size() > 0, "每日挑战应能生成")
		assert(int(daily["question_count"]) == 3, "每日挑战应为 3 题")
		assert(daily["puzzles"].size() == 3, "每日挑战题目数量不足")
		if str(daily["rule_id"]) == "no_division":
			for puzzle in daily["puzzles"]:
				assert(not str(puzzle["solution"]).contains("÷"), "禁除法规则生成了除法解")
		if str(daily["rule_id"]) == "must_subtract":
			for puzzle in daily["puzzles"]:
				assert(str(puzzle["solution"]).contains("-"), "必须减法规则没有减法解")
		if str(daily["rule_id"]) == "big_digits":
			for puzzle in daily["puzzles"]:
				var has_big := false
				for number in puzzle["numbers"]:
					has_big = has_big or int(number) >= 10
				assert(has_big, "进阶数字规则没有出现 10～13")
	var reward_progress := {"coins": 0, "daily": {"streak": 3, "reward_claimed": {}}}
	var reward_a: Dictionary = RewardServiceScript.claim_daily_reward(reward_progress, "2026-08-13", 500, 3, false)
	assert(int(reward_a["coins"]) == 70, "每日奖励计算不正确")
	var reward_b: Dictionary = RewardServiceScript.claim_daily_reward(reward_progress, "2026-08-13", 500, 3, false)
	assert(bool(reward_b["already_claimed"]), "每日奖励不能重复领取")
	assert(int(reward_progress["coins"]) == 70, "金币存储不正确")
	for question_index in range(12):
		var endless_config: Dictionary = EndlessModeScript.config_for_question(question_index)
		var endless_puzzle: Dictionary = EndlessModeScript.generate_question(generator, question_index, 20260813)
		assert(not endless_puzzle.is_empty(), "无尽模式第 %d 题生成失败" % (question_index + 1))
		assert(endless_puzzle["numbers"].size() == 4, "无尽模式题目必须有 4 个数字")
		assert(int(endless_puzzle["target"]) == 24, "无尽模式目标必须是 24")
		assert(int(endless_config["stage"]) == int(question_index / 3), "无尽模式难度阶段错误")
	assert(float(EndlessModeScript.config_for_question(0)["time_limit"]) > float(EndlessModeScript.config_for_question(60)["time_limit"]), "无尽模式应逐步缩短时间")
	var endless_reward_progress := {"coins": 0, "endless": {"best_score": 0, "best_questions": 0, "best_combo": 0}}
	var endless_reward: Dictionary = RewardServiceScript.claim_endless_reward(endless_reward_progress, 1200, 10, 6)
	assert(int(endless_reward["coins"]) == 68, "无尽模式奖励计算不正确")
	assert(int(endless_reward_progress["coins"]) == 68, "无尽模式金币存储不正确")
	print("MVP verification passed: %d puzzles across %d levels" % [total_puzzles, levels.size()])
	quit()
