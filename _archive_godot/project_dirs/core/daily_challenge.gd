class_name DailyChallenge
extends RefCounted

## 每日挑战配置器。
## 候选数字来自题目生成器，最终解法一定由 PuzzleGenerator 穷举验证。

const DAILY_QUESTION_COUNT := 3
const RULE_COUNT := 8
const SIMPLE_CANDIDATES := [
	[1, 2, 3, 4], [1, 2, 3, 8], [1, 2, 4, 5], [1, 2, 5, 5],
	[1, 3, 5, 6], [2, 3, 4, 6], [2, 3, 6, 8], [3, 4, 6, 9],
	[4, 4, 6, 8], [1, 5, 5, 5], [2, 4, 7, 8], [3, 5, 7, 9],
]
const ADVANCED_CANDIDATES := [
	[4, 6, 10, 12], [3, 7, 11, 13], [2, 8, 10, 13], [5, 6, 11, 13],
	[1, 2, 6, 10], [1, 2, 7, 12], [1, 4, 10, 12], [2, 4, 10, 13],
	[2, 6, 10, 13], [2, 6, 11, 12], [3, 4, 6, 13],
]
const NO_MULTIPLY_CANDIDATES := [
	[5, 5, 5, 9], [6, 6, 6, 6],
]


static func build(generator: RefCounted, date_key: String, date_seed: int) -> Dictionary:
	var rule_index := posmod(date_seed, RULE_COUNT)
	var rule := _rule_for_index(rule_index)
	var puzzles: Array = []
	var candidates: Array = NO_MULTIPLY_CANDIDATES if rule["id"] == "no_multiply" else (ADVANCED_CANDIDATES if rule["id"] == "big_digits" else SIMPLE_CANDIDATES)
	for stage in range(DAILY_QUESTION_COUNT):
		var min_solutions := 1 if stage == 2 else 2
		var max_solutions := 6 if stage == 2 else 20
		var required := str(rule.get("required_operator", ""))
		var forbidden := str(rule.get("forbidden_operator", ""))
		if required == "−":
			required = "-"
		var puzzle := _pick_verified_puzzle(generator, candidates, date_seed, stage, min_solutions, max_solutions, required, forbidden)
		if puzzle.is_empty():
			return {}
		puzzle["daily_stage"] = stage
		puzzle["daily_stage_name"] = ["热身题", "进阶题", "高难题"][stage]
		puzzle["daily_rule_id"] = rule["id"]
		puzzles.append(puzzle)
	return {
		"date_key": date_key,
		"seed": date_seed,
		"title": "每日挑战 · 三题连战",
		"rule_id": rule["id"],
		"rule_title": rule["title"],
		"rule_text": rule["text"],
		"rule_index": rule_index,
		"time_bonus": bool(rule.get("time_bonus", false)),
		"required_operator": rule.get("required_operator", ""),
		"forbidden_operator": rule.get("forbidden_operator", ""),
		"max_digit": int(rule["max_digit"]),
		"question_count": DAILY_QUESTION_COUNT,
		"time_limit": 150.0,
		"hint_count": 2 if rule["id"] == "no_undo" else 1,
		"allow_hint": true,
		"puzzles": puzzles,
	}


static func rule_preview(date_seed: int) -> Dictionary:
	return _rule_for_index(posmod(date_seed, RULE_COUNT)).duplicate(true)


static func _pick_verified_puzzle(generator: RefCounted, candidates: Array, date_seed: int, stage: int, min_solutions: int, max_solutions: int, required: String, forbidden: String) -> Dictionary:
	var start := posmod(date_seed + stage * 7, candidates.size())
	var seen: Dictionary = {}
	for offset in range(candidates.size()):
		var candidate: Array = candidates[(start + offset) % candidates.size()]
		seen[_candidate_key(candidate)] = true
		var puzzle: Dictionary = generator.make_verified_record(candidate, 3000 + stage, stage, min_solutions, max_solutions, required, forbidden)
		if not puzzle.is_empty():
			return puzzle
	# 规则越丰富，固定候选池越容易出现“今天刚好不够三题”的情况。
	# 这里用固定种子生成候选数字，再交给同一个穷举验证器筛选，绝不直接相信随机结果。
	var max_digit := 9
	for candidate_value in candidates:
		for number in candidate_value:
			max_digit = maxi(max_digit, int(number))
	var rng := RandomNumberGenerator.new()
	rng.seed = absi(date_seed * 97 + stage * 7919 + required.length() * 31 + forbidden.length() * 53) + 1
	for attempt in range(1200):
		var generated: Array = []
		for _i in range(4):
			generated.append(rng.randi_range(1, max_digit))
		var key := _candidate_key(generated)
		if seen.has(key):
			continue
		seen[key] = true
		var generated_puzzle: Dictionary = generator.make_verified_record(generated, 3000 + stage, stage, min_solutions, max_solutions, required, forbidden)
		if not generated_puzzle.is_empty():
			return generated_puzzle
	return {}


static func _candidate_key(numbers: Array) -> String:
	var sorted := numbers.duplicate()
	sorted.sort()
	return ",".join(sorted.map(func(item): return str(item)))


static func _rule_for_index(index: int) -> Dictionary:
	match index:
		0:
			return {
				"id": "no_division",
				"title": "今日规则：禁用除法",
				"text": "三题都不能使用 ÷，全部答对可领取完整奖励。",
				"max_digit": 9,
				"forbidden_operator": "÷",
			}
		1:
			return {
				"id": "no_undo",
				"title": "今日规则：一步到底",
				"text": "今天不能撤销，每一步都要先想清楚。",
				"max_digit": 9,
			}
		2:
			return {
				"id": "big_digits",
				"title": "今日规则：进阶数字",
				"text": "题目会出现 10～13，观察数字组合再开始。",
				"max_digit": 13,
			}
		3:
			return {
				"id": "must_subtract",
				"title": "今日规则：必须减法",
				"text": "每题至少使用一次减法，才算完成挑战。",
				"max_digit": 9,
				"required_operator": "−",
			}
		4:
			return {
				"id": "must_multiply",
				"title": "今日规则：必须乘法",
				"text": "每题至少使用一次乘法，找出能快速合并的数字。",
				"max_digit": 9,
				"required_operator": "×",
			}
		5:
			return {
				"id": "no_multiply",
				"title": "今日规则：禁用乘法",
				"text": "今天不能使用 ×，尝试用加减除完成目标。",
				"max_digit": 9,
				"forbidden_operator": "×",
			}
		6:
			return {
				"id": "no_add",
				"title": "今日规则：禁用加法",
				"text": "今天不能使用 +，先观察减法和除法的组合。",
				"max_digit": 9,
				"forbidden_operator": "+",
			}
		_:
			return {
				"id": "quick_start",
				"title": "今日规则：快速出手",
				"text": "每题限时更短，连续答对可以获得额外分数。",
				"max_digit": 13,
				"time_bonus": true,
			}
