/* Ranked matchmaking rules shared by the client UI and the future server contract. */

const DEFAULT_RATING = 1000;
const DIVISION_COUNT = 3;
const STARS_PER_DIVISION = 5;

const TIERS = Object.freeze([
  { id: 'bronze', name: '青铜', min_rating: 0, variant: 'gold' },
  { id: 'silver', name: '白银', min_rating: 1100, variant: 'cyan' },
  { id: 'gold', name: '黄金', min_rating: 1300, variant: 'gold' },
  { id: 'platinum', name: '铂金', min_rating: 1500, variant: 'cyan' },
  { id: 'diamond', name: '钻石', min_rating: 1700, variant: 'violet' },
  { id: 'master', name: '大师', min_rating: 1900, variant: 'magenta' },
  { id: 'king', name: '算王', min_rating: 2100, variant: 'magenta' },
]);

function safeInt(value, fallback = 0, min = -Infinity, max = Infinity) {
  const number = Number(value);
  if (!Number.isFinite(number)) return fallback;
  return Math.max(min, Math.min(max, Math.floor(number)));
}

function seasonId(dateValue = new Date()) {
  const source = String(dateValue || '');
  const match = source.match(/^(\d{4})-(\d{1,2})/);
  if (match) {
    const year = Number(match[1]);
    const month = Number(match[2]);
    if (year > 2000 && month >= 1 && month <= 12) return `${year}-S${Math.floor((month - 1) / 3) + 1}`;
  }
  const date = dateValue instanceof Date ? dateValue : new Date(dateValue);
  const safeDate = Number.isNaN(date.getTime()) ? new Date() : date;
  return `${safeDate.getFullYear()}-S${Math.floor(safeDate.getMonth() / 3) + 1}`;
}

function tierForRating(rating) {
  const value = safeInt(rating, DEFAULT_RATING, 0, 9999);
  let selected = TIERS[0];
  TIERS.forEach((tier) => {
    if (value >= tier.min_rating) selected = tier;
  });
  return selected;
}

function tierByID(id) {
  return TIERS.find((tier) => tier.id === String(id || '').toLowerCase()) || TIERS[0];
}

function deriveVisibleRank(rating) {
  const value = safeInt(rating, DEFAULT_RATING, 0, 9999);
  const tier = tierForRating(value);
  const nextTier = TIERS[TIERS.indexOf(tier) + 1];
  const tierWidth = nextTier ? nextTier.min_rating - tier.min_rating : 200;
  const progress = Math.max(0, value - tier.min_rating);
  const divisionWidth = Math.max(1, Math.floor(tierWidth / DIVISION_COUNT));
  const division = Math.max(1, DIVISION_COUNT - Math.min(DIVISION_COUNT - 1, Math.floor(progress / divisionWidth)));
  const divisionProgress = progress % divisionWidth;
  const stars = Math.max(0, Math.min(STARS_PER_DIVISION - 1, Math.floor((divisionProgress / divisionWidth) * STARS_PER_DIVISION)));
  return { tier: tier.id, division, stars };
}

function rankPosition(value) {
  const rank = value && typeof value === 'object' ? value : {};
  const tierIndex = Math.max(0, TIERS.findIndex((tier) => tier.id === String(rank.tier || '').toLowerCase()));
  const division = safeInt(rank.division, DIVISION_COUNT, 1, DIVISION_COUNT);
  const stars = safeInt(rank.stars, 0, 0, STARS_PER_DIVISION - 1);
  return tierIndex * DIVISION_COUNT * STARS_PER_DIVISION
    + (DIVISION_COUNT - division) * STARS_PER_DIVISION + stars;
}

function rankFromPosition(position) {
  const maxPosition = TIERS.length * DIVISION_COUNT * STARS_PER_DIVISION - 1;
  const safePosition = safeInt(position, 0, 0, maxPosition);
  const tierSize = DIVISION_COUNT * STARS_PER_DIVISION;
  const tierIndex = Math.min(TIERS.length - 1, Math.floor(safePosition / tierSize));
  const withinTier = safePosition % tierSize;
  const division = DIVISION_COUNT - Math.floor(withinTier / STARS_PER_DIVISION);
  const stars = withinTier % STARS_PER_DIVISION;
  return { tier: TIERS[tierIndex].id, division, stars };
}

function defaults(dateValue = new Date()) {
  return {
    season_id: seasonId(dateValue),
    rating: DEFAULT_RATING,
    tier: 'bronze',
    division: 3,
    stars: 0,
    placement_matches: 0,
    ranked_matches: 0,
    wins: 0,
    losses: 0,
    draws: 0,
    best_tier: 'bronze',
    last_delta: 0,
    last_outcome: '',
    last_match_id: '',
    updated_at: 0,
  };
}

