package player

import (
	"fmt"
)

func campaignAttemptOperators(attempts []CampaignRunAttemptInput) []string {
	operators := make([]string, 0)
	for _, attempt := range attempts {
		operators = append(operators, solutionOperators(attempt.SolutionSteps)...)
	}
	return operators
}

func solutionOperators(steps []FriendMatchSolutionStep) []string {
	operators := make([]string, 0, len(steps))
	for _, step := range steps {
		if step.Operator != "" {
			operators = append(operators, step.Operator)
		}
	}
	return operators
}

func recordServerSolveStats(state map[string]any, mode string, questions, score, elapsedMS, fastestMS, bestCombo, levelID int, operators []string) {
	stats := ensureObject(state, "player_stats")
	questions = maxInt(0, questions)
	stats["total_solved"] = readInt(stats["total_solved"]) + questions
	stats["total_score"] = readInt(stats["total_score"]) + maxInt(0, score)
	stats["best_combo"] = maxInt(readInt(stats["best_combo"]), bestCombo)
	if fastestMS <= 0 {
		fastestMS = elapsedMS
	}
	if fastestMS > 0 && (readInt(stats["fastest_ms"]) <= 0 || fastestMS < readInt(stats["fastest_ms"])) {
		stats["fastest_ms"] = fastestMS
	}
	if levelID >= 0 {
		stats["best_level"] = maxInt(readInt(stats["best_level"]), levelID+1)
		stats["best_chapter"] = maxInt(readInt(stats["best_chapter"]), levelID/20+1)
	}
	modeQuestions := ensureObject(stats, "mode_questions")
	modeQuestions[mode] = readInt(modeQuestions[mode]) + questions
	operatorCounts := ensureObject(stats, "operator_counts")
	for _, operator := range operators {
		if operator != "" {
			operatorCounts[operator] = readInt(operatorCounts[operator]) + 1
		}
	}
	stats["mode_questions"] = modeQuestions
	stats["operator_counts"] = operatorCounts
	stats["last_solve"] = map[string]any{"mode": mode, "elapsed_ms": maxInt(0, elapsedMS), "score": maxInt(0, score)}
	state["player_stats"] = stats
}

func applyEndlessServerResult(state map[string]any, summary endlessSummary, dateKey, runID string) (int, error) {
	events := ensureObject(state, "server_events")
	endlessEvents := ensureObject(events, "endless")
	if readBool(endlessEvents[runID]) {
		return 0, nil
	}

	endless := ensureObject(state, "endless")
	questions := maxInt(0, summary.Questions)
	endless["best_score"] = maxInt(readInt(endless["best_score"]), summary.Score)
	endless["best_questions"] = maxInt(readInt(endless["best_questions"]), questions)
	endless["best_combo"] = maxInt(readInt(endless["best_combo"]), summary.BestCombo)
	endless["best_stage"] = maxInt(readInt(endless["best_stage"]), (maxInt(0, questions-1)/3)+1)
	endless["last_score"] = summary.Score

	reward := claimServerEndlessReward(endless, runID, questions, dateKey)
	reward += recordServerDailyTask(state, "endless_questions", questions, dateKey, 5, 25)
	reward += recordServerDailyTaskMax(state, "combo", summary.BestCombo, dateKey, 5, 20)
	reward += recordServerWeeklyTask(state, "weekly_endless", questions, dateKey, 20, 50)
	reward += unlockServerAchievement(state, "endless_5", questions >= 5, 30)
	reward += unlockServerAchievement(state, "endless_10", questions >= 10, 60)
	reward += unlockServerAchievement(state, "endless_30", questions >= 30, 150)
	reward += unlockServerAchievement(state, "combo_5", summary.BestCombo >= 5, 30)
	reward += unlockServerAchievement(state, "combo_10", summary.BestCombo >= 10, 80)

	recordServerSolveStats(state, "endless", questions, summary.Score, summary.ElapsedMS, 0, summary.BestCombo, -1, nil)

	endlessEvents[runID] = true
	events["endless"] = endlessEvents
	state["server_events"] = events
	state["coins"] = minInt(999999, maxInt(0, readInt(state["coins"])+reward))
	return reward, nil
}

