-- name: ListCampaignLeaderboard :many
SELECT level_scores.user_id,
       u.nickname,
       u.avatar,
       SUM(level_scores.best_score) AS score,
       MAX(level_scores.last_created_at) AS last_created_at
FROM (
    SELECT user_id, level_id, MAX(best_score) AS best_score, MAX(created_at) AS last_created_at
    FROM player_level_completions
    GROUP BY user_id, level_id
) AS level_scores
INNER JOIN users AS u ON u.id = level_scores.user_id
WHERE u.status = 1
GROUP BY level_scores.user_id, u.nickname, u.avatar
ORDER BY score DESC, level_scores.user_id ASC;

-- name: ListDailyLeaderboard :many
SELECT completion.user_id,
       u.nickname,
       u.avatar,
       completion.date_key,
       completion.best_score AS score,
       completion.created_at
FROM player_daily_completions AS completion
INNER JOIN users AS u ON u.id = completion.user_id
WHERE u.status = 1 AND completion.date_key = ?
ORDER BY completion.best_score DESC, completion.user_id ASC;
