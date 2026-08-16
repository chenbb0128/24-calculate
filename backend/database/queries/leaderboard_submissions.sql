-- name: GetLeaderboardSubmissionByKey :one
SELECT id, user_id, mode, idempotency_key, score, questions, elapsed_ms, room_id, outcome, metadata_json, created_at
FROM player_leaderboard_submissions
WHERE user_id = ? AND mode = ? AND idempotency_key = ?
LIMIT 1;

-- name: CreateLeaderboardSubmission :exec
INSERT INTO player_leaderboard_submissions (
    user_id,
    mode,
    idempotency_key,
    score,
    questions,
    elapsed_ms,
    room_id,
    outcome,
    metadata_json,
    created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListEndlessLeaderboard :many
SELECT submission.user_id,
       u.nickname,
       u.avatar,
       MAX(submission.score) AS score,
       MAX(submission.created_at) AS last_created_at
FROM player_leaderboard_submissions AS submission
INNER JOIN users AS u ON u.id = submission.user_id
WHERE submission.mode = 'endless' AND u.status = 1
GROUP BY submission.user_id, u.nickname, u.avatar
ORDER BY score DESC, submission.user_id ASC;

-- name: ListFriendLeaderboard :many
SELECT submission.user_id,
       u.nickname,
       u.avatar,
       MAX(submission.score) AS score,
       MAX(submission.created_at) AS last_created_at
FROM player_leaderboard_submissions AS submission
INNER JOIN users AS u ON u.id = submission.user_id
WHERE submission.mode = 'friend' AND u.status = 1
GROUP BY submission.user_id, u.nickname, u.avatar
ORDER BY score DESC, submission.user_id ASC;

-- name: ListFriendUserIDs :many
SELECT DISTINCT candidate.user_id
FROM player_leaderboard_submissions AS candidate
INNER JOIN player_leaderboard_submissions AS mine
    ON mine.mode = 'friend'
   AND mine.user_id = ?
   AND mine.room_id <> ''
   AND mine.room_id = candidate.room_id
WHERE candidate.mode = 'friend'
UNION
SELECT ? AS user_id;