function normalize(value, dateValue = new Date()) {
  const source = value && typeof value === 'object' ? value : {};
  const currentSeason = seasonId(dateValue);
  const sourceSeason = String(source.season_id || source.seasonId || '');
  // A stale season starts from the default rating but keeps no stale stars.
  const sameSeason = !sourceSeason || sourceSeason === currentSeason;
  const result = Object.assign(defaults(dateValue), sameSeason ? source : {});
  result.season_id = currentSeason;
  result.rating = safeInt(result.rating, DEFAULT_RATING, 0, 9999);
  const visible = deriveVisibleRank(result.rating);
  const explicitTier = source.tier || source.rank_tier;
  const explicitDivision = source.division ?? source.rank_division;
  const explicitStars = source.stars;
  result.tier = explicitTier ? tierByID(explicitTier).id : visible.tier;
  result.division = explicitDivision !== undefined
    ? safeInt(explicitDivision, visible.division, 1, DIVISION_COUNT) : visible.division;
  result.stars = explicitStars !== undefined
    ? safeInt(explicitStars, visible.stars, 0, STARS_PER_DIVISION - 1) : visible.stars;
  result.placement_matches = safeInt(result.placement_matches, 0, 0, 5);
  result.ranked_matches = safeInt(result.ranked_matches, 0, 0, 999999);
  result.wins = safeInt(result.wins, 0, 0, 999999);
  result.losses = safeInt(result.losses, 0, 0, 999999);
  result.draws = safeInt(result.draws, 0, 0, 999999);
  result.best_tier = tierByID(result.best_tier || result.tier).id;
  result.last_delta = safeInt(result.last_delta, 0, -999, 999);
  result.last_outcome = String(result.last_outcome || '').slice(0, 16);
  result.last_match_id = String(result.last_match_id || '').slice(0, 128);
  result.updated_at = safeInt(result.updated_at, 0, 0, Number.MAX_SAFE_INTEGER);
  return result;
}

function summary(value) {
  const rank = normalize(value);
  const tier = tierByID(rank.tier);
  return {
    season_id: rank.season_id,
    tier: tier.id,
    tier_name: tier.name,
    division: rank.division,
    stars: rank.stars,
    max_stars: STARS_PER_DIVISION,
    rating: rank.rating,
    label: `${tier.name} ${['', 'I', 'II', 'III'][rank.division] || 'III'}`,
    stars_label: `${rank.stars}/${STARS_PER_DIVISION} 星`,
    variant: tier.variant,
    placement_matches: rank.placement_matches,
    placement_left: Math.max(0, 5 - rank.placement_matches),
  };
}

function hasRankFields(value) {
  if (!value || typeof value !== 'object') return false;
  return ['rating', 'tier', 'division', 'stars', 'rank_delta', 'rating_delta', 'star_delta', 'season_id', 'seasonId'].some((key) => value[key] !== undefined);
}

function extractRankPayload(payload) {
  const source = payload && typeof payload === 'object' ? payload : {};
  const candidates = [
    source.rank_result, source.rankResult, source.rank,
    source.match_result && source.match_result.rank_result,
    source.matchResult && source.matchResult.rankResult,
    source.data && source.data.rank_result,
    source.data && source.data.rank,
    source.progress && source.progress.rank,
  ];
  return candidates.find((candidate) => hasRankFields(candidate)) || null;
}

function extractRankSnapshot(payload) {
  const source = extractRankPayload(payload);
  if (!source) return null;
  return {
    season_id: source.season_id || source.seasonId,
    rating: source.rating,
    tier: source.tier || source.rank_tier,
    division: source.division || source.rank_division,
    stars: source.stars,
    placement_matches: source.placement_matches ?? source.placementMatches,
    ranked_matches: source.ranked_matches ?? source.rankedMatches,
    wins: source.wins,
    losses: source.losses,
    draws: source.draws,
    best_tier: source.best_tier || source.bestTier,
  };
}

