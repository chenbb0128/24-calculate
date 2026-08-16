class_name PuzzleGenerator
extends RefCounted

## 24 点题目生成器。
## 只生成并验证程序搜索过的题目，不依赖 AI 随机编造答案。

const OPERATORS := ["+", "-", "×", "÷"]
const MAX_SAVED_SOLUTIONS := 20

var random := RandomNumberGenerator.new()
var solution_cache := {}


func generate_level(level_config: Dictionary, level_index: int, count: int, seed_override: int = -1) -> Array:
	random.seed = seed_override if seed_override >= 0 else 240000 + level_index * 9973
	var result: Array = []
	var used_keys := {}
	var candidates: Array = _candidate_pool(level_config)
	var start_index := (level_index * 7) % maxi(1, candidates.size())
	for offset in range(candidates.size()):
		if result.size() >= count:
			break
		var numbers: Array = candidates[(start_index + offset) % candidates.size()].duplicate()
		var key := _numbers_key(numbers)
		if used_keys.has(key):
			continue
		var solved := solve(numbers)
		if solved.is_empty():
			continue
		if not _fits_difficulty(solved.size(), level_config):
			continue
		if not _fits_special_rule(solved, level_config):
			continue
		used_keys[key] = true
		result.append(_make_record(numbers, solved, level_index, result.size()))

	# 极端情况下仍然保证关卡可玩：再用固定种子随机补题，但有严格上限。
	if result.size() < count:
		var attempts := 0
		while result.size() < count and attempts < 100:
			attempts += 1
			var fallback_numbers := _random_numbers(level_config)
			var fallback_key := _numbers_key(fallback_numbers)
			if used_keys.has(fallback_key):
				continue
			var fallback_solutions := solve(fallback_numbers)
			if not fallback_solutions.is_empty() and _fits_difficulty(fallback_solutions.size(), level_config) and _fits_special_rule(fallback_solutions, level_config):
				used_keys[fallback_key] = true
				result.append(_make_record(fallback_numbers, fallback_solutions, level_index, result.size()))
	return result


func solve(numbers: Array) -> Array:
	var cache_key := _numbers_key(numbers)
	if solution_cache.has(cache_key):
		return solution_cache[cache_key].duplicate()
	# 用位掩码表示“已经使用了哪些数字”。4 个数字只有 15 个子集，
	# 比直接重复展开完整表达式树更适合在手机启动时生成关卡。
	var dp := {}
	for index in range(numbers.size()):
		var mask := 1 << index
		dp[mask] = {int(numbers[index]): {"expression": str(numbers[index]), "ways": 1}}
	for mask in range(1, 1 << numbers.size()):
		if (mask & (mask - 1)) == 0:
			continue
		var values := {}
		var left_mask := (mask - 1) & mask
		while left_mask > 0:
			var right_mask := mask ^ left_mask
			if right_mask != 0 and left_mask < right_mask:
				for left_value in dp[left_mask].keys():
					var left_entry: Dictionary = dp[left_mask][left_value]
					for right_value in dp[right_mask].keys():
						var right_entry: Dictionary = dp[right_mask][right_value]
						var left_number := int(left_value)
						var right_number := int(right_value)
						var ways := mini(MAX_SAVED_SOLUTIONS, int(left_entry["ways"]) * int(right_entry["ways"]))
						_store_value(values, left_number + right_number, "(%s + %s)" % [left_entry["expression"], right_entry["expression"]], ways)
						_store_value(values, left_number * right_number, "(%s × %s)" % [left_entry["expression"], right_entry["expression"]], ways)
						_store_value(values, left_number - right_number, "(%s - %s)" % [left_entry["expression"], right_entry["expression"]], ways)
						_store_value(values, right_number - left_number, "(%s - %s)" % [right_entry["expression"], left_entry["expression"]], ways)
						if right_number != 0 and left_number % right_number == 0:
							_store_value(values, int(left_number / right_number), "(%s ÷ %s)" % [left_entry["expression"], right_entry["expression"]], ways)
						if left_number != 0 and right_number % left_number == 0:
							_store_value(values, int(right_number / left_number), "(%s ÷ %s)" % [right_entry["expression"], left_entry["expression"]], ways)
			left_mask = (left_mask - 1) & mask
		dp[mask] = values
	var full_mask := (1 << numbers.size()) - 1
	var solutions: Array = []
	if dp[full_mask].has(24):
		var answer: Dictionary = dp[full_mask][24]
		for _i in range(int(answer["ways"])):
			solutions.append(answer["expression"])
	solution_cache[cache_key] = solutions.duplicate()
	return solutions


