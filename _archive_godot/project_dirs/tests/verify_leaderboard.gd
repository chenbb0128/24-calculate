extends SceneTree

const LeaderboardServiceScript = preload("res://services/leaderboard_service.gd")


func _init() -> void:
	var progress := {
		"levels": {
			"0": {"best_score": 1200},
			"1": {"best_score": 1800},
		},
		"endless": {"best_score": 3500},
		"daily": {"best_score": 420},
	}
	LeaderboardServiceScript.ensure_progress(progress)
	_assert(LeaderboardServiceScript.player_score(progress, LeaderboardServiceScript.MODE_CAMPAIGN) == 3000, "campaign score sums level records")
	_assert(LeaderboardServiceScript.player_score(progress, LeaderboardServiceScript.MODE_ENDLESS) == 3500, "endless score reads record")
	_assert(LeaderboardServiceScript.player_score(progress, LeaderboardServiceScript.MODE_DAILY) == 420, "daily reads legacy record")

	var daily_submit := LeaderboardServiceScript.submit_score(progress, LeaderboardServiceScript.MODE_DAILY, 560, {"questions": 3})
	_assert(bool(daily_submit["new_record"]), "daily new record accepted")
	_assert(LeaderboardServiceScript.player_score(progress, LeaderboardServiceScript.MODE_DAILY) == 560, "daily score saved")
	var old_submit := LeaderboardServiceScript.submit_score(progress, LeaderboardServiceScript.MODE_DAILY, 300)
	_assert(not bool(old_submit["new_record"]), "lower score does not replace record")

	for board_id in [LeaderboardServiceScript.BOARD_FRIENDS, LeaderboardServiceScript.BOARD_GLOBAL]:
		for mode_id in LeaderboardServiceScript.mode_ids():
			var entries: Array = LeaderboardServiceScript.get_entries(progress, board_id, mode_id)
			_assert(not entries.is_empty(), "entries exist")
			for index in range(1, entries.size()):
				_assert(int(entries[index - 1]["score"]) >= int(entries[index]["score"]), "entries sorted descending")
			_assert(bool(entries.back().has("rank")), "rank assigned")

	print("Leaderboard verification passed: two boards, three modes, score submission and sorting")
	quit()


func _assert(condition: bool, message: String) -> void:
	if not condition:
		push_error("Leaderboard verification failed: " + message)
		quit(1)
