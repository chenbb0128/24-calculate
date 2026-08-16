const {
  KEY,
  BACKUP_KEY,
  ERROR_LOG_KEY,
  COIN_CAP,
  STORAGE_VERSION,
  LEGACY_KEYS,
} = require('../config/storage_keys.js');

let lastLoadInfo = {
  recovered: false,
  primaryValid: false,
  backupValid: false,
  migratedLegacy: false,
};

function clone(value) {
  if (value === undefined || value === null) return value;
  try {
    return JSON.parse(JSON.stringify(value));
  } catch (error) {
    return null;
  }
}

function asRecord(data) {
  if (data && typeof data === 'object' && !Array.isArray(data)) return data;
  if (typeof data === 'string') {
    try {
      const parsed = JSON.parse(data);
      return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {};
    } catch (error) {
      return {};
    }
  }
  return {};
}

function parseStoredRecord(data) {
  const record = asRecord(data);
  return Object.keys(record).length ? record : null;
}

function readWxValue(key) {
  try {
    if (typeof wx !== 'undefined' && wx.getStorageSync) return wx.getStorageSync(key);
  } catch (error) {
    return null;
  }
  return null;
}

function writeWxValue(key, value) {
  try {
    if (typeof wx !== 'undefined' && wx.setStorageSync) {
      wx.setStorageSync(key, value);
      return true;
    }
  } catch (error) {
    return false;
  }
  return false;
}

function defaults() {
  return {
    version: STORAGE_VERSION,
    unlocked_level: 0,
    last_level: 0,
    tutorial_seen: false,
    levels: {},
    level_rewards: {},
    milestones: { first_clear: false, three_star: false, chapters: {} },
    login: { last_date: '', streak: 0, last_reward: 0 },
    player_stats: {
      total_solved: 0,
      total_score: 0,
      fastest_ms: 0,
      best_combo: 0,
      best_level: 0,
      best_chapter: 0,
      operator_counts: {},
      mode_questions: {},
      last_solve: {},
    },
    coins: 0,
    owned_skins: ['classic'],
    equipped_skin: 'classic',
    owned_cosmetics: ['card_classic', 'operator_classic', 'result_classic'],
    equipped_cosmetics: { card: 'card_classic', operator: 'operator_classic', result: 'result_classic' },
    daily: { last_date: '', streak: 0, best_score: 0, completed: {}, reward_claimed: {} },
    endless: {
      best_score: 0,
      best_questions: 0,
      best_combo: 0,
      best_stage: 0,
      last_score: 0,
      reward_date: '',
      reward_coins_today: 0,
      reward_run_id: '',
      rewarded_questions: {},
    },
    tasks: { date: '', values: {}, claimed: {} },
    audio: { music_enabled: true, sfx_enabled: true, music_track: 0, music_volume: 0.42, sfx_volume: 0.72 },
    friend_matches: { date: '', played: 0, wins: 0, best_score: 0, best_time_ms: 0, reward_date: '', reward_count: 0 },
    ads: { date: '', rewarded_used: 0 },
    weekly_tasks: { week: '', values: {}, claimed: {} },
    leaderboards: {},
    achievements: { unlocked: {}, claimed: {} },
  };
}

