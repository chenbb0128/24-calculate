// 正式后端 HTTPS 地址；本地开发时可临时切换为 localhost/局域网地址。
const API_BASE_URL = 'https://calc-api.pdurl.cn';
// 正式环境使用 HTTPS 时不需要开启本地后端兜底。
const ALLOW_LOCAL_BACKEND = false;
const REQUEST_TIMEOUTS = {
  default: 3500,
  login: 5000,
  bootstrap: 4000,
  runStart: 3000,
  runSubmit: 5000,
  room: 5000,
  progress: 2500,
  leaderboard: 3500,
};
const AUTH_STORAGE_KEY = 'twenty_four_auth';
const DEV_LOGIN_SLOT_KEY = 'twenty_four_dev_login_slot';
let refreshPromise = null;

function isConfigured() {
  const url = String(API_BASE_URL || '').trim();
  if (!url) return false;
  if (/^https:\/\//i.test(url)) return true;
  return ALLOW_LOCAL_BACKEND && /^https?:\/\/(localhost|127\.0\.0\.1|192\.168\.31\.132)(:\d+)?$/i.test(url);
}

function hasWx() {
  return typeof wx !== 'undefined';
}

function readAuth() {
  if (!hasWx() || !wx.getStorageSync) return null;
  try {
    const value = wx.getStorageSync(AUTH_STORAGE_KEY);
    return value && typeof value === 'object' ? value : null;
  } catch (error) {
    return null;
  }
}

function writeAuth(data) {
  const auth = {
    access_token: String(data && data.access_token || ''),
    refresh_token: String(data && data.refresh_token || ''),
    token_type: String(data && data.token_type || 'Bearer'),
    expires_in: Number(data && data.expires_in || 0),
    saved_at: Date.now(),
  };
  if (!auth.access_token || !auth.refresh_token) throw new Error('登录令牌为空');
  if (hasWx() && wx.setStorageSync) wx.setStorageSync(AUTH_STORAGE_KEY, auth);
  return auth;
}

function clearAuth() {
  try {
    if (hasWx() && wx.removeStorageSync) wx.removeStorageSync(AUTH_STORAGE_KEY);
  } catch (error) { /* 本地缓存失败不影响游戏运行 */ }
}

function makeError(message, statusCode, code) {
  const error = new Error(String(message || '请求失败'));
  error.statusCode = Number(statusCode || 0);
  error.code = Number.isFinite(Number(code)) ? Number(code) : 0;
  error.apiCode = String(code || '');
  return error;
}

function request(path, options = {}) {
  if (!isConfigured()) {
    return Promise.reject(makeError('后端地址尚未配置，当前使用本地模式', 0, 0));
  }
  if (!hasWx() || typeof wx.request !== 'function') {
    return Promise.reject(makeError('当前环境不支持网络请求', 0, 0));
  }
  return new Promise((resolve, reject) => {
    const header = Object.assign({ 'content-type': 'application/json' }, options.header || {});
    const timeout = Math.max(500, Number(options.timeout || REQUEST_TIMEOUTS.default));
    let settled = false;
    let requestTask = null;
    let timeoutTimer = null;
    const settle = (handler, value) => {
      if (settled) return;
      settled = true;
      if (timeoutTimer) clearTimeout(timeoutTimer);
      handler(value);
    };
    const onTimeout = () => {
      if (settled) return;
      try { if (requestTask && requestTask.abort) requestTask.abort(); } catch (abortError) { /* 超时清理失败不影响错误提示 */ }
      settle(reject, makeError('网络请求超时，请重试', 0, 'REQUEST_TIMEOUT'));
    };
    try {
      requestTask = wx.request({
      url: `${API_BASE_URL}${path}`,
      method: options.method || 'GET',
      data: options.data,
      header,
      timeout,
      success(response) {
        const body = response && response.data && typeof response.data === 'object'
          ? response.data : {};
        if (response.statusCode >= 200 && response.statusCode < 300 && Number(body.code) === 0) {
          settle(resolve, body.data);
          return;
        }
        settle(reject, makeError(body.message || '请求失败', response.statusCode, body.code));
      },
      fail(error) {
        settle(reject, makeError(error && error.errMsg || '网络请求失败', 0, 'NETWORK_ERROR'));
      },
      });
      if (!settled) timeoutTimer = setTimeout(onTimeout, timeout + 250);
    } catch (error) {
      settle(reject, makeError(error && error.message || '网络请求失败', 0, 'REQUEST_CREATE_FAILED'));
    }
  });
}

function loginCode() {
  if (!hasWx() || typeof wx.login !== 'function') {
    return Promise.reject(makeError('当前环境不支持微信登录', 0, 0));
  }
  return new Promise((resolve, reject) => {
    wx.login({
      success(response) {
        const code = String(response && response.code || '').trim();
        if (!code) {
          reject(makeError('微信未返回登录凭证', 0, 0));
          return;
        }
        resolve(code);
      },
      fail(error) {
        reject(makeError(error && error.errMsg || '微信登录调用失败', 0, 0));
      },
    });
  });
}

function login(profile = {}) {
  return loginCode().then((code) => request('/api/v1/auth/wechat-login', {
    method: 'POST',
    timeout: REQUEST_TIMEOUTS.login,
    data: {
      code,
      nickname: String(profile.nickname || ''),
      avatar: String(profile.avatar || ''),
    },
  })).then((data) => {
    writeAuth(data);
    return data;
  });
}

function devLogin(slot = 1) {
  return request('/api/v1/auth/dev-login', {
    method: 'POST',
    data: { slot: Math.max(1, Math.min(9, Math.floor(Number(slot) || 1))) },
  }).then((data) => {
    writeAuth(data);
    return data;
  });
}

function readDevLoginSlot() {
  if (!hasWx() || !wx.getStorageSync) return 0;
  try {
    const value = Math.floor(Number(wx.getStorageSync(DEV_LOGIN_SLOT_KEY) || 0));
    return value >= 1 && value <= 9 ? value : 0;
  } catch (error) {
    return 0;
  }
}

function canUseDevLogin() {
  const url = String(API_BASE_URL || '').trim();
  // 开发登录接口只在本地 development 后端注册。生产 HTTPS 地址即使
  // 本地残留了 dev slot，也必须走真实 wx.login，避免收到 404。
  return ALLOW_LOCAL_BACKEND && /^https?:\/\/(localhost|127\.0\.0\.1|192\.168\.31\.132)(:\d+)?$/i.test(url);
}

function refresh() {
  if (refreshPromise) return refreshPromise;
  const auth = readAuth();
  if (!auth || !auth.refresh_token) return Promise.reject(makeError('没有可用的刷新令牌', 401, 20001));
  refreshPromise = request('/api/v1/auth/refresh', {
    method: 'POST',
    timeout: REQUEST_TIMEOUTS.login,
    data: { refresh_token: auth.refresh_token },
  }).then((data) => {
    writeAuth(data);
    return data;
  }).catch((error) => {
    clearAuth();
    throw error;
  }).then((data) => {
    refreshPromise = null;
    return data;
  }, (error) => {
    refreshPromise = null;
    throw error;
  });
  return refreshPromise;
}

function authenticatedRequest(path, options = {}, retried = false) {
  const auth = readAuth();
  if (!auth || !auth.access_token) return Promise.reject(makeError('用户尚未登录', 401, 20001));
  const header = Object.assign({}, options.header || {}, {
    Authorization: `${auth.token_type || 'Bearer'} ${auth.access_token}`,
  });
  return request(path, Object.assign({}, options, { header })).catch((error) => {
    if (error.statusCode === 401 && !retried) {
      return refresh().then(() => authenticatedRequest(path, options, true));
    }
    throw error;
  });
}

function ensureLogin(profile = {}) {
  if (!isConfigured()) return Promise.reject(makeError('后端地址尚未配置，当前使用本地模式', 0, 0));
  const devSlot = readDevLoginSlot();
  if (devSlot > 0 && canUseDevLogin()) return devLogin(devSlot);
  const auth = readAuth();
  if (!auth || !auth.access_token) return login(profile);
  return authenticatedRequest('/api/v1/users/me').catch(() => login(profile));
}

function me() {
  return authenticatedRequest('/api/v1/users/me');
}

function bootstrap() {
  return authenticatedRequest('/api/v1/player/bootstrap', { timeout: REQUEST_TIMEOUTS.bootstrap });
}

function completeLevel(levelId, data = {}) {
  const safeLevelID = encodeURIComponent(String(levelId));
  return authenticatedRequest(`/api/v1/player/levels/${safeLevelID}/complete`, {
    method: 'POST',
    data: {
      idempotency_key: String(data.idempotency_key || ''),
      score: Number(data.score || 0),
      stars: Number(data.stars || 0),
    },
  });
}

function completeDaily(data = {}) {
  return authenticatedRequest('/api/v1/player/daily/complete', {
    method: 'POST',
    data: {
      idempotency_key: String(data.idempotency_key || ''),
      score: Number(data.score || 0),
    },
  });
}

function purchaseSkin(skinID) {
  const safeSkinID = encodeURIComponent(String(skinID || ''));
  return authenticatedRequest(`/api/v1/player/skins/${safeSkinID}/purchase`, { method: 'POST', data: {} });
}

function equipSkin(skinID) {
  const safeSkinID = encodeURIComponent(String(skinID || ''));
  return authenticatedRequest(`/api/v1/player/skins/${safeSkinID}/equip`, { method: 'POST', data: {} });
}

function purchaseCosmetic(cosmeticID) {
  const safeCosmeticID = encodeURIComponent(String(cosmeticID || ''));
  return authenticatedRequest(`/api/v1/player/cosmetics/${safeCosmeticID}/purchase`, { method: 'POST', data: {} });
}

function equipCosmetic(cosmeticID) {
  const safeCosmeticID = encodeURIComponent(String(cosmeticID || ''));
  return authenticatedRequest(`/api/v1/player/cosmetics/${safeCosmeticID}/equip`, { method: 'POST', data: {} });
}

function updatePreferences(data = {}) {
  return authenticatedRequest('/api/v1/player/preferences', { method: 'PATCH', data });
}

function getLeaderboard(mode, scope = 'global') {
  const safeMode = encodeURIComponent(String(mode || ''));
  const safeScope = encodeURIComponent(String(scope || 'global'));
  return authenticatedRequest(`/api/v1/player/leaderboards/${safeMode}?scope=${safeScope}`, { timeout: REQUEST_TIMEOUTS.leaderboard });
}

function submitLeaderboardScore(mode, data = {}) {
  const safeMode = encodeURIComponent(String(mode || ''));
  return authenticatedRequest(`/api/v1/player/leaderboards/${safeMode}/submit`, {
    method: 'POST',
    timeout: REQUEST_TIMEOUTS.runSubmit,
    data: {
      idempotency_key: String(data.idempotency_key || ''),
      score: Number(data.score || 0),
      questions: Number(data.questions || 0),
      elapsed_ms: Number(data.elapsed_ms || 0),
      room_id: String(data.room_id || ''),
      outcome: String(data.outcome || ''),
      metadata: data.metadata && typeof data.metadata === 'object' ? data.metadata : {},
    },
  });
}

function createEndlessRun() {
  return authenticatedRequest('/api/v1/player/endless/runs', {
    method: 'POST',
    timeout: REQUEST_TIMEOUTS.runStart,
    data: {},
  });
}

function submitEndlessRun(runID, submission = {}) {
  const safeRunID = encodeURIComponent(String(runID || ''));
  return authenticatedRequest(`/api/v1/player/endless/runs/${safeRunID}/submit`, {
    method: 'POST',
    timeout: REQUEST_TIMEOUTS.runSubmit,
    data: submission && typeof submission === 'object' ? submission : {},
  });
}

function createCampaignRun(levelID) {
  return authenticatedRequest('/api/v1/player/campaign/runs', {
    method: 'POST',
    timeout: REQUEST_TIMEOUTS.runStart,
    data: { level_id: Math.max(0, Math.floor(Number(levelID) || 0)) },
  });
}

function submitCampaignRun(runID, submission = {}) {
  const safeRunID = encodeURIComponent(String(runID || ''));
  return authenticatedRequest(`/api/v1/player/campaign/runs/${safeRunID}/submit`, {
    method: 'POST',
    timeout: REQUEST_TIMEOUTS.runSubmit,
    data: submission && typeof submission === 'object' ? submission : {},
  });
}

function createDailyRun() {
  return authenticatedRequest('/api/v1/player/daily/runs', {
    method: 'POST',
    timeout: REQUEST_TIMEOUTS.runStart,
    data: {},
  });
}

function submitDailyRun(runID, submission = {}) {
  const safeRunID = encodeURIComponent(String(runID || ''));
  return authenticatedRequest(`/api/v1/player/daily/runs/${safeRunID}/submit`, {
    method: 'POST',
    timeout: REQUEST_TIMEOUTS.runSubmit,
    data: submission && typeof submission === 'object' ? submission : {},
  });
}

function createFriendRoom() {
  return authenticatedRequest('/api/v1/player/friend/rooms', {
    method: 'POST',
    timeout: REQUEST_TIMEOUTS.room,
    data: {},
  });
}

function joinFriendRoom(roomCode) {
  const safeRoomCode = encodeURIComponent(String(roomCode || ''));
  return authenticatedRequest(`/api/v1/player/friend/rooms/${safeRoomCode}/join`, {
    method: 'POST',
    timeout: REQUEST_TIMEOUTS.room,
    data: {},
  });
}

function getFriendRoom(roomCode) {
  const safeRoomCode = encodeURIComponent(String(roomCode || ''));
  return authenticatedRequest(`/api/v1/player/friend/rooms/${safeRoomCode}`, { timeout: REQUEST_TIMEOUTS.room });
}

function leaveFriendRoom(roomCode) {
  const safeRoomCode = encodeURIComponent(String(roomCode || ''));
  return authenticatedRequest(`/api/v1/player/friend/rooms/${safeRoomCode}`, { method: 'DELETE', timeout: REQUEST_TIMEOUTS.room });
}

function readyFriendRoom(roomCode, ready = true) {
  const safeRoomCode = encodeURIComponent(String(roomCode || ''));
  return authenticatedRequest(`/api/v1/player/friend/rooms/${safeRoomCode}/ready`, {
    method: 'POST',
    timeout: REQUEST_TIMEOUTS.room,
    data: { ready: Boolean(ready) },
  });
}

function startFriendRoom(roomCode) {
  const safeRoomCode = encodeURIComponent(String(roomCode || ''));
  return authenticatedRequest(`/api/v1/player/friend/rooms/${safeRoomCode}/start`, {
    method: 'POST',
    timeout: REQUEST_TIMEOUTS.room,
    data: {},
  });
}

function updateFriendMatchProgress(roomCode, data = {}) {
  const safeRoomCode = encodeURIComponent(String(roomCode || ''));
  return authenticatedRequest(`/api/v1/player/friend/rooms/${safeRoomCode}/match/progress`, {
    method: 'POST',
    timeout: REQUEST_TIMEOUTS.progress,
    data: {
      question_index: Number(data.question_index || 0),
      solved: Number(data.solved || 0),
      score: Number(data.score || 0),
      elapsed_ms: Number(data.elapsed_ms || 0),
      finished: Boolean(data.finished),
      match_id: String(data.match_id || ''),
      question_hash: String(data.question_hash || ''),
      event_id: String(data.event_id || ''),
      attempt: data.attempt && typeof data.attempt === 'object' ? data.attempt : null,
    },
  });
}

function getFriendMatchProgress(roomCode) {
  const safeRoomCode = encodeURIComponent(String(roomCode || ''));
  return authenticatedRequest(`/api/v1/player/friend/rooms/${safeRoomCode}/match/progress`, { timeout: REQUEST_TIMEOUTS.progress });
}

function submitFriendMatch(roomCode, submission = {}) {
  const safeRoomCode = encodeURIComponent(String(roomCode || ''));
  return authenticatedRequest(`/api/v1/player/friend/rooms/${safeRoomCode}/match/submit`, {
    method: 'POST',
    timeout: REQUEST_TIMEOUTS.runSubmit,
    data: submission && typeof submission === 'object' ? submission : {},
  });
}

function joinMatchmaking(data = {}) {
  return authenticatedRequest('/api/v1/player/matchmaking/join', {
    method: 'POST',
    timeout: REQUEST_TIMEOUTS.room,
    data: {
      mode: 'friend',
      rules_version: 1,
      client_ticket: String(data.client_ticket || ''),
      region: String(data.region || ''),
    },
  });
}

function getMatchmakingStatus(ticketId) {
  const ticket = encodeURIComponent(String(ticketId || ''));
  return authenticatedRequest(`/api/v1/player/matchmaking/status?ticket_id=${ticket}`, { timeout: REQUEST_TIMEOUTS.progress });
}

function cancelMatchmaking(ticketId) {
  const ticket = encodeURIComponent(String(ticketId || ''));
  return authenticatedRequest('/api/v1/player/matchmaking/cancel', {
    method: 'POST',
    timeout: REQUEST_TIMEOUTS.room,
    data: { ticket_id: String(ticketId || '') },
  });
}

module.exports = {
  API_BASE_URL,
  REQUEST_TIMEOUTS,
  isConfigured,
  AUTH_STORAGE_KEY,
  DEV_LOGIN_SLOT_KEY,
  readAuth,
  clearAuth,
  request,
  login,
  devLogin,
  readDevLoginSlot,
  refresh,
  ensureLogin,
  me,
  bootstrap,
  completeLevel,
  completeDaily,
  purchaseSkin,
  equipSkin,
  purchaseCosmetic,
  equipCosmetic,
  updatePreferences,
  getLeaderboard,
  submitLeaderboardScore,
  createEndlessRun,
  submitEndlessRun,
  createCampaignRun,
  submitCampaignRun,
  createDailyRun,
  submitDailyRun,
  createFriendRoom,
  joinFriendRoom,
  getFriendRoom,
  leaveFriendRoom,
  readyFriendRoom,
  startFriendRoom,
  updateFriendMatchProgress,
  getFriendMatchProgress,
  submitFriendMatch,
  joinMatchmaking,
  getMatchmakingStatus,
  cancelMatchmaking,
};
