class_name EndlessMode
extends RefCounted

## 无尽模式的本地智能出题器。
##
## 它不是把题目写死在列表里，而是根据题数、答题速度和连续表现
## 生成候选数字，再交给 PuzzleGenerator 穷举验证。验证不通过的题目
## 永远不会进入游戏。未来如果接入真实 AI，也应复用这套验证流程。

static func config_for_question(question_index: int, performance: Dictionary = {}) -> Dictionary:
	var base_stage := maxi(0, int(question_index / 3))
	var speed_ratio := float(performance.get("speed_ratio", 1.0))
	var fast_streak := int(performance.get("fast_streak", 0))
	var ai_stage_bonus := 1 if fast_streak >= 2 and speed_ratio <= 0.62 else 0
	var ai_stage := base_stage + ai_stage_bonus
	var time_limit := maxf(18.0, 45.0 - float(base_stage) * 1.0)
	if ai_stage_bonus > 0:
		time_limit = maxf(18.0, time_limit - 2.0)
	var advanced_digits := ai_stage >= 4
	return {
		"stage": base_stage,
		"ai_stage": ai_stage,
		"stage_name": _stage_name(ai_stage),
		"question_index": question_index,
		"question_count": 1,
		"time_limit": time_limit,
		"min_digit": 1,
		"max_digit": 13 if advanced_digits else 9,
		"min_solutions": 2 if ai_stage < 2 else 1,
		"max_solutions": 999999 if ai_stage < 2 else maxi(1, 12 - ai_stage),
		"allow_hint": ai_stage < 5,
		"hint_count": 1 if ai_stage < 3 else 0,
		"generator": "local_ai_director",
	}


static func score_multiplier(combo: int, fast_streak: int) -> float:
	var combo_steps := int(combo / 5)
	var multiplier := 1.0 + float(combo_steps) * 0.25
	if fast_streak >= 2:
		multiplier += 0.25
	return minf(3.0, multiplier)


static func milestone_for_questions(questions: int) -> Dictionary:
	if questions <= 0 or questions % 5 != 0:
		return {}
	var reward := 20 + int(questions / 5) * 5
	return {
		"questions": questions,
		"reward": reward,
		"title": "里程碑达成：连续答对 %d 题" % questions,
	}


static func generate_question(generator: RefCounted, question_index: int, run_seed: int, used_keys: Dictionary = {}, performance: Dictionary = {}) -> Dictionary:
	var config := config_for_question(question_index, performance)
	var rng := RandomNumberGenerator.new()
	rng.seed = _mix_seed(run_seed, question_index)
	var strict_attempts := 0
	var max_attempts := 180
	while strict_attempts < max_attempts:
		strict_attempts += 1
		var numbers := _generate_ai_candidate(rng, config)
		var key := _numbers_key(numbers)
		if used_keys.has(key):
			continue
		var puzzle := _validate_candidate(generator, numbers, config, question_index)
		if puzzle.is_empty():
			continue
		used_keys[key] = true
		return _decorate_puzzle(puzzle, config, key, strict_attempts)

	# 极端情况下优先保证无尽模式不断档：仍然必须有解，但允许放宽
	# 解法数量筛选；这不是跳过检测，而是降低本阶段的难度门槛。
	var relaxed_config := config.duplicate(true)
	relaxed_config["min_solutions"] = 1
	relaxed_config["max_solutions"] = 999999
	for fallback_attempt in range(240):
		var fallback_numbers := _generate_ai_candidate(rng, relaxed_config)
		var fallback_key := _numbers_key(fallback_numbers)
		if used_keys.has(fallback_key):
			continue
		var fallback_puzzle := _validate_candidate(generator, fallback_numbers, relaxed_config, question_index)
		if fallback_puzzle.is_empty():
			continue
		used_keys[fallback_key] = true
		return _decorate_puzzle(fallback_puzzle, config, fallback_key, max_attempts + fallback_attempt)
	return {}


static func _validate_candidate(generator: RefCounted, numbers: Array, config: Dictionary, question_index: int) -> Dictionary:
	# make_verified_record 内部调用 solve()，检查目标值、整数规则、解法数量，
	# 并把第一种已验证解法保存到题目记录中。
	return generator.make_verified_record(
		numbers,
		2000 + question_index,
		question_index,
		int(config["min_solutions"]),
		int(config["max_solutions"])
	)


static func _generate_ai_candidate(rng: RandomNumberGenerator, config: Dictionary) -> Array:
	var max_digit := int(config["max_digit"])
	var numbers: Array = []
	for _i in range(4):
		numbers.append(rng.randi_range(int(config["min_digit"]), max_digit))
	# 进入进阶数字阶段后，保证新数字规则确实能被玩家感知。
	if max_digit >= 13 and not _contains_advanced_digit(numbers):
		numbers[rng.randi_range(0, 3)] = rng.randi_range(10, 13)
	return numbers


static func _decorate_puzzle(puzzle: Dictionary, config: Dictionary, key: String, attempts: int) -> Dictionary:
	puzzle["puzzle_id"] = "ENDLESS-Q%04d" % int(config["question_index"] + 1)
	puzzle["endless_stage"] = int(config["stage"])
	puzzle["endless_ai_stage"] = int(config["ai_stage"])
	puzzle["endless_stage_name"] = str(config["stage_name"])
	puzzle["generation"] = {
		"source": "local_ai_director",
		"validated": true,
		"validator": "PuzzleGenerator.solve",
		"candidate_attempts": attempts,
		"number_key": key,
	}
	return puzzle


static func _contains_advanced_digit(numbers: Array) -> bool:
	for number in numbers:
		if int(number) >= 10:
			return true
	return false


static func _numbers_key(numbers: Array) -> String:
	var sorted := numbers.duplicate()
	sorted.sort()
	return ",".join(sorted.map(func(item): return str(item)))


static func _mix_seed(run_seed: int, question_index: int) -> int:
	var mixed := int(run_seed) + question_index * 7919 + question_index * question_index * 17
	return absi(mixed) + 1


static func _stage_name(stage: int) -> String:
	match stage:
		0:
			return "热身"
		1:
			return "加速"
		2:
			return "进阶"
		3:
			return "困难"
		4:
			return "高压"
		_:
			return "极限 %d" % (stage - 4)