function todayKey() {
  const now = new Date();
  try {
    const parts = new Intl.DateTimeFormat('zh-CN', {
      timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit',
    }).formatToParts(now);
    const map = {};
    parts.forEach((part) => { map[part.type] = part.value; });
    return `${map.year}-${map.month}-${map.day}`;
  } catch (error) {
    return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`;
  }
}

function todaySeed() {
  return Number(todayKey().replace(/-/g, ''));
}

function normalize(data) {
  const base = defaults();
  const source = clone(asRecord(data));
  const result = Object.assign(base, source && typeof source === 'object' ? source : {});
  ['daily', 'endless', 'tasks', 'audio', 'friend_matches', 'ads', 'milestones', 'login', 'player_stats', 'weekly_tasks', 'achievements'].forEach((key) => {
    result[key] = Object.assign(base[key], result[key] && typeof result[key] === 'object' ? result[key] : {});
  });
  result.version = Math.max(Number(result.version || 0), STORAGE_VERSION);
  result.level_rewards = result.level_rewards && typeof result.level_rewards === 'object' ? result.level_rewards : {};
  result.leaderboards = result.leaderboards && typeof result.leaderboards === 'object' ? result.leaderboards : {};
  result.player_stats.operator_counts = result.player_stats.operator_counts && typeof result.player_stats.operator_counts === 'object' ? result.player_stats.operator_counts : {};
  result.player_stats.mode_questions = result.player_stats.mode_questions && typeof result.player_stats.mode_questions === 'object' ? result.player_stats.mode_questions : {};
  result.levels = result.levels && typeof result.levels === 'object' ? result.levels : {};
  result.daily.completed = result.daily.completed && typeof result.daily.completed === 'object' ? result.daily.completed : {};
  result.daily.reward_claimed = result.daily.reward_claimed && typeof result.daily.reward_claimed === 'object' ? result.daily.reward_claimed : {};
  result.coins = Math.max(0, Math.min(COIN_CAP, Math.floor(Number(result.coins) || 0)));
  result.unlocked_level = Math.max(0, Math.min(200, Math.floor(Number(result.unlocked_level) || 0)));
  result.last_level = Math.max(0, Math.min(199, Math.floor(Number(result.last_level) || 0)));
  Object.keys(result.levels).forEach((levelKey) => {
    const level = result.levels[levelKey];
    if (!level || typeof level !== 'object') { delete result.levels[levelKey]; return; }
    level.stars = Math.max(0, Math.min(3, Math.floor(Number(level.stars) || 0)));
    // 闯关分数统一为 0～100；旧版本可能保存过几百甚至上千分，读取时安全归一化为满分 100。
    level.best_score = Math.max(0, Math.min(100, Math.floor(Number(level.best_score) || 0)));
    level.completed = Boolean(level.completed || level.stars > 0 || level.best_score > 0);
    level.best_score = Math.max(0, Math.min(100, Math.floor(Number(level.best_score) || 0)));
  });
  result.endless.best_score = Math.max(0, Math.min(9999999, Math.floor(Number(result.endless.best_score) || 0)));
  result.endless.best_questions = Math.max(0, Math.floor(Number(result.endless.best_questions) || 0));
  result.endless.best_combo = Math.max(0, Math.floor(Number(result.endless.best_combo) || 0));
  result.endless.best_stage = Math.max(0, Math.floor(Number(result.endless.best_stage) || 0));
  result.endless.reward_coins_today = Math.max(0, Math.min(60, Math.floor(Number(result.endless.reward_coins_today) || 0)));
  result.endless.rewarded_questions = result.endless.rewarded_questions && typeof result.endless.rewarded_questions === 'object' ? result.endless.rewarded_questions : {};
  result.friend_matches.played = Math.max(0, Math.floor(Number(result.friend_matches.played) || 0));
  result.friend_matches.wins = Math.max(0, Math.min(result.friend_matches.played, Math.floor(Number(result.friend_matches.wins) || 0)));
  result.friend_matches.reward_count = Math.max(0, Math.min(3, Math.floor(Number(result.friend_matches.reward_count) || 0)));
  const musicVolume = Number(result.audio.music_volume);
  const sfxVolume = Number(result.audio.sfx_volume);
  result.audio.music_volume = Math.max(0, Math.min(1, Number.isFinite(musicVolume) ? musicVolume : 0.42));
  result.audio.sfx_volume = Math.max(0, Math.min(1, Number.isFinite(sfxVolume) ? sfxVolume : 0.72));
  result.ads.rewarded_used = Math.max(0, Math.min(3, Math.floor(Number(result.ads.rewarded_used) || 0)));
  if (!Array.isArray(result.owned_skins) || result.owned_skins.length === 0) result.owned_skins = ['classic'];
  if (!result.owned_skins.includes(result.equipped_skin)) result.equipped_skin = 'classic';
  const cosmeticDefaults = { card: 'card_classic', operator: 'operator_classic', result: 'result_classic' };
  const cosmeticIds = new Set([
    'card_classic', 'card_neon', 'card_candy',
    'operator_classic', 'operator_bubble', 'operator_prism',
    'result_classic', 'result_burst', 'result_fireworks',
  ]);
  if (!Array.isArray(result.owned_cosmetics) || result.owned_cosmetics.length === 0) result.owned_cosmetics = Object.values(cosmeticDefaults);
  result.owned_cosmetics = Array.from(new Set(result.owned_cosmetics.map((id) => String(id)).filter((id) => cosmeticIds.has(id))));
  if (!result.equipped_cosmetics || typeof result.equipped_cosmetics !== 'object') result.equipped_cosmetics = {};
  Object.keys(cosmeticDefaults).forEach((category) => {
    const selected = result.equipped_cosmetics && result.equipped_cosmetics[category];
    result.equipped_cosmetics[category] = cosmeticIds.has(String(selected)) && String(selected).startsWith(`${category}_`)
      && result.owned_cosmetics.includes(String(selected)) ? String(selected) : cosmeticDefaults[category];
  });
  const date = todayKey();
  if (result.tasks.date !== date) result.tasks = { date, values: {}, claimed: {} };
  if (result.ads.date !== date) result.ads = { date, rewarded_used: 0 };
  return result;
}

function load() {
  const primaryRaw = readWxValue(KEY);
  const backupRaw = readWxValue(BACKUP_KEY);
  const primary = parseStoredRecord(primaryRaw);
  const backup = parseStoredRecord(backupRaw);
  const recovered = !primary && Boolean(backup);
  const data = primary || backup || {};
  lastLoadInfo = {
    recovered,
    primaryValid: Boolean(primary),
    backupValid: Boolean(backup),
    migratedLegacy: false,
  };
  const result = normalize(data);
  if (recovered) {
    writeWxValue(KEY, result);
    writeWxValue(BACKUP_KEY, result);
    appendErrorLog('storage-recovery', '主存档损坏，已从备用存档恢复', { version: result.version });
  }
  // 兼容迁移前曾使用的三个独立键；新版本仍会同步写回，避免丢失玩家进度。
  try {
    if (typeof wx !== 'undefined' && wx.getStorageSync) {
      const legacyCoins = Number(wx.getStorageSync(LEGACY_KEYS.coins));
      const legacyLevel = Number(wx.getStorageSync(LEGACY_KEYS.unlockedLevel));
      const legacyDailyDone = wx.getStorageSync(LEGACY_KEYS.dailyDone);
      const hasDateAwareDaily = Boolean(data && data.daily && typeof data.daily === 'object'
        && (data.daily.completed || data.daily.last_date));
      // When the primary file is corrupt, the structured backup is the last
      // known-good snapshot. Do not let stale legacy mirror keys overwrite it.
      if (!recovered && Number.isFinite(legacyCoins)) result.coins = Math.max(result.coins, legacyCoins);
      if (!recovered && Number.isFinite(legacyLevel)) result.unlocked_level = Math.max(result.unlocked_level, legacyLevel);
      // 只有完全没有日期存档的旧版本才迁移 dailyDone；新版本必须按日期判断，保证零点后自动开放。
      if (legacyDailyDone === true && !hasDateAwareDaily) {
        const migratedDate = todayKey();
        result.daily.completed[migratedDate] = true;
        result.daily.last_date = migratedDate;
        lastLoadInfo.migratedLegacy = true;
        writeWxValue(KEY, result);
      }
    }
  } catch (error) { /* 读取旧键失败不影响主存档 */ }
  return result;
}

function save(progress) {
  const normalized = normalize(progress);
  const previous = parseStoredRecord(readWxValue(KEY));
  // Keep the last known-good version. If the next write is interrupted, the
  // backup remains available for the next launch.
  if (previous) writeWxValue(BACKUP_KEY, previous);
  try {
    if (typeof wx !== 'undefined' && wx.setStorageSync) {
      wx.setStorageSync(KEY, normalized);
      if (!previous) writeWxValue(BACKUP_KEY, normalized);
      // 与旧版/外部页面约定保持兼容，后续可以平滑移除这些镜像键。
      wx.setStorageSync(LEGACY_KEYS.coins, Number(normalized.coins || 0));
      wx.setStorageSync(LEGACY_KEYS.unlockedLevel, Number(normalized.unlocked_level || 0));
      wx.setStorageSync(LEGACY_KEYS.dailyDone, isDailyCompleted(normalized));
    }
  } catch (error) {
    appendErrorLog('storage-save', error, { key: KEY, version: normalized.version });
    /* 本地缓存失败时仍保留当前会话 */
  }
  return normalized;
}

function restoreBackup() {
  const backup = parseStoredRecord(readWxValue(BACKUP_KEY));
  if (!backup) return null;
  const restored = normalize(backup);
  if (!writeWxValue(KEY, restored)) return null;
  writeWxValue(LEGACY_KEYS.coins, Number(restored.coins || 0));
  writeWxValue(LEGACY_KEYS.unlockedLevel, Number(restored.unlocked_level || 0));
  writeWxValue(LEGACY_KEYS.dailyDone, isDailyCompleted(restored));
  return restored;
}

function getLastLoadInfo() {
  return clone(lastLoadInfo) || {
    recovered: false, primaryValid: false, backupValid: false, migratedLegacy: false,
  };
}

function getDiagnostics() {
  const primary = parseStoredRecord(readWxValue(KEY));
  const backup = parseStoredRecord(readWxValue(BACKUP_KEY));
  const logs = getErrorLogs();
  return {
    key: KEY,
    backupKey: BACKUP_KEY,
    primaryValid: Boolean(primary),
    backupValid: Boolean(backup),
    primaryVersion: primary ? Number(primary.version || 0) : 0,
    backupVersion: backup ? Number(backup.version || 0) : 0,
    errorLogCount: logs.length,
    lastError: logs.length ? logs[logs.length - 1] : null,
  };
}

function getErrorLogs() {
  const raw = readWxValue(ERROR_LOG_KEY);
  if (Array.isArray(raw)) return raw.slice(-30);
  if (typeof raw === 'string') {
    try {
      const parsed = JSON.parse(raw);
      return Array.isArray(parsed) ? parsed.slice(-30) : [];
    } catch (error) {
      return [];
    }
  }
  return [];
}

function appendErrorLog(stage, error, context = {}) {
  const message = String(error && (error.stack || error.message) || error || 'unknown error').slice(0, 500);
  const logs = getErrorLogs();
  const last = logs[logs.length - 1];
  const now = Date.now();
  if (last && last.stage === String(stage) && last.message === message && now - Number(last.time || 0) < 3000) return last;
  const entry = {
    time: now,
    stage: String(stage || 'runtime'),
    message,
    mode: context.mode ? String(context.mode) : '',
    screen: context.screen ? String(context.screen) : '',
  };
  logs.push(entry);
  writeWxValue(ERROR_LOG_KEY, logs.slice(-30));
  return entry;
}

function clearErrorLogs() {
  return writeWxValue(ERROR_LOG_KEY, []);
}

function mergeServerProgress(progress, remote, options = {}) {
  const local = normalize(progress);
  if (!remote || typeof remote !== 'object') return local;

  // 登录 bootstrap 和服务端结算返回的余额是权威值；普通的部分同步仍保留
  // 本地较大值，兼容离线体验和旧版本没有 pending_mutations 的存档。
  const remoteCoins = Number(remote.coins);
  if (Number.isFinite(remoteCoins) && options.authoritative === true) {
    local.coins = Math.max(0, Math.min(COIN_CAP, Math.floor(remoteCoins)));
  } else {
    local.coins = Math.max(local.coins, Math.floor(remoteCoins) || 0);
  }
  local.unlocked_level = Math.max(local.unlocked_level, Math.floor(Number(remote.unlocked_level) || 0));
  local.last_level = Math.max(local.last_level, Math.floor(Number(remote.last_level) || 0));

  const remoteLevels = remote.levels && typeof remote.levels === 'object' ? remote.levels : {};
  Object.keys(remoteLevels).forEach((levelKey) => {
    const remoteLevel = remoteLevels[levelKey];
    if (!remoteLevel || typeof remoteLevel !== 'object') return;
    const localLevel = local.levels[levelKey] || {};
    local.levels[levelKey] = {
      stars: Math.max(Number(localLevel.stars) || 0, Number(remoteLevel.stars) || 0),
      best_score: Math.max(Number(localLevel.best_score) || 0, Number(remoteLevel.best_score) || 0),
      completed: Boolean(localLevel.completed || remoteLevel.completed
        || Number(localLevel.stars) > 0 || Number(remoteLevel.stars) > 0
        || Number(localLevel.best_score) > 0 || Number(remoteLevel.best_score) > 0),
    };
  });

  const remoteRewards = remote.level_rewards && typeof remote.level_rewards === 'object' ? remote.level_rewards : {};
  Object.keys(remoteRewards).forEach((key) => {
    if (remoteRewards[key]) local.level_rewards[key] = true;
  });

  const remoteSkins = Array.isArray(remote.owned_skins) ? remote.owned_skins.map((id) => String(id)) : [];
  local.owned_skins = Array.from(new Set(local.owned_skins.concat(remoteSkins)));
  if (!local.owned_skins.includes(local.equipped_skin) && local.owned_skins.includes(String(remote.equipped_skin || ''))) {
    local.equipped_skin = String(remote.equipped_skin);
  }

  const remoteDaily = remote.daily && typeof remote.daily === 'object' ? remote.daily : {};
  local.daily.best_score = Math.max(local.daily.best_score, Number(remoteDaily.best_score) || 0);
  local.daily.streak = Math.max(local.daily.streak, Number(remoteDaily.streak) || 0);
  const remoteCompleted = remoteDaily.completed && typeof remoteDaily.completed === 'object' ? remoteDaily.completed : {};
  Object.keys(remoteCompleted).forEach((date) => {
    if (remoteCompleted[date]) local.daily.completed[date] = true;
  });
  const remoteClaimed = remoteDaily.reward_claimed && typeof remoteDaily.reward_claimed === 'object' ? remoteDaily.reward_claimed : {};
  Object.keys(remoteClaimed).forEach((date) => {
    if (remoteClaimed[date]) local.daily.reward_claimed[date] = true;
  });

  const remoteLogin = remote.login && typeof remote.login === 'object' ? remote.login : {};
  if (remoteLogin.last_date) local.login.last_date = String(remoteLogin.last_date);
  local.login.streak = Math.max(Number(local.login.streak) || 0, Number(remoteLogin.streak) || 0);
  local.login.last_reward = Math.max(Number(local.login.last_reward) || 0, Number(remoteLogin.last_reward) || 0);

  const remoteAchievements = remote.achievements && typeof remote.achievements === 'object' ? remote.achievements : {};
  const remoteUnlocked = remoteAchievements.unlocked && typeof remoteAchievements.unlocked === 'object' ? remoteAchievements.unlocked : {};
  Object.keys(remoteUnlocked).forEach((id) => {
    if (remoteUnlocked[id]) local.achievements.unlocked[id] = remoteUnlocked[id];
  });
  const remoteAchievementClaims = remoteAchievements.claimed && typeof remoteAchievements.claimed === 'object' ? remoteAchievements.claimed : {};
  Object.keys(remoteAchievementClaims).forEach((id) => {
    if (remoteAchievementClaims[id]) local.achievements.claimed[id] = true;
  });

  const remoteTasks = remote.tasks && typeof remote.tasks === 'object' ? remote.tasks : {};
  const remoteTaskValues = remoteTasks.values && typeof remoteTasks.values === 'object' ? remoteTasks.values : {};
  Object.keys(remoteTaskValues).forEach((id) => {
    local.tasks.values[id] = Math.max(Number(local.tasks.values[id]) || 0, Number(remoteTaskValues[id]) || 0);
  });
  const remoteTaskClaims = remoteTasks.claimed && typeof remoteTasks.claimed === 'object' ? remoteTasks.claimed : {};
  Object.keys(remoteTaskClaims).forEach((id) => {
    if (remoteTaskClaims[id]) local.tasks.claimed[id] = true;
  });

  const remoteWeekly = remote.weekly_tasks && typeof remote.weekly_tasks === 'object' ? remote.weekly_tasks : {};
  if (remoteWeekly.week && String(remoteWeekly.week) === String(local.weekly_tasks.week)) {
    const remoteWeeklyValues = remoteWeekly.values && typeof remoteWeekly.values === 'object' ? remoteWeekly.values : {};
    Object.keys(remoteWeeklyValues).forEach((id) => { local.weekly_tasks.values[id] = Math.max(Number(local.weekly_tasks.values[id]) || 0, Number(remoteWeeklyValues[id]) || 0); });
    const remoteWeeklyClaims = remoteWeekly.claimed && typeof remoteWeekly.claimed === 'object' ? remoteWeekly.claimed : {};
    Object.keys(remoteWeeklyClaims).forEach((id) => { if (remoteWeeklyClaims[id]) local.weekly_tasks.claimed[id] = true; });
  } else if (remoteWeekly.week) {
    local.weekly_tasks = clone(remoteWeekly);
  }

  const remoteEndless = remote.endless && typeof remote.endless === 'object' ? remote.endless : {};
  ['best_score', 'best_questions', 'best_combo', 'best_stage', 'last_score', 'reward_coins_today'].forEach((key) => {
    local.endless[key] = Math.max(Number(local.endless[key]) || 0, Number(remoteEndless[key]) || 0);
  });
  if (remoteEndless.reward_date) local.endless.reward_date = String(remoteEndless.reward_date);
  if (remoteEndless.reward_run_id) local.endless.reward_run_id = String(remoteEndless.reward_run_id);
  const remoteRewarded = remoteEndless.rewarded_questions && typeof remoteEndless.rewarded_questions === 'object' ? remoteEndless.rewarded_questions : {};
  Object.keys(remoteRewarded).forEach((key) => { if (remoteRewarded[key]) local.endless.rewarded_questions[key] = true; });

  const remoteFriend = remote.friend_matches && typeof remote.friend_matches === 'object' ? remote.friend_matches : {};
  ['played', 'wins', 'best_score', 'best_time_ms', 'reward_count'].forEach((key) => { local.friend_matches[key] = Math.max(Number(local.friend_matches[key]) || 0, Number(remoteFriend[key]) || 0); });
  if (remoteFriend.date) local.friend_matches.date = String(remoteFriend.date);
  if (remoteFriend.reward_date) local.friend_matches.reward_date = String(remoteFriend.reward_date);

  const remoteStats = remote.player_stats && typeof remote.player_stats === 'object' ? remote.player_stats : {};
  ['total_solved', 'total_score', 'fastest_ms', 'best_combo', 'best_level', 'best_chapter'].forEach((key) => { local.player_stats[key] = Math.max(Number(local.player_stats[key]) || 0, Number(remoteStats[key]) || 0); });
  ['operator_counts', 'mode_questions'].forEach((key) => {
    const values = remoteStats[key] && typeof remoteStats[key] === 'object' ? remoteStats[key] : {};
    Object.keys(values).forEach((id) => { local.player_stats[key][id] = Math.max(Number(local.player_stats[key][id]) || 0, Number(values[id]) || 0); });
  });
  if (remoteStats.last_solve && typeof remoteStats.last_solve === 'object') local.player_stats.last_solve = clone(remoteStats.last_solve);

  const remoteAudio = remote.audio && typeof remote.audio === 'object' ? remote.audio : {};
  local.audio = Object.assign(local.audio, remoteAudio);
  const remoteCosmetics = Array.isArray(remote.owned_cosmetics) ? remote.owned_cosmetics.map((id) => String(id)) : [];
  local.owned_cosmetics = Array.from(new Set(local.owned_cosmetics.concat(remoteCosmetics)));
  if (remote.equipped_cosmetics && typeof remote.equipped_cosmetics === 'object') local.equipped_cosmetics = Object.assign(local.equipped_cosmetics, clone(remote.equipped_cosmetics));

  return save(local);
}

function isDailyCompleted(progress, date = todayKey()) {
  return Boolean(progress.daily && progress.daily.completed && progress.daily.completed[date]);
}

function saveLevel(progress, levelIndex, stars, score) {
  const old = progress.levels[String(levelIndex)] || {};
  progress.levels[String(levelIndex)] = {
    stars: Math.max(Number(old.stars || 0), Number(stars || 0)),
    best_score: Math.max(Math.min(100, Number(old.best_score || 0)), Math.min(100, Number(score || 0))),
    completed: true,
  };
  progress.unlocked_level = Math.max(Number(progress.unlocked_level || 0), levelIndex + 1);
  progress.last_level = levelIndex;
  return save(progress);
}

function addCoins(progress, amount) {
  const gain = Math.max(0, Math.floor(Number(amount) || 0));
  progress.coins = Math.max(0, Math.min(COIN_CAP, Math.floor(Number(progress.coins) || 0) + gain));
  return gain;
}

function spendCoins(progress, amount) {
  const cost = Math.max(0, Math.floor(Number(amount) || 0));
  const balance = Math.max(0, Math.floor(Number(progress.coins) || 0));
  if (cost > balance) return false;
  progress.coins = balance - cost;
  return true;
}

function claimDailyLoginReward(progress, dateKey = todayKey()) {
  const login = progress.login || (progress.login = { last_date: '', streak: 0, last_reward: 0 });
  const date = String(dateKey || todayKey());
  if (login.last_date === date) return 0;
  const previous = new Date(`${date}T00:00:00Z`);
  previous.setUTCDate(previous.getUTCDate() - 1);
  const previousKey = previous.toISOString().slice(0, 10);
  login.streak = login.last_date === previousKey ? Math.min(7, Number(login.streak || 0) + 1) : 1;
  const reward = 5 + Math.max(0, login.streak - 1) * 2 + (login.streak === 7 ? 10 : 0);
  login.last_date = date;
  login.last_reward = reward;
  addCoins(progress, reward);
  return reward;
}

function claimEndlessReward(progress, runId, questions, dateKey = todayKey()) {
  const endless = progress.endless || (progress.endless = {});
  const safeDate = String(dateKey || todayKey());
  const safeRunId = String(runId || 'default-run');
  const questionNumber = Math.floor(Number(questions) || 0);
  if (questionNumber <= 0) return 0;
  if (endless.reward_date !== safeDate) {
    endless.reward_date = safeDate;
    endless.reward_coins_today = 0;
  }
  if (endless.reward_run_id !== safeRunId) {
    endless.reward_run_id = safeRunId;
    endless.rewarded_questions = {};
  }
  endless.rewarded_questions = endless.rewarded_questions || {};
  const dailyCap = 60;
  const remaining = Math.max(0, dailyCap - Number(endless.reward_coins_today || 0));
  if (remaining <= 0) return 0;
  const milestoneRewards = { 5: 5, 10: 8, 20: 12, 30: 15, 50: 20, 100: 30 };
  let reward = 0;
  for (let question = 1; question <= questionNumber && reward < remaining; question += 1) {
    const questionKey = String(question);
    if (endless.rewarded_questions[questionKey]) continue;
    reward += 1 + (milestoneRewards[question] || 0);
    endless.rewarded_questions[questionKey] = true;
  }
  reward = Math.min(reward, remaining);
  endless.reward_coins_today = Number(endless.reward_coins_today || 0) + reward;
  addCoins(progress, reward);
  return reward;
}

function claimFriendReward(progress, outcome, dateKey = todayKey(), dailyLimit = 3) {
  const match = progress.friend_matches || (progress.friend_matches = {});
  const safeDate = String(dateKey || todayKey());
  if (match.reward_date !== safeDate) { match.reward_date = safeDate; match.reward_count = 0; }
  if (Number(match.reward_count || 0) >= Number(dailyLimit || 3)) return 0;
  const reward = outcome === 'win' ? 15 : outcome === 'draw' ? 8 : 5;
  match.reward_count = Number(match.reward_count || 0) + 1;
  addCoins(progress, reward);
  return reward;
}

function claimLevelReward(progress, levelIndex, stars) {
  const rewards = progress.level_rewards && typeof progress.level_rewards === 'object' ? progress.level_rewards : {};
  const key = String(levelIndex);
  if (rewards[key]) return 0;
  const coins = 8 + Math.max(0, Math.min(3, Number(stars || 0))) * 3;
  rewards[key] = true;
  progress.level_rewards = rewards;
  addCoins(progress, coins);
  return coins;
}

function claimCampaignBonus(progress, levelIndex, stars, chapterIndex, chapterComplete, newLevelClear) {
  const milestones = progress.milestones || { first_clear: false, three_star: false, chapters: {} };
  const chapters = milestones.chapters && typeof milestones.chapters === 'object' ? milestones.chapters : {};
  const labels = [];
  let coins = 0;
  if (newLevelClear && !milestones.first_clear) {
    milestones.first_clear = true;
    coins += 12;
    labels.push('首次通关 +12');
  }
  if (Number(stars || 0) >= 3 && !milestones.three_star) {
    milestones.three_star = true;
    coins += 8;
    labels.push('首次三星 +8');
  }
  const chapterKey = String(chapterIndex);
  if (chapterComplete && newLevelClear && !chapters[chapterKey]) {
    chapters[chapterKey] = true;
    coins += 20;
    labels.push('章节完成 +20');
  }
  milestones.chapters = chapters;
  progress.milestones = milestones;
  addCoins(progress, coins);
  return { coins, labels };
}

module.exports = {
  KEY, BACKUP_KEY, ERROR_LOG_KEY, COIN_CAP, STORAGE_VERSION, defaults, normalize, load, save, restoreBackup,
  getLastLoadInfo, getDiagnostics, getErrorLogs, appendErrorLog, clearErrorLogs,
  mergeServerProgress, todayKey, todaySeed, isDailyCompleted, addCoins, spendCoins, claimDailyLoginReward,
  saveLevel, claimLevelReward, claimCampaignBonus, claimEndlessReward, claimFriendReward, clone,
};