func claimServerEndlessReward(endless map[string]any, runID string, questions int, dateKey string) int {
	if questions <= 0 {
		return 0
	}
	if fmtString(endless["reward_date"]) != dateKey {
		endless["reward_date"] = dateKey
		endless["reward_coins_today"] = 0
	}
	if fmtString(endless["reward_run_id"]) != runID {
		endless["reward_run_id"] = runID
		endless["rewarded_questions"] = map[string]any{}
	}
	rewarded := ensureObject(endless, "rewarded_questions")
	remaining := maxInt(0, 60-readInt(endless["reward_coins_today"]))
	if remaining <= 0 {
		return 0
	}
	milestones := map[int]int{5: 5, 10: 8, 20: 12, 30: 15, 50: 20, 100: 30}
	reward := 0
	for question := 1; question <= questions && reward < remaining; question++ {
		key := fmtInt(question)
		if readBool(rewarded[key]) {
			continue
		}
		reward += 1 + milestones[question]
		rewarded[key] = true
	}
	reward = minInt(reward, remaining)
	endless["reward_coins_today"] = readInt(endless["reward_coins_today"]) + reward
	endless["rewarded_questions"] = rewarded
	return reward
}

func applyFriendServerResult(state map[string]any, result FriendMatchResult, dateKey, matchID string) (int, error) {
	events := ensureObject(state, "server_events")
	friendEvents := ensureObject(events, "friend")
	if readBool(friendEvents[matchID]) {
		return 0, nil
	}

	friend := ensureObject(state, "friend_matches")
	friend["date"] = dateKey
	friend["played"] = readInt(friend["played"]) + 1
	if result.Outcome == "win" {
		friend["wins"] = readInt(friend["wins"]) + 1
	}
	friend["best_score"] = maxInt(readInt(friend["best_score"]), result.PlayerScore)
	if result.PlayerElapsedMS > 0 {
		bestTime := readInt(friend["best_time_ms"])
		if bestTime <= 0 || result.PlayerElapsedMS < bestTime {
			friend["best_time_ms"] = result.PlayerElapsedMS
		}
	}

	reward := 0
	if fmtString(friend["reward_date"]) != dateKey {
		friend["reward_date"] = dateKey
		friend["reward_count"] = 0
	}
	if readInt(friend["reward_count"]) < 3 {
		reward = map[string]int{"win": 15, "draw": 8, "lose": 5}[result.Outcome]
		friend["reward_count"] = readInt(friend["reward_count"]) + 1
	}
	state["friend_matches"] = friend
	reward += recordServerWeeklyTask(state, "weekly_friend", boolToAmount(result.Outcome == "win"), dateKey, 2, 40)
	reward += unlockServerAchievement(state, "friend_first_win", result.Outcome == "win", 40)

	recordServerSolveStats(state, "friend", result.PlayerSolved, result.PlayerScore, result.PlayerElapsedMS, result.PlayerElapsedMS, 0, -1, nil)

	friendEvents[matchID] = true
	events["friend"] = friendEvents
	state["server_events"] = events
	state["coins"] = minInt(999999, maxInt(0, readInt(state["coins"])+reward))
	return reward, nil
}

type FriendMatchResult struct {
	Outcome           string      `json:"outcome"`
	PlayerSolved      int         `json:"player_solved"`
	PlayerScore       int         `json:"player_score"`
	PlayerMistakes    int         `json:"player_mistakes"`
	PlayerElapsedMS   int         `json:"-"`
	PlayerElapsed     float64     `json:"player_elapsed"`
	OpponentSolved    int         `json:"opponent_solved"`
	OpponentScore     int         `json:"opponent_score"`
	OpponentMistakes  int         `json:"opponent_mistakes"`
	OpponentElapsedMS int         `json:"-"`
	OpponentElapsed   float64     `json:"opponent_elapsed"`
	RankResult        *RankResult `json:"rank_result,omitempty"`
}

func compareFriendResults(left, right FriendMatchSubmissionRecord) string {
	if left.Solved != right.Solved {
		if left.Solved > right.Solved {
			return "win"
		}
		return "lose"
	}
	if left.Score != right.Score {
		if left.Score > right.Score {
			return "win"
		}
		return "lose"
	}
	if left.ElapsedMS != right.ElapsedMS {
		if left.ElapsedMS < right.ElapsedMS {
			return "win"
		}
		return "lose"
	}
	return "draw"
}

func fmtString(value any) string { return fmt.Sprint(value) }
func fmtInt(value int) string    { return fmt.Sprintf("%d", value) }
func boolToAmount(value bool) int {
	if value {
		return 1
	}
	return 0
}
