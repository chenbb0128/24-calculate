class_name MatchData
extends RefCounted

## 单机与未来联机共用的数据外形。网络层只需要替换题目来源和提交方式。

static func create_match(match_id: String, puzzles: Array, rules: Dictionary) -> Dictionary:
	return {
		"version": 1,
		"match_id": match_id,
		"rules": rules.duplicate(true),
		"puzzles": puzzles.duplicate(true),
		"players": {},
		"events": [],
	}


static func create_attempt(puzzle_id: String, elapsed_ms: int, solved: bool, mistakes: int, score: int) -> Dictionary:
	return {
		"puzzle_id": puzzle_id,
		"elapsed_ms": elapsed_ms,
		"solved": solved,
		"mistakes": mistakes,
		"score": score,
	}