func make_verified_record(numbers: Array, level_index: int, question_index: int, min_solutions: int = 1, max_solutions: int = MAX_SAVED_SOLUTIONS, required_operator: String = "", forbidden_operator: String = "") -> Dictionary:
	# 普通题直接使用缓存的穷举结果；带特殊规则的题目必须保留“是否使用过某运算符”状态，
	# 否则同一个中间结果只保存一种表达式时，可能误删真正符合规则的解法。
	var normalized_required := _normalize_operator(required_operator)
	var normalized_forbidden := _normalize_operator(forbidden_operator)
	var valid_solutions: Array = solve(numbers) if normalized_required.is_empty() and normalized_forbidden.is_empty() else _solve_with_operator_rules(numbers, normalized_required, normalized_forbidden)
	if valid_solutions.size() < min_solutions or valid_solutions.size() > max_solutions:
		return {}
	var record := _make_record(numbers, valid_solutions, level_index, question_index)
	record["solution_count"] = valid_solutions.size()
	record["all_solution_count"] = solve(numbers).size()
	if not normalized_required.is_empty() or not normalized_forbidden.is_empty():
		record["rules"]["required_operator"] = normalized_required
		record["rules"]["forbidden_operator"] = normalized_forbidden
	return record


func _solve_with_operator_rules(numbers: Array, required_operator: String, forbidden_operator: String) -> Array:
	var dp: Dictionary = {}
	for index in range(numbers.size()):
		var mask := 1 << index
		dp[mask] = {}
		var initial_key := _rule_state_key(int(numbers[index]), false)
		dp[mask][initial_key] = {
			"value": int(numbers[index]),
			"required": false,
			"expressions": [str(numbers[index])],
		}

	for mask in range(1, 1 << numbers.size()):
		if (mask & (mask - 1)) == 0:
			continue
		var values: Dictionary = {}
		var left_mask := (mask - 1) & mask
		while left_mask > 0:
			var right_mask := mask ^ left_mask
			if right_mask != 0 and left_mask < right_mask:
				for left_state in dp[left_mask].values():
					for right_state in dp[right_mask].values():
						var left_value := int(left_state["value"])
						var right_value := int(right_state["value"])
						var left_expressions: Array = left_state["expressions"]
						var right_expressions: Array = right_state["expressions"]
						var inherited_required := bool(left_state["required"]) or bool(right_state["required"])
						_add_rule_operation(values, left_value + right_value, "+", left_expressions, right_expressions, inherited_required, required_operator, forbidden_operator)
						_add_rule_operation(values, left_value * right_value, "×", left_expressions, right_expressions, inherited_required, required_operator, forbidden_operator)
						_add_rule_operation(values, left_value - right_value, "-", left_expressions, right_expressions, inherited_required, required_operator, forbidden_operator)
						_add_rule_operation(values, right_value - left_value, "-", right_expressions, left_expressions, inherited_required, required_operator, forbidden_operator)
						if right_value != 0 and left_value % right_value == 0:
							_add_rule_operation(values, int(left_value / right_value), "÷", left_expressions, right_expressions, inherited_required, required_operator, forbidden_operator)
						if left_value != 0 and right_value % left_value == 0:
							_add_rule_operation(values, int(right_value / left_value), "÷", right_expressions, left_expressions, inherited_required, required_operator, forbidden_operator)
			left_mask = (left_mask - 1) & mask
		dp[mask] = values

	var full_mask := (1 << numbers.size()) - 1
	var solutions: Array = []
	for state in dp.get(full_mask, {}).values():
		if int(state["value"]) != 24:
			continue
		if not required_operator.is_empty() and not bool(state["required"]):
			continue
		for expression in state["expressions"]:
			if not solutions.has(expression):
				solutions.append(expression)
			if solutions.size() >= MAX_SAVED_SOLUTIONS:
				return solutions
	return solutions


