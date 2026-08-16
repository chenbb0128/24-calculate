class_name PlayerStats
extends RefCounted

## 玩家挑战记录。只记录已验证的答对题目，适合本地原型和后续微信云存档共用。

const DEFAULT_STATS := {
	"total_solved": 0,
	"total_score": 0,
	"fastest_ms": 0,
	"best_combo": 0,
	"best_level": 0,
	"best_chapter": 0,
	"operator_counts": {},
	"mode_questions": {},
	"last_solve": {},
}


static func ensure_progress(progress: Dictionary) -> void:
	if not progress.has("player_stats") or not (progress["player_stats"] is Dictionary):
		progress["player_stats"] = DEFAULT_STATS.duplicate(true)
		return
	var stats: Dictionary = progress["player_stats"]
	for key in DEFAULT_STATS.keys():
		if not stats.has(key):
			stats[key] = DEFAULT_STATS[key].duplicate(true) if DEFAULT_STATS[key] is Dictionary else DEFAULT_STATS[key]
	progress["player_stats"] = stats


static func record_solve(progress: Dictionary, mode_id: String, elapsed_ms: int, score: int, combo: int, operators: Array, level_index: int = -1) -> void:
	ensure_progress(progress)
	var stats: Dictionary = progress["player_stats"]
	stats["total_solved"] = int(stats.get("total_solved", 0)) + 1
	stats["total_score"] = int(stats.get("total_score", 0)) + maxi(0, score)
	stats["best_combo"] = maxi(int(stats.get("best_combo", 0)), combo)
	if elapsed_ms > 0 and (int(stats.get("fastest_ms", 0)) == 0 or elapsed_ms < int(stats.get("fastest_ms", 0))):
		stats["fastest_ms"] = elapsed_ms
	if level_index >= 0:
		stats["best_level"] = maxi(int(stats.get("best_level", 0)), level_index + 1)
		stats["best_chapter"] = maxi(int(stats.get("best_chapter", 0)), int(level_index / 20) + 1)
	var operator_counts: Dictionary = stats.get("operator_counts", {})
	for operator in operators:
		var key := str(operator)
		operator_counts[key] = int(operator_counts.get(key, 0)) + 1
	stats["operator_counts"] = operator_counts
	var mode_questions: Dictionary = stats.get("mode_questions", {})
	mode_questions[mode_id] = int(mode_questions.get(mode_id, 0)) + 1
	stats["mode_questions"] = mode_questions
	stats["last_solve"] = {"mode": mode_id, "elapsed_ms": elapsed_ms, "score": score}
	progress["player_stats"] = stats


static func summary(progress: Dictionary) -> Dictionary:
	ensure_progress(progress)
	var stats: Dictionary = progress["player_stats"]
	var operator_counts: Dictionary = stats.get("operator_counts", {})
	var favorite := "暂无"
	var favorite_count := 0
	for operator in operator_counts.keys():
		if int(operator_counts[operator]) > favorite_count:
			favorite = str(operator)
			favorite_count = int(operator_counts[operator])
	return {
		"total_solved": int(stats.get("total_solved", 0)),
		"total_score": int(stats.get("total_score", 0)),
		"fastest_ms": int(stats.get("fastest_ms", 0)),
		"best_combo": int(stats.get("best_combo", 0)),
		"best_level": int(stats.get("best_level", 0)),
		"best_chapter": int(stats.get("best_chapter", 0)),
		"favorite_operator": favorite,
		"favorite_count": favorite_count,
		"mode_questions": stats.get("mode_questions", {}).duplicate(true),
	}