function applyServerResult(current, payload, outcome = '') {
  const source = extractRankPayload(payload);
  if (!source) return null;
  const before = normalize(current, source.season_id || source.seasonId || new Date());
  const rawRating = source.rating ?? source.mmr;
  const nextRating = rawRating !== undefined ? rawRating : before.rating;
  const visible = deriveVisibleRank(nextRating);
  const next = normalize(Object.assign({}, before, {
    season_id: source.season_id || source.seasonId || before.season_id,
    rating: nextRating,
    tier: source.tier || source.rank_tier || visible.tier,
    division: source.division ?? source.rank_division ?? visible.division,
    stars: source.stars !== undefined ? source.stars : visible.stars,
    placement_matches: source.placement_matches ?? source.placementMatches ?? before.placement_matches,
    ranked_matches: source.ranked_matches ?? source.rankedMatches ?? before.ranked_matches,
    wins: source.wins !== undefined ? source.wins : before.wins,
    losses: source.losses !== undefined ? source.losses : before.losses,
    draws: source.draws !== undefined ? source.draws : before.draws,
    best_tier: source.best_tier || source.bestTier || before.best_tier,
    last_delta: source.star_delta ?? source.rank_delta ?? source.rating_delta ?? before.last_delta,
    last_outcome: outcome || source.outcome || before.last_outcome,
    last_match_id: source.match_id || source.matchId || before.last_match_id,
    updated_at: Date.now(),
  }));
  const delta = safeInt(source.star_delta ?? source.rank_delta ?? source.rating_delta, next.rating - before.rating, -999, 999);
  return {
    profile: next,
    change: {
      eligible: true,
      delta,
      outcome: outcome || String(source.outcome || ''),
      before: summary(before),
      after: summary(next),
      placement: next.placement_matches < 5,
    },
  };
}

// Offline/dev fallback only. Production matches must use the server result;
// this keeps the local hidden-opponent experience playable without pretending
// that the client is authoritative for a real online match.
function applyLocalResult(current, outcome = '', options = {}) {
  const normalizedOutcome = String(outcome || '').toLowerCase();
  if (!['win', 'lose', 'draw'].includes(normalizedOutcome)) return null;
  const before = normalize(current);
  const starDelta = normalizedOutcome === 'win' ? 1 : normalizedOutcome === 'lose' ? -1 : 0;
  const ratingDelta = normalizedOutcome === 'win' ? 32 : normalizedOutcome === 'lose' ? -22 : 0;
  const afterVisible = rankFromPosition(rankPosition(before) + starDelta);
  const beforeBestIndex = TIERS.findIndex((tier) => tier.id === before.best_tier);
  const afterTierIndex = TIERS.findIndex((tier) => tier.id === afterVisible.tier);
  const next = normalize(Object.assign({}, before, {
    rating: safeInt(before.rating + ratingDelta, DEFAULT_RATING, 0, 9999),
    tier: afterVisible.tier,
    division: afterVisible.division,
    stars: afterVisible.stars,
    placement_matches: Math.min(5, before.placement_matches + 1),
    ranked_matches: before.ranked_matches + 1,
    wins: before.wins + (normalizedOutcome === 'win' ? 1 : 0),
    losses: before.losses + (normalizedOutcome === 'lose' ? 1 : 0),
    draws: before.draws + (normalizedOutcome === 'draw' ? 1 : 0),
    best_tier: afterTierIndex > beforeBestIndex ? afterVisible.tier : before.best_tier,
    last_delta: starDelta,
    last_outcome: normalizedOutcome,
    last_match_id: options.match_id || before.last_match_id,
    updated_at: Date.now(),
  }));
  return {
    profile: next,
    change: {
      eligible: true,
      delta: starDelta,
      outcome: normalizedOutcome,
      before: summary(before),
      after: summary(next),
      placement: next.placement_matches < 5,
      local: true,
    },
  };
}

function ineligibleChange(reason = '本局不计入段位') {
  return { eligible: false, delta: 0, reason: String(reason), before: null, after: null, placement: false };
}

function changeLabel(change) {
  if (!change || !change.eligible) return String(change && change.reason || '本局不计入段位');
  if (change.placement) return `定级赛进行中 · 还剩 ${Math.max(0, 5 - Number(change.after.placement_matches || 0))} 局`;
  const delta = Number(change.delta || 0);
  const sign = delta > 0 ? '+' : '';
  const movement = change.before && change.after && change.before.label !== change.after.label
    ? ` · ${change.after.label}`
    : '';
  return `段位 ${sign}${delta} 星${movement}`;
}

module.exports = {
  DEFAULT_RATING,
  DIVISION_COUNT,
  STARS_PER_DIVISION,
  TIERS,
  seasonId,
  defaults,
  normalize,
  summary,
  tierForRating,
  extractRankPayload,
  extractRankSnapshot,
  applyServerResult,
  applyLocalResult,
  ineligibleChange,
  changeLabel,
};