func _add_rule_operation(values: Dictionary, value: int, symbol: String, left_expressions: Array, right_expressions: Array, inherited_required: bool, required_operator: String, forbidden_operator: String) -> void:
	if not forbidden_operator.is_empty() and symbol == forbidden_operator:
		return
	var has_required := inherited_required or symbol == required_operator
	var state_key := _rule_state_key(value, has_required)
	if not values.has(state_key):
		values[state_key] = {
			"value": value,
			"required": has_required,
			"expressions": [],
		}
	var expressions: Array = values[state_key]["expressions"]
	for left_expression in left_expressions:
		for right_expression in right_expressions:
			var expression := "(%s %s %s)" % [left_expression, symbol, right_expression]
			if not expressions.has(expression):
				expressions.append(expression)
			if expressions.size() >= MAX_SAVED_SOLUTIONS:
				values[state_key]["expressions"] = expressions
				return
	values[state_key]["expressions"] = expressions


func _rule_state_key(value: int, required: bool) -> String:
	return "%d|%d" % [value, 1 if required else 0]


func _normalize_operator(operator: String) -> String:
	if operator == "−" or operator == "–" or operator == "—":
		return "-"
	return operator


func _store_value(values: Dictionary, value: int, expression: String, ways: int) -> void:
	if not values.has(value):
		values[value] = {"expression": expression, "ways": 0}
	var entry: Dictionary = values[value]
	entry["ways"] = mini(MAX_SAVED_SOLUTIONS, int(entry["ways"]) + ways)
	values[value] = entry


func _search(states: Array, solutions: Array, seen: Dictionary, visited_states: Dictionary) -> void:
	if solutions.size() >= MAX_SAVED_SOLUTIONS:
		return
	var state_key := _state_key(states)
	if visited_states.has(state_key):
		return
	visited_states[state_key] = true
	if states.size() == 1:
		if int(states[0].value) == 24:
			var expression: String = states[0].expression
			if not seen.has(expression):
				seen[expression] = true
				solutions.append(expression)
		return

	for i in range(states.size()):
		for j in range(i + 1, states.size()):
			var left: Dictionary = states[i]
			var right: Dictionary = states[j]
			var rest: Array = []
			for k in range(states.size()):
				if k != i and k != j:
					rest.append(states[k])

			var candidates: Array = [
				{"value": int(left.value) + int(right.value), "symbol": "+", "a": left.expression, "b": right.expression},
				{"value": int(left.value) * int(right.value), "symbol": "×", "a": left.expression, "b": right.expression},
			]
			# 减法和除法需要尝试两个方向；只保留整数中间结果。
			candidates.append({"value": int(left.value) - int(right.value), "symbol": "-", "a": left.expression, "b": right.expression})
			candidates.append({"value": int(right.value) - int(left.value), "symbol": "-", "a": right.expression, "b": left.expression})
			if int(right.value) != 0 and int(left.value) % int(right.value) == 0:
				candidates.append({"value": int(left.value) / int(right.value), "symbol": "÷", "a": left.expression, "b": right.expression})
			if int(left.value) != 0 and int(right.value) % int(left.value) == 0:
				candidates.append({"value": int(right.value) / int(left.value), "symbol": "÷", "a": right.expression, "b": left.expression})

			for candidate in candidates:
				var next_states := rest.duplicate(true)
				next_states.append({
					"value": int(candidate.value),
					"expression": "(%s %s %s)" % [candidate.a, candidate.symbol, candidate.b],
				})
				_search(next_states, solutions, seen, visited_states)


func _random_numbers(level_config: Dictionary) -> Array:
	var min_digit := int(level_config.get("min_digit", 1))
	var max_digit := int(level_config.get("max_digit", 9))
	var numbers: Array = []
	for _i in range(4):
		numbers.append(random.randi_range(min_digit, max_digit))
	return numbers


func _candidate_pool(level_config: Dictionary) -> Array:
	if int(level_config.get("max_digit", 9)) > 9:
		return _advanced_candidates()
	return _simple_candidates()


