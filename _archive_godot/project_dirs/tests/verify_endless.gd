extends SceneTree

const PuzzleGeneratorScript = preload("res://core/puzzle_generator.gd")
const EndlessModeScript = preload("res://core/endless_mode.gd")
const RewardServiceScript = preload("res://services/reward_service.gd")

func _init() -> void:
	var generator := PuzzleGeneratorScript.new()
	var used_keys := {}
	var generated_keys := {}
	var previous_stage := -1
	for question_index in range(40):
		var puzzle: Dictionary = EndlessModeScript.generate_question(generator, question_index, 20260813, used_keys)
		assert(not puzzle.is_empty(), "无尽模式第 %d 题生成失败" % (question_index + 1))
		assert(puzzle["numbers"].size() == 4, "无尽模式题目必须有 4 个数字")
		assert(int(puzzle["target"]) == 24, "无尽模式目标必须为 24")
		assert(bool(puzzle["generation"]["validated"]), "无尽题目必须标记为已验证")
		assert(str(puzzle["generation"]["validator"]) == "PuzzleGenerator.solve", "无尽题目必须经过求解器")
		var key: String = str(puzzle["generation"]["number_key"])
		assert(not generated_keys.has(key), "同一局不能重复题目")
		generated_keys[key] = true
		var expected_stage := int(question_index / 3)
		assert(int(puzzle["endless_stage"]) == expected_stage, "无尽模式难度阶段错误")
		previous_stage = int(puzzle["endless_stage"])
	var fast_config: Dictionary = EndlessModeScript.config_for_question(6, {"speed_ratio": 0.5, "fast_streak": 2})
	var normal_config: Dictionary = EndlessModeScript.config_for_question(6)
	assert(int(fast_config["ai_stage"]) > int(normal_config["ai_stage"]), "快速答题应触发提前升档")
	assert(float(fast_config["time_limit"]) < float(normal_config["time_limit"]), "提前升档应缩短时间")
	assert(is_equal_approx(EndlessModeScript.score_multiplier(1, 0), 1.0), "基础倍率错误")
	assert(is_equal_approx(EndlessModeScript.score_multiplier(5, 0), 1.25), "5 连击倍率错误")
	assert(is_equal_approx(EndlessModeScript.score_multiplier(10, 2), 1.75), "快速高连击倍率错误")
	assert(int(EndlessModeScript.milestone_for_questions(5)["reward"]) == 25, "5 题里程碑奖励错误")
	assert(EndlessModeScript.milestone_for_questions(6).is_empty(), "非里程碑题数不应触发奖励")
	assert(is_equal_approx(float(EndlessModeScript.config_for_question(0)["time_limit"]), 45.0), "无尽模式首题应有 45 秒")
	assert(float(EndlessModeScript.config_for_question(120)["time_limit"]) >= 18.0, "无尽模式最低时间不能少于 18 秒")
	var late_puzzle: Dictionary = EndlessModeScript.generate_question(generator, 120, 20260813, {})
	assert(not late_puzzle.is_empty(), "高题数无尽题目生成失败")
	var progress := {"coins": 0, "endless": {"best_score": 0, "best_questions": 0, "best_combo": 0}}
	var reward: Dictionary = RewardServiceScript.claim_endless_reward(progress, 1200, 10, 6, "2026-08-13")
	assert(int(reward["coins"]) == 68, "无尽奖励计算错误")
	assert(int(progress["coins"]) == 68, "无尽金币保存错误")
	var second_reward: Dictionary = RewardServiceScript.claim_endless_reward(progress, 1200, 10, 6, "2026-08-13")
	assert(int(second_reward["coins"]) == 52, "每日上限前的奖励计算错误")
	assert(int(progress["coins"]) == 120, "每日无尽奖励累计错误")
	var capped_reward: Dictionary = RewardServiceScript.claim_endless_reward(progress, 1200, 10, 6, "2026-08-13")
	assert(int(capped_reward["coins"]) == 0, "达到每日上限时不应继续发奖励")
	assert(int(progress["coins"]) == 120, "每日无尽奖励上限错误")
	var after_cap_reward: Dictionary = RewardServiceScript.claim_endless_reward(progress, 1200, 10, 6, "2026-08-13")
	assert(int(after_cap_reward["coins"]) == 0, "达到每日上限后不应继续发无尽金币")
	var next_day_reward: Dictionary = RewardServiceScript.claim_endless_reward(progress, 1200, 1, 1, "2026-08-14")
	assert(int(next_day_reward["coins"]) > 0, "第二天应恢复无尽奖励额度")
	print("Endless verification passed: 40 unique validated questions, adaptive difficulty and reward")
	quit()
