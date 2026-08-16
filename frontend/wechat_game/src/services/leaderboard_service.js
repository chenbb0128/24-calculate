const BOARD_FRIENDS = 'friends';
const BOARD_GLOBAL = 'global';
const MODE_CAMPAIGN = 'campaign';
const MODE_DAILY = 'daily';
const MODE_ENDLESS = 'endless';
const MODE_FRIEND = 'friend';
const MODES = [MODE_CAMPAIGN, MODE_DAILY, MODE_ENDLESS, MODE_FRIEND];

function modeIds() { return MODES.slice(); }
function modeName(modeId) { return ({ campaign: '闯关模式', daily: '每日挑战', endless: '无尽模式', friend: '好友对战' })[modeId] || '排行榜'; }
function boardName(boardId) { return boardId === BOARD_FRIENDS ? '微信好友榜' : '游戏总榜'; }
function safeScore(value, max = 9999999) {
  const number = Number(value);
  return Number.isFinite(number) ? Math.max(0, Math.min(max, Math.floor(number))) : 0;
}
function ensureProgress(progress) { if (!progress || typeof progress !== 'object') return; if (!progress.leaderboards || typeof progress.leaderboards !== 'object') progress.leaderboards = {}; MODES.forEach((mode) => { if (!progress.leaderboards[mode] || typeof progress.leaderboards[mode] !== 'object') progress.leaderboards[mode] = { best_score: 0, last_score: 0, best_detail: {}, last_detail: {}, last_submitted_at: 0 }; const record = progress.leaderboards[mode]; record.best_score = safeScore(record.best_score); record.last_score = safeScore(record.last_score); }); }
function campaignScore(progress) { return Object.values((progress && progress.levels) || {}).reduce((sum, record) => sum + safeScore(record && record.best_score, 100), 0); }
function playerScore(progress, modeId) { ensureProgress(progress); if (!progress || !progress.leaderboards) return 0; if (modeId === MODE_CAMPAIGN) return campaignScore(progress); const record = progress.leaderboards[modeId] || {}; const best = safeScore(record.best_score); if (modeId === MODE_ENDLESS) return Math.max(best, safeScore(progress.endless && progress.endless.best_score)); if (modeId === MODE_DAILY) return Math.max(best, safeScore(progress.daily && progress.daily.best_score)); if (modeId === MODE_FRIEND) return Math.max(best, safeScore(progress.friend_matches && progress.friend_matches.best_score)); return best; }
function submitScore(progress, modeId, score, detail = {}) { ensureProgress(progress); if (!progress || !MODES.includes(modeId)) return { accepted: false, new_record: false, best_score: 0 }; const record = progress.leaderboards[modeId]; const normalizedScore = safeScore(score); const accepted = normalizedScore > 0; const newRecord = accepted && normalizedScore > safeScore(record.best_score); if (accepted) { record.last_score = normalizedScore; record.last_detail = JSON.parse(JSON.stringify(detail)); record.last_submitted_at = Date.now(); if (newRecord) { record.best_score = normalizedScore; record.best_detail = JSON.parse(JSON.stringify(detail)); } } return { accepted, new_record: newRecord, best_score: safeScore(record.best_score), mode_id: modeId }; }
function getRemoteEntries(remoteData, myUserID) {
  if (!remoteData || !Array.isArray(remoteData.entries)) return null;
  const currentUserID = Number(remoteData.my_user_id || myUserID || 0);
  return remoteData.entries.map((entry, index) => {
    const rawUserID = Number(entry && (entry.user_id ?? entry.userId) || 0);
    const userID = Number.isFinite(rawUserID) ? rawUserID : 0;
    const isPlayer = Boolean(entry && (entry.is_me || (currentUserID > 0 && userID === currentUserID)));
    return {
      name: String(entry && entry.nickname || '') || `玩家${userID || ''}`,
      score: safeScore(entry && entry.score),
      is_player: isPlayer,
      subtitle: isPlayer ? '当前玩家' : '挑战者',
      rank: Math.max(1, safeScore(entry && entry.rank, 999999) || index + 1),
      user_id: userID,
      avatar: String(entry && entry.avatar || ''),
    };
  });
}

function remotePersonalSummary(remoteData) {
  if (!remoteData || typeof remoteData !== 'object') return null;
  const raw = remoteData.me || remoteData.my_entry || remoteData.myEntry || null;
  const hasRank = remoteData.my_rank !== undefined || remoteData.myRank !== undefined;
  const hasScore = remoteData.my_score !== undefined || remoteData.myScore !== undefined;
  if (!raw && !hasRank && !hasScore) return null;
  const source = raw && typeof raw === 'object' ? raw : {};
  const rank = safeScore(source.rank ?? remoteData.my_rank ?? remoteData.myRank, 999999);
  const score = safeScore(source.score ?? remoteData.my_score ?? remoteData.myScore);
  return { rank: rank > 0 ? rank : 0, score, name: String(source.nickname || source.name || '我') };
}

function personalSummary(progress, modeId, remoteData = null, myUserID = 0) {
  const remote = remotePersonalSummary(remoteData);
  const entries = getRemoteEntries(remoteData, myUserID) || [];
  const mine = entries.find((entry) => entry.is_player);
  return {
    score: remote ? remote.score : mine ? mine.score : playerScore(progress, modeId),
    rank: remote && remote.rank > 0 ? remote.rank : mine ? mine.rank : 0,
    source: remote || mine ? 'server' : 'local',
    name: remote && remote.name ? remote.name : mine && mine.name ? mine.name : '我的成绩',
  };
}

function getEntries(progress, boardId, modeId, remoteData = null, myUserID = 0) {
  ensureProgress(progress);
  if (remoteData && (modeId === MODE_CAMPAIGN || modeId === MODE_DAILY || modeId === MODE_ENDLESS || modeId === MODE_FRIEND)) {
    const remoteEntries = getRemoteEntries(remoteData, myUserID);
    if (remoteEntries) return remoteEntries;
  }
  // 没有服务端数据时保持空榜，禁止使用写死的演示昵称和分数。
  return [];
}

module.exports = { BOARD_FRIENDS, BOARD_GLOBAL, MODE_CAMPAIGN, MODE_DAILY, MODE_ENDLESS, MODE_FRIEND, modeIds, modeName, boardName, ensureProgress, submitScore, campaignScore, playerScore, getEntries, getRemoteEntries, remotePersonalSummary, personalSummary, safeScore };