func _fits_difficulty(solution_count: int, level_config: Dictionary) -> bool:
	var min_solutions := int(level_config.get("min_solutions", 1))
	var max_solutions := int(level_config.get("max_solutions", 999999))
	return solution_count >= min_solutions and solution_count <= max_solutions


func _fits_special_rule(solutions: Array, level_config: Dictionary) -> bool:
	var required_operator := str(level_config.get("required_operator", ""))
	var forbidden_operator := str(level_config.get("forbidden_operator", ""))
	if required_operator.is_empty() and forbidden_operator.is_empty():
		return true
	for solution in solutions:
		var expression := str(solution)
		if not required_operator.is_empty() and not expression.contains(required_operator):
			continue
		if not forbidden_operator.is_empty() and expression.contains(forbidden_operator):
			continue
		return true
	return false


func _make_record(numbers: Array, solutions: Array, level_index: int, question_index: int) -> Dictionary:
	return {
		"version": 1,
		"puzzle_id": "L%02d-Q%02d" % [level_index + 1, question_index + 1],
		"seed": random.seed,
		"numbers": numbers.duplicate(),
		"target": 24,
		"solution": str(solutions[0]),
		"solution_count": solutions.size(),
		"rules": {
			"use_each_number_once": true,
			"integer_intermediate_results": true,
			"allowed_operators": OPERATORS.duplicate(),
		},
	}


func _numbers_key(numbers: Array) -> String:
	var sorted := numbers.duplicate()
	sorted.sort()
	return ",".join(sorted.map(func(item): return str(item)))


func _state_key(states: Array) -> String:
	var values: Array = []
	for state in states:
		values.append(int(state.value))
	values.sort()
	return ",".join(values.map(func(item): return str(item)))


func _fallback_candidates() -> Array:
	return [
		[1, 2, 3, 4], [1, 2, 3, 8], [1, 1, 4, 6], [1, 3, 4, 6],
		[2, 2, 3, 8], [2, 3, 4, 6], [2, 3, 4, 9], [2, 3, 6, 8],
		[3, 3, 4, 6], [3, 4, 6, 9], [4, 4, 6, 8], [5, 5, 5, 5],
		[1, 5, 5, 5], [2, 4, 7, 8], [3, 5, 7, 9], [6, 7, 8, 9],
		[4, 6, 10, 12], [3, 7, 11, 13], [2, 8, 10, 13], [5, 6, 11, 13],
	]


func _simple_candidates() -> Array:
	return [
		[1, 1, 1, 8], [1, 1, 2, 6], [1, 1, 2, 7], [1, 1, 2, 9],
		[1, 1, 3, 4], [1, 1, 3, 5], [1, 1, 4, 4], [1, 1, 4, 9],
		[1, 1, 5, 7], [1, 1, 5, 8], [1, 2, 2, 4], [1, 2, 2, 5],
		[1, 2, 2, 7], [1, 2, 2, 8], [1, 2, 2, 9], [1, 2, 3, 3],
		[1, 2, 4, 5], [1, 2, 5, 5], [1, 3, 3, 3], [1, 3, 5, 6],
	]


func _advanced_candidates() -> Array:
	return [
		[1, 1, 1, 11], [1, 1, 1, 13], [1, 1, 3, 13], [1, 1, 4, 12],
		[1, 2, 3, 13], [1, 2, 4, 11], [1, 2, 4, 13], [1, 2, 5, 13],
		[1, 2, 6, 10], [1, 2, 6, 11], [1, 2, 6, 13], [1, 2, 7, 11],
		[1, 2, 7, 12], [1, 3, 5, 11], [1, 3, 6, 11], [1, 3, 7, 12],
		[1, 3, 8, 10], [1, 3, 8, 11], [1, 3, 12, 13], [1, 4, 5, 13],
		[1, 4, 10, 12], [1, 6, 10, 12], [1, 6, 10, 13], [1, 6, 11, 12],
		[1, 6, 12, 13], [2, 3, 8, 13], [2, 3, 9, 13], [2, 4, 10, 13],
		[2, 6, 10, 13], [2, 6, 11, 12], [2, 8, 10, 13], [3, 4, 6, 13],
	]
