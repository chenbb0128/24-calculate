class_name FriendMatchService
extends RefCounted

## 好友同题竞速的规则与房间数据。
## 当前桌面版用本地模拟对手；微信版只替换房间同步，不改变题目和计分结构。
const QUESTION_COUNT := 8
const TIME_LIMIT := 120.0
const DAILY_REWARD_MATCH_LIMIT := 3


static func create_room(seed_value: int, owner_name: String = "我") -> Dictionary:
	var safe_seed := absi(seed_value)
	var room_code := "%06d" % (100000 + (safe_seed % 900000))
	return {
		"version": 1,
		"room_id": "friend-%s" % room_code,
		"room_code": room_code,
		"room_seed": safe_seed,
		"owner": {"id": "local-player", "name": owner_name},
		"players": [{"id": "local-player", "name": owner_name, "ready": true}],
		"status": "waiting",
		"rules": rules(),
	}


static func rules() -> Dictionary:
	return {
		"question_count": QUESTION_COUNT,
		"time_limit": TIME_LIMIT,
		"target": 24,
		"no_hint": true,
		"use_same_seed": true,
		"integer_intermediate_results": true,
	}


static func join_room(room: Dictionary, player_id: String = "friend-local", player_name: String = "好友") -> Dictionary:
	var joined := room.duplicate(true)
	var players: Array = joined.get("players", []).duplicate(true)
	for player in players:
		if str(player.get("id", "")) == player_id:
			joined["status"] = "ready"
			return joined
	players.append({"id": player_id, "name": player_name, "ready": true})
	joined["players"] = players
	joined["status"] = "ready"
	return joined


static func generate_puzzles(generator: RefCounted, room_seed: int) -> Array:
	var config := {
		"min_digit": 1,
		"max_digit": 9,
		"min_solutions": 1,
		"max_solutions": 12,
		"question_count": QUESTION_COUNT,
	}
	var puzzles: Array = generator.generate_level(config, absi(room_seed) % 97, QUESTION_COUNT, absi(room_seed))
	if puzzles.size() < QUESTION_COUNT:
		config["max_solutions"] = 999999
		puzzles = generator.generate_level(config, absi(room_seed) % 97, QUESTION_COUNT, absi(room_seed))
	return puzzles


static func create_match(room: Dictionary, puzzles: Array) -> Dictionary:
	return {
		"version": 1,
		"match_id": str(room.get("room_id", "friend-local")),
		"room_id": str(room.get("room_id", "friend-local")),
		"room_code": str(room.get("room_code", "")),
		"room_seed": int(room.get("room_seed", 0)),
		"rules": rules(),
		"puzzles": puzzles.duplicate(true),
		"players": room.get("players", []).duplicate(true),
		"events": [],
	}


static func build_opponent_plan(room_seed: int, puzzle_count: int = QUESTION_COUNT) -> Array:
	var random := RandomNumberGenerator.new()
	random.seed = absi(room_seed) + 24024
	var plan: Array = []
	for index in range(puzzle_count):
		# 对手速度有快慢变化，避免每局都出现完全固定的表现。
		plan.append(7.0 + random.randf_range(1.5, 7.5) + float(index % 3) * 0.45)
	return plan


static func opponent_snapshot(plan: Array, elapsed: float, puzzle_count: int = QUESTION_COUNT) -> Dictionary:
	var solved := 0
	var spent := 0.0
	for value in plan:
		spent += float(value)
		if spent <= elapsed and solved < puzzle_count:
			solved += 1
	var score := solved * 100 + maxi(0, int((TIME_LIMIT - elapsed) * 1.5)) if solved > 0 else 0
	return {
		"solved": solved,
		"score": score,
		"elapsed": mini(elapsed, spent),
		"finished": solved >= puzzle_count,
	}


static func calculate_result(player_solved: int, player_score: int, player_mistakes: int, player_elapsed: float, opponent: Dictionary) -> Dictionary:
	var opponent_solved := int(opponent.get("solved", 0))
	var opponent_score := int(opponent.get("score", 0))
	var outcome := "draw"
	if player_solved > opponent_solved or (player_solved == opponent_solved and player_score > opponent_score):
		outcome = "win"
	elif player_solved < opponent_solved or (player_solved == opponent_solved and player_score < opponent_score):
		outcome = "lose"
	return {
		"outcome": outcome,
		"player_solved": player_solved,
		"player_score": player_score,
		"player_mistakes": player_mistakes,
		"player_elapsed": player_elapsed,
		"opponent_solved": opponent_solved,
		"opponent_score": opponent_score,
		"opponent_elapsed": float(opponent.get("elapsed", 0.0)),
	}
