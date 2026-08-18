const assert = require('assert');
const fs = require('fs');
const path = require('path');
const apiClient = require('../src/services/api_client.js');
const rankHistory = require('../src/services/rank_history_service.js');

assert.strictEqual(typeof apiClient.getRankedSummary, 'function');
assert.strictEqual(typeof apiClient.getRankedMatches, 'function');
assert.strictEqual(typeof apiClient.getRankedMatch, 'function');

const apiSource = fs.readFileSync(path.join(__dirname, '..', 'src', 'services', 'api_client.js'), 'utf8');
assert.ok(apiSource.includes('/api/v1/player/ranked/summary'), '缺少排位概览接口');
assert.ok(apiSource.includes('/api/v1/player/ranked/matches'), '缺少排位记录接口');

const summary = rankHistory.normalizeSummary({
  summary: {
    season_id: '2026-S3',
    tier: 'gold',
    division: 2,
    stars: 3,
    rating: 1418,
    ranked_matches: 12,
    wins: 8,
    losses: 3,
    draws: 1,
    current_streak: 2,
    best_streak: 4,
  },
}, {});
assert.strictEqual(summary.season_id, '2026-S3');
assert.strictEqual(summary.wins, 8);
assert.strictEqual(summary.losses, 3);
assert.strictEqual(summary.draws, 1);
assert.strictEqual(summary.win_rate, 8 / 12);

const page = rankHistory.normalizePage({
  matches: [{
    match_id: 'm-1',
    outcome: 'win',
    opponent: { nickname: '玩家A' },
    mode: 'quick_match',
    player_score: 640,
    player_solved: 8,
    question_count: 8,
    elapsed_ms: 92300,
    mistakes: 1,
    rating_delta: 24,
    verified: true,
  }],
  next_cursor: 'cursor-2',
  has_more: true,
});
assert.strictEqual(page.matches.length, 1);
assert.strictEqual(page.matches[0].match_id, 'm-1');
assert.strictEqual(page.matches[0].outcome, 'win');
assert.strictEqual(page.matches[0].opponent_name, '玩家A');
assert.strictEqual(page.matches[0].rating_delta, 24);
assert.strictEqual(page.next_cursor, 'cursor-2');
assert.strictEqual(page.has_more, true);

const unsafe = rankHistory.normalizeMatch({ outcome: 'unknown', score: 'not-a-number', elapsed_ms: Infinity });
assert.strictEqual(unsafe.outcome, 'draw');
assert.strictEqual(unsafe.score, 0);
assert.strictEqual(unsafe.elapsed_ms, 0);

console.log('RANK_HISTORY_AUDIT_OK');
