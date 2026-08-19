const rankService = require('./rank_service.js');

function asObject(value) {
  return value && typeof value === 'object' && !Array.isArray(value) ? value : {};
}

function unwrap(value) {
  let source = value;
  for (let index = 0; index < 2; index += 1) {
    if (!source || typeof source !== 'object' || Array.isArray(source)) break;
    if (source.data && typeof source.data === 'object' && !Array.isArray(source.data)
      && !source.summary && !source.matches && !source.records && !source.items) {
      source = source.data;
    } else break;
  }
  return source;
}

function int(value, fallback = 0, min = -Infinity, max = Infinity) {
  const number = Number(value);
  if (!Number.isFinite(number)) return fallback;
  return Math.max(min, Math.min(max, Math.floor(number)));
}

function number(value, fallback = 0, min = -Infinity, max = Infinity) {
  const result = Number(value);
  if (!Number.isFinite(result)) return fallback;
  return Math.max(min, Math.min(max, result));
}

function text(value, fallback = '') {
  return String(value === undefined || value === null ? fallback : value).trim();
}

function normalizeSummary(payload, fallbackRank = {}) {
  const source = unwrap(payload);
  const raw = asObject(source.summary || source.rank_summary || source.stats || source);
  const rankSource = asObject(raw.rank || raw.current_rank || raw);
  const rank = rankService.normalize(Object.assign({}, fallbackRank, rankSource), raw.season_id || raw.seasonId);
  const total = int(raw.ranked_matches ?? raw.total_matches ?? rank.ranked_matches, rank.ranked_matches, 0, 999999);
  const wins = int(raw.wins ?? rank.wins, rank.wins, 0, total);
  const losses = int(raw.losses ?? rank.losses, rank.losses, 0, total);
  const draws = int(raw.draws ?? rank.draws, rank.draws, 0, total);
  const winRate = raw.win_rate ?? raw.winRate;
  return {
    ...rankService.summary(rank),
    season_id: text(raw.season_id || raw.seasonId || rank.season_id),
    rating: int(raw.rating ?? raw.mmr, rank.rating, 0, 999999),
    ranked_matches: total,
    wins,
    losses,
    draws,
    win_rate: winRate === undefined ? (total > 0 ? wins / total : 0) : number(winRate, 0, 0, 1),
    current_streak: int(raw.current_streak ?? raw.currentStreak, 0, 0, 999999),
    best_streak: int(raw.best_streak ?? raw.bestStreak, 0, 0, 999999),
    verified: true,
  };
}

function normalizeOutcome(value, won) {
  const outcome = text(value || won, '').toLowerCase();
  if (['win', 'won', 'victory', '1', 'true'].includes(outcome)) return 'win';
  if (['lose', 'loss', 'lost', 'defeat', '0', 'false'].includes(outcome)) return 'lose';
  return 'draw';
}

function normalizeMatch(value, index = 0) {
  const source = asObject(value);
  const opponent = asObject(source.opponent || source.enemy || source.other_player);
  const outcome = normalizeOutcome(source.outcome || source.result || source.result_type, source.won);
  const before = source.rank_before || source.before_rank || source.previous_rank || null;
  const after = source.rank_after || source.after_rank || source.current_rank || null;
  return {
    match_id: text(source.match_id || source.matchId || source.id || `ranked-${index + 1}`),
    outcome,
    opponent_name: text(opponent.nickname || opponent.name || source.opponent_name || source.opponentName, '对手'),
    opponent_avatar: text(opponent.avatar || opponent.avatar_url || source.opponent_avatar, ''),
    mode: text(source.mode || source.match_mode || 'quick_match'),
    score: int(source.score ?? source.player_score, 0, 0, 999999999),
    solved: int(source.solved ?? source.player_solved ?? source.questions_solved, 0, 0, 9999),
    question_count: int(source.question_count ?? source.questions ?? source.total_questions, 0, 0, 9999),
    elapsed_ms: int(source.elapsed_ms ?? source.player_elapsed_ms ?? (Number(source.elapsed || 0) * 1000), 0, 0, 86400000),
    mistakes: int(source.mistakes ?? source.player_mistakes, 0, 0, 9999),
    rating_delta: int(source.rating_delta ?? source.rank_delta ?? source.star_delta, 0, -9999, 9999),
    rank_before: before && typeof before === 'object' ? before : null,
    rank_after: after && typeof after === 'object' ? after : null,
    created_at: text(source.created_at || source.createdAt || source.finished_at || source.updated_at, ''),
    verified: source.verified !== undefined ? Boolean(source.verified) : true,
  };
}

function normalizeMatches(payload) {
  const source = unwrap(payload);
  const list = Array.isArray(source)
    ? source
    : source.matches || source.records || source.items || source.entries || [];
  return (Array.isArray(list) ? list : []).map((item, index) => normalizeMatch(item, index));
}

function normalizePage(payload) {
  const source = unwrap(payload);
  const page = asObject(source.page || source.pagination);
  const cursor = text(source.next_cursor || source.nextCursor || page.next_cursor || page.nextCursor, '');
  const hasMore = source.has_more !== undefined
    ? Boolean(source.has_more)
    : page.has_more !== undefined ? Boolean(page.has_more) : Boolean(cursor);
  return { matches: normalizeMatches(source), next_cursor: cursor, has_more: hasMore };
}

function modeLabel(value) {
  const mode = text(value).toLowerCase();
  if (mode.includes('friend')) return '好友房';
  if (mode.includes('bot')) return '快速匹配';
  if (mode.includes('quick') || mode.includes('rank')) return '快速匹配';
  return '排位对战';
}

function outcomeLabel(value) {
  return value === 'win' ? '胜利' : value === 'lose' ? '失败' : '平局';
}

module.exports = {
  normalizeSummary,
  normalizeMatch,
  normalizeMatches,
  normalizePage,
  modeLabel,
  outcomeLabel,
};
