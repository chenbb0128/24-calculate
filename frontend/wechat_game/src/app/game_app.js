const puzzle = require('../core/puzzle_generator.js');
const levelCatalog = require('../core/level_catalog.js');
const dailyChallenge = require('../core/daily_challenge.js');
const friendMatch = require('../core/friend_match_service.js');
const matchData = require('../core/match_data.js');
const skinCatalog = require('../core/skin_catalog.js');
const storage = require('../services/storage.js');
const diagnostics = require('../services/diagnostics.js');
const apiClient = require('../services/api_client.js');
const platform = require('../services/platform.js');
const shareService = require('../services/share_service.js');
const achievementService = require('../services/achievement_service.js');
const leaderboardService = require('../services/leaderboard_service.js');
const playerStats = require('../services/player_stats.js');
const taskService = require('../services/task_service.js');
const audioService = require('../services/audio_service.js');
const adService = require('../services/ad_service.js');
const endlessMode = require('../core/endless_mode.js');
const campaignPuzzleData = require('../core/campaign_puzzle_data.js');
const questionServiceModule = require('../core/question_service.js');
const { COLORS, GAME_UI, UI_FONT, FONT_SCALE, HOME_TITLE } = require('../ui/theme.js');
const { clamp, safeNumber, previousDateKey, uiFont, scaleFont, resizeFont, uiSafeText } = require('./app_utils.js');
const touchGeometry = require('../input/touch_geometry.js');
const hitTest = require('../input/hit_test.js');
const screenRenderer = require('../ui/screen_renderer.js');

function makeEndlessSeed() {
  const now = Date.now() >>> 0;
  const random = Math.floor(Math.random() * 0x100000000) >>> 0;
  const day = storage.todaySeed() >>> 0;
  return (now ^ random ^ ((day * 2654435761) >>> 0)) >>> 0 || 1;
}

class GameApp {
  constructor() {
    this.canvas = wx.createCanvas();
    const info = wx.getSystemInfoSync ? wx.getSystemInfoSync() : { windowWidth: 360, windowHeight: 640, pixelRatio: 2 };
    this.dpr = info.pixelRatio || 1;
    this.viewportWidth = Number(info.windowWidth) || 360;
    this.viewportHeight = Number(info.windowHeight) || 640;
    // UI 使用统一比例缩放，首页按参考图的高瘦比例复刻，避免不同手机把圆形、字体和按钮拉伸变形。
    this.width = 750;
    this.renderScale = this.viewportWidth / this.width;
    this.visibleHeight = Math.max(1, this.viewportHeight / this.renderScale);
    this.height = Math.max(1334, Math.round(this.viewportHeight / this.renderScale));
    this.canvas.width = Math.round(this.viewportWidth * this.dpr);
    this.canvas.height = Math.round(this.viewportHeight * this.dpr);
    this.ctx = this.canvas.getContext('2d');
    const safeArea = info.safeArea || {};
    let menuButtonRect = null;
    try {
      if (wx.getMenuButtonBoundingClientRect) menuButtonRect = wx.getMenuButtonBoundingClientRect();
    } catch (error) { menuButtonRect = null; }
    this.menuButton = menuButtonRect;
    // 部分开发者工具版本没有返回 safeArea，使用状态栏和导航栏的保守兜底值。
    const fallbackTop = Math.max(28, safeNumber(info.statusBarHeight, 0));
    const fallbackBottom = 24;
    this.safeTop = Math.max(fallbackTop, safeNumber(safeArea.top, 0));
    const safeBottomEdge = safeNumber(safeArea.bottom, this.viewportHeight - fallbackBottom);
    this.safeBottom = Math.max(fallbackBottom, this.viewportHeight - safeBottomEdge);
    this.renderOffsetX = 0;
    this.renderOffsetY = 0;
    this.homeYScale = Math.min(1, this.height / 1584);
    this.applyRenderTransform();
    this.progress = storage.load();
    this.storageLoadInfo = storage.getLastLoadInfo ? storage.getLastLoadInfo() : {};
    this.backendAuth = { status: 'pending', user: null, error: null };
    if (apiClient.isConfigured && apiClient.isConfigured()) this.startBackendLogin();
    else this.backendAuth = { status: 'offline', user: null, error: '后端地址尚未配置，当前使用本地模式' };
    // 先初始化，再领取登录奖励。不要在后面把已领取的奖励重置为 0，
    // 否则奖励虽然写入存档，但用户看不到提示反馈。
    this.loginReward = 0;
    this.audio = new audioService.AudioService(this.progress);
    this.ads = new adService.AdService();
    this.ads.configure(this.progress.ads || {}, storage.todayKey());
    if (!apiClient.isConfigured || !apiClient.isConfigured()) {
      this.loginReward = storage.claimDailyLoginReward(this.progress, storage.todayKey());
      if (this.loginReward > 0) storage.save(this.progress);
    }
    this.levels = levelCatalog.all();
    const staticCampaign = puzzle.loadCampaignPuzzleBankFromData(campaignPuzzleData, this.levels, 240000);
    this.campaignPuzzleBank = staticCampaign ? staticCampaign.bank : null;
    this.campaignPuzzleBankStats = staticCampaign;
    this.questionService = new questionServiceModule.QuestionService({
      generator: puzzle,
      levels: this.levels,
      campaignData: campaignPuzzleData,
      campaignBank: this.campaignPuzzleBank,
      campaignSeedBase: 240000,
    });
    this.screen = 'home';
    this.audioScene = '';
    this.popup = '';
    this.tutorialStep = 0;
    this.buttons = [];
    this.stars = this.makeStars();
    this.floatNumbers = this.makeFloatNumbers();
    this.homeMotion = { activeButton: null, activeUntil: 0 };
    this.lastFrame = Date.now();
    this.mode = 'campaign';
    this.friendRoom = null;
    this.friendRoomFromInvite = false;
    this.friendRoomBackendStatus = 'idle';
    this.friendRoomBackendLoading = false;
    this.friendRoomLastPollAt = 0;
    this.friendMatch = null;
    this.friendMatchContract = null;
    this.friendAttempts = [];
    this.friendRules = friendMatch.rules();
    this.friendLocalFallback = false;
    this.friendServerResult = null;
    this.friendStartRequestInFlight = false;
    this.friendMatchProgress = null;
    this.friendProgressLastPollAt = 0;
    this.friendProgressRequestInFlight = false;
    this.friendProgressLastSentKey = '';
    this.friendMatchResolutionApplied = false;
    this.friendStartedAt = 0;
    this.friendPlayerSolved = 0;
    this.friendLobbyView = 'entry';
    this.friendRoomInput = '';
    this.friendInputKeyboardActive = false;
    this.friendMatchmaking = null;
    this.friendMatchmakingRunId = 0;
    this.friendMatchmakingRequestInFlight = false;
    this.friendMatchmakingLastPollAt = 0;
    this.friendMatchmakingLocal = false;
    this.friendBotDifficulty = 'standard';
    this.friendBotName = '';
    this.friendSelfReady = false;
    this.friendReadyRequestInFlight = false;
    this.friendStartRequestInFlight = false;
    this.friendServerStartAt = 0;
    this.friendCountdownActive = false;
    this.friendCountdownUntil = 0;
    this.friendCountdownLastNumber = 0;
    // 好友对战网络状态。网络短暂异常时保留对局，不直接结束或切换本地结果。
    this.friendConnectionState = 'connected';
    this.friendReconnectStartedAt = 0;
    this.friendReconnectDeadline = 0;
    this.friendReconnectNextAt = 0;
    this.friendReconnectRequestInFlight = false;
    this.friendRoomRequestInFlight = false;
    this.friendRoomExpired = false;
    this.friendRoomError = '';
    this.menuPage = clamp(Math.floor(safeNumber(this.progress.unlocked_level, 0) / 20), 0, 9);
    this.currentLevel = 0;
    // 每次切换模式、重开或返回首页都会递增。旧的网络响应即使晚到，也不能覆盖当前这一局。
    this.gameRequestToken = 0;
    this.lastLaunchSignature = '';
    this.currentQuestion = 0;
    this.puzzles = [];
    this.currentPuzzle = null;
    this.cards = [];
    this.originalCards = [];
    this.undoStack = [];
    this.selectedIndex = -1;
    this.selectedOperator = '';
    this.questionSteps = [];
    this.timeLeft = 0;
    this.timerLimit = 60;
    this.score = 0;
    this.combo = 0;
    this.mistakes = 0;
    this.maxCombo = 0;
    this.freeUndo = true;
    this.freeHint = true;
    this.hintUsed = false;
    this.status = '';
    this.result = null;
    this.shopNotice = '';
    this.shopActionInFlight = '';
    this.questionOperators = [];
    this.questionSteps = [];
    this.shopPage = 0;
    this.shopTab = 'themes';
    this.previewSkinId = '';
    this.previewSkinUntil = 0;
    this.previewSkinPrevious = '';
    this.achievementPage = 0;
    this.taskTab = 'daily';
    this.dailyChallenge = null;
    this.hintPopup = null;
    this.resultHelpPopup = false;
    this.touchEffect = null;
    this.feedback = { type: '', text: '', until: 0 };
    if (this.loginReward > 0) this.triggerFeedback('success', `今日登录奖励 +${this.loginReward} 金币`);
    this.lastSolvedElapsed = 0;
    this.dateKey = storage.todayKey();
    this.gamePaused = false;
    this.backgroundPausedAt = 0;
    this.transitioning = false;
    this.settling = false;
    this.settleToken = 0;
    this.autoNextAt = 0;
    this.autoNextToken = 0;
    this.endlessRunId = '';
    this.endlessSeed = 0;
    this.endlessRun = null;
    this.endlessAttempts = [];
    this.endlessServerResult = null;
    this.endlessRunLoading = false;
    this.endlessUsedKeys = {};
    this.campaignRun = null;
    this.campaignAttempts = [];
    this.campaignRunLoading = false;
    this.dailyRun = null;
    this.dailyAttempts = [];
    this.dailyRunLoading = false;
    this.autoNextFallbackMs = 0;
    this.renderRecovery = false;
    this.lastRuntimeError = null;
    this.diagnosticsEnabled = false;
    this.volumeDragType = '';
    this.volumeDragAreas = {};
    this.gameCardHitAreas = [];
    this.gameCardHitCardCount = 0;
    this.lastTouchHandled = false;
    this.lastHandledTouchPoint = null;
    if (!this.progress.tutorial_seen) this.popup = 'tutorial';
    this.leaderboardBoard = leaderboardService.BOARD_GLOBAL;
    this.leaderboardMode = leaderboardService.MODE_CAMPAIGN;
    this.leaderboardRemote = {};
    this.leaderboardRemoteLoading = {};
    this.leaderboardRemoteFailedAt = {};
    this.loop = this.loop.bind(this);
    wx.onTouchStart((event) => this.onTouch(event));
    if (wx.onKeyboardInput) wx.onKeyboardInput((event) => this.onFriendKeyboardInput(event));
    if (wx.onKeyboardConfirm) wx.onKeyboardConfirm((event) => this.onFriendKeyboardConfirm(event));
    if (wx.onKeyboardComplete) wx.onKeyboardComplete(() => { this.friendInputKeyboardActive = false; });
    // 音量条需要连续触摸事件。只监听 onTouchStart 会让滑块只能显示，无法拖动。
    if (wx.onTouchMove) wx.onTouchMove((event) => this.onTouchMove(event));
    if (wx.onTouchEnd) wx.onTouchEnd((event) => this.onTouchEnd(event));
    if (wx.onTouchCancel) wx.onTouchCancel((event) => this.onTouchEnd(event));
    if (wx.onHide) wx.onHide(() => this.pauseForBackground());
    if (wx.onShow) wx.onShow((options) => {
      this.resumeFromBackground();
      this.handleLaunchOptions(options);
    });
    if (wx.onError) wx.onError((error) => this.handleRuntimeError(error, 'wx.onError'));
    if (wx.onUnhandledRejection) wx.onUnhandledRejection((event) => this.handleRuntimeError(event && event.reason ? event.reason : event, 'unhandledRejection'));
    this.readFriendLaunchParams();
    this.loop();
  }

  startBackendLogin() {
    apiClient.ensureLogin().then((user) => {
      this.backendAuth = { status: 'ready', user: user || null, error: null };
      this.syncFriendRoomWithBackend();
      apiClient.bootstrap().then((bootstrap) => {
        if (bootstrap && bootstrap.progress) {
          this.progress = storage.mergeServerProgress(this.progress, bootstrap.progress, { authoritative: true });
          this.ads.configure(this.progress.ads || {}, storage.todayKey());
        }
        this.loginReward = Math.max(0, Number(bootstrap && bootstrap.login_reward || 0));
        this.backendAuth = { status: 'ready', user: (bootstrap && bootstrap.user) || user || null, error: null };
        this.syncFriendRoomWithBackend();
      }).catch((bootstrapError) => {
        try {
          if (typeof console !== 'undefined' && console.warn) console.warn('[game-backend-bootstrap]', bootstrapError);
        } catch (logError) { /* 鍒濆鏁版嵁澶辫触涓嶅奖鍝嶆父鎴?*/ }
      });
    }).catch((error) => {
      // Local play remains available when the API is unavailable in development.
      this.backendAuth = { status: 'offline', user: null, error: String(error && error.message || error || 'login failed') };
      if (this.screen === 'friend_lobby') this.activateLocalFriendRoom(this.friendRoom && this.friendRoom.room_code);
      try {
        if (typeof console !== 'undefined' && console.warn) console.warn('[game-backend-login]', this.backendAuth.error);
      } catch (logError) { /* 日志失败不影响游戏 */ }
    });
  }

  syncFriendRoomWithBackend() {
    if (!this.backendAuth || this.backendAuth.status !== 'ready') return;
    if (this.friendRoomFromInvite) this.joinBackendFriendRoom();
    else if (this.screen === 'friend_lobby' && this.friendLobbyView === 'room' && this.friendRoom) this.createBackendFriendRoom();
  }

  createBackendFriendRoom() {
    if (this.friendRoomFromInvite || this.friendRoomBackendLoading || this.friendRoomBackendStatus === 'ready') return;
    if (!apiClient.createFriendRoom) return;
    this.friendRoomBackendLoading = true;
    this.friendRoomBackendStatus = 'loading';
    const requestToken = this.gameRequestToken;
    apiClient.createFriendRoom().then((room) => {
      if (requestToken !== this.gameRequestToken || this.screen !== 'friend_lobby') return;
      if (room && room.room_code) {
        if (!this.applyFriendRoomPayload(room)) return;
        this.friendLocalFallback = false;
        this.friendRoomBackendStatus = 'ready';
      } else this.activateLocalFriendRoom(this.friendRoom && this.friendRoom.room_code);
    }).catch((error) => {
      if (requestToken !== this.gameRequestToken || this.screen !== 'friend_lobby') return;
      this.activateLocalFriendRoom(this.friendRoom && this.friendRoom.room_code);
      try {
        if (typeof console !== 'undefined' && console.warn) console.warn('[game-backend-friend-room-create]', error);
      } catch (logError) { /* local room remains available */ }
    }).then(() => {
      if (requestToken === this.gameRequestToken) this.friendRoomBackendLoading = false;
    });
  }

  joinBackendFriendRoom() {
    const roomCode = this.friendRoom && String(this.friendRoom.room_code || '').trim();
    if (!roomCode || this.friendRoomBackendLoading || this.friendRoomBackendStatus === 'ready') return;
    if (!apiClient.joinFriendRoom) return;
    this.friendRoomBackendLoading = true;
    this.friendRoomBackendStatus = 'loading';
    const requestToken = this.gameRequestToken;
    apiClient.joinFriendRoom(roomCode).then((room) => {
      if (requestToken !== this.gameRequestToken || this.screen !== 'friend_lobby') return;
      if (room && room.room_code) {
        if (!this.applyFriendRoomPayload(room)) return;
        this.friendLocalFallback = false;
        this.friendRoomBackendStatus = 'ready';
      } else this.activateLocalFriendRoom(roomCode);
    }).catch((error) => {
      if (requestToken !== this.gameRequestToken || this.screen !== 'friend_lobby') return;
      if (this.isFriendRoomTerminalError(error)) {
        this.friendRoomBackendStatus = 'ready';
        this.markFriendRoomExpired('expired');
      } else if (this.friendRoomFromInvite && apiClient.isConfigured && apiClient.isConfigured()) {
        this.friendRoomBackendStatus = 'ready';
        this.friendLocalFallback = false;
        this.beginFriendReconnect(error, 'join');
      } else {
        this.activateLocalFriendRoom(roomCode);
      }
      try {
        if (typeof console !== 'undefined' && console.warn) console.warn('[game-backend-friend-room-join]', error);
      } catch (logError) { /* local room remains available */ }
    }).then(() => {
      if (requestToken === this.gameRequestToken) this.friendRoomBackendLoading = false;
    });
  }

  activateLocalFriendRoom(roomCode = '') {
    const localRoom = friendMatch.createLocalRoom(roomCode || (this.friendRoom && this.friendRoom.room_code));
    this.friendRoom = localRoom;
    this.friendRules = Object.assign({}, friendMatch.rules(), localRoom.rules || {});
    this.friendRoomBackendStatus = 'local';
    this.friendRoomBackendLoading = false;
    this.friendLocalFallback = true;
    this.friendConnectionState = 'connected';
    this.friendRoomExpired = false;
    this.friendRoomError = '';
    this.friendMatchProgress = { players: [] };
    return localRoom;
  }

  friendRuleConfig() {
    return Object.assign({}, friendMatch.rules(), this.friendRules || {}, this.friendRoom && this.friendRoom.rules || {});
  }

  friendQuestionCount() {
    return Math.max(1, Math.floor(Number(this.friendRuleConfig().question_count || friendMatch.QUESTION_COUNT)));
  }

  friendTimeLimit() {
    return Math.max(10, Number(this.friendRuleConfig().time_limit || friendMatch.TIME_LIMIT));
  }

  isFriendBackendSession() {
    return (this.mode === 'friend' || this.screen === 'friend_lobby')
      && !this.friendLocalFallback
      && this.friendRoomBackendStatus === 'ready'
      && this.backendAuth
      && this.backendAuth.status === 'ready';
  }

  friendErrorStatus(error) {
    return Number(error && (error.statusCode || error.status || error.httpStatus || 0)) || 0;
  }

  isFriendRoomTerminalError(error) {
    const status = this.friendErrorStatus(error);
    const code = Number(error && error.code || 0);
    const message = String(error && (error.message || error.errMsg || error.code) || '').toLowerCase();
    return status === 404 || status === 410 || code === 404 || code === 410
      || message.includes('expired')
      || message.includes('cancelled')
      || message.includes('canceled')
      || message.includes('room_not_found')
      || message.includes('room_expired')
      || message.includes('room_cancelled');
  }

  isFriendRoomTerminal(room) {
    const status = String(room && (room.status || room.state) || '').toLowerCase();
    return status === friendMatch.ROOM_STATUS.EXPIRED || status === friendMatch.ROOM_STATUS.CANCELLED;
  }

  applyFriendRoomPayload(room) {
    if (!room || !room.room_code) return false;
    if (this.isFriendRoomTerminal(room)) {
      this.markFriendRoomExpired(String(room.status || room.state || 'expired'));
      return false;
    }
    this.friendRoom = friendMatch.normalizeRoom(room, this.friendRoom);
    this.friendRules = Object.assign({}, friendMatch.rules(), this.friendRoom.rules || {});
    this.friendRoomLastPollAt = Date.now();
    return true;
  }

  markFriendRoomExpired(reason = 'expired') {
    const normalizedReason = String(reason || 'expired').toLowerCase();
    this.friendRoomExpired = true;
    this.friendConnectionState = 'expired';
    this.friendRoomError = normalizedReason.includes('cancel') ? '房间已取消' : '房间已过期';
    this.friendReconnectRequestInFlight = false;
    this.friendRoomRequestInFlight = false;
    this.friendRoomBackendLoading = false;
    if (this.friendRoom) this.friendRoom.status = normalizedReason.includes('cancel') ? friendMatch.ROOM_STATUS.CANCELLED : friendMatch.ROOM_STATUS.EXPIRED;
    this.status = this.friendRoomError;
    this.triggerFeedback('error', this.friendRoomError);
  }

  markFriendConnectionRecovered() {
    if (this.friendRoomExpired) return;
    const wasReconnecting = this.friendConnectionState === 'reconnecting' || this.friendConnectionState === 'reconnect_timeout';
    this.friendConnectionState = 'connected';
    this.friendReconnectStartedAt = 0;
    this.friendReconnectDeadline = 0;
    this.friendReconnectNextAt = 0;
    this.friendRoomError = '';
    if (wasReconnecting) {
      this.status = '连接已恢复';
      this.triggerFeedback('success', '连接已恢复，继续对战');
    }
  }

  beginFriendReconnect(error = null, source = 'network') {
    if (!this.isFriendBackendSession() || this.friendRoomExpired) return;
    const now = Date.now();
    if (this.isFriendRoomTerminalError(error)) {
      this.markFriendRoomExpired('expired');
      return;
    }
    if (this.friendConnectionState !== 'reconnecting') {
      this.friendConnectionState = 'reconnecting';
      this.friendReconnectStartedAt = now;
      this.friendReconnectDeadline = now + 15000;
      this.friendReconnectNextAt = now;
      this.friendRoomError = source === 'resume' ? '正在恢复对战连接' : '网络连接中断';
      this.status = '正在重新连接，请稍候';
      this.triggerFeedback('info', '正在重新连接，不会立即结束对局');
    }
    if (error && typeof console !== 'undefined' && console.warn) {
      try { console.warn('[game-backend-friend-reconnect]', error); } catch (logError) { /* reconnect UI remains available */ }
    }
  }

  retryFriendConnection() {
    if (!this.isFriendBackendSession() || this.friendRoomExpired) return;
    const now = Date.now();
    this.friendConnectionState = 'reconnecting';
    this.friendReconnectStartedAt = now;
    this.friendReconnectDeadline = now + 15000;
    this.friendReconnectNextAt = now;
    this.friendReconnectRequestInFlight = false;
    this.friendRoomLastPollAt = 0;
    this.friendProgressLastPollAt = 0;
    this.status = '正在重新连接，请稍候';
  }

  updateFriendReconnect() {
    if (this.friendConnectionState !== 'reconnecting' || this.friendRoomExpired) return;
    if (!this.isFriendBackendSession()) return;
    if (!['friend_lobby', 'game', 'result'].includes(this.screen)) return;
    const now = Date.now();
    if (now >= Number(this.friendReconnectDeadline || 0)) {
      this.friendConnectionState = 'reconnect_timeout';
      this.friendRoomError = '连接超时，请选择重连或退出对战';
      this.status = this.friendRoomError;
      this.triggerFeedback('error', '连接超时');
      return;
    }
    if (this.friendReconnectRequestInFlight || now < Number(this.friendReconnectNextAt || 0)) return;
    const roomCode = this.friendRoom && String(this.friendRoom.room_code || '').trim();
    if (!roomCode || !apiClient.getFriendRoom) return;
    this.friendReconnectRequestInFlight = true;
    apiClient.getFriendRoom(roomCode).then((room) => {
      if (!['friend_lobby', 'game', 'result'].includes(this.screen)
        || String(this.friendRoom && this.friendRoom.room_code || '') !== roomCode) return;
      if (!this.applyFriendRoomPayload(room)) return;
      this.friendProgressLastPollAt = 0;
      this.markFriendConnectionRecovered();
    }).catch((error) => {
      if (!['friend_lobby', 'game', 'result'].includes(this.screen)
        || String(this.friendRoom && this.friendRoom.room_code || '') !== roomCode) return;
      if (this.isFriendRoomTerminalError(error)) this.markFriendRoomExpired('expired');
      else {
        this.friendReconnectNextAt = Date.now() + 1500;
        this.friendRoomError = '正在重新连接，请稍候';
        this.status = this.friendRoomError;
      }
    }).then(() => {
      this.friendReconnectRequestInFlight = false;
      if (this.friendConnectionState === 'reconnecting') this.friendReconnectNextAt = Date.now() + 1500;
    });
  }

  pollBackendFriendRoom() {
    if (this.friendRoomBackendStatus !== 'ready' || this.friendRoomBackendLoading || this.friendRoomRequestInFlight || !apiClient.getFriendRoom) return;
    if (this.friendConnectionState === 'reconnecting' || this.friendConnectionState === 'reconnect_timeout' || this.friendRoomExpired) return;
    if (!['friend_lobby', 'game', 'result'].includes(this.screen)) return;
    const roomCode = this.friendRoom && String(this.friendRoom.room_code || '').trim();
    if (!roomCode || Date.now() - this.friendRoomLastPollAt < 2000) return;
    this.friendRoomLastPollAt = Date.now();
    this.friendRoomRequestInFlight = true;
    apiClient.getFriendRoom(roomCode).then((room) => {
      if (!['friend_lobby', 'game', 'result'].includes(this.screen)
        || String(this.friendRoom && this.friendRoom.room_code || '') !== roomCode) return;
      this.applyFriendRoomPayload(room);
    }).catch((error) => {
      if (this.isFriendRoomTerminalError(error)) this.markFriendRoomExpired('expired');
      else if (this.screen === 'friend_lobby' || this.screen === 'game' || this.screen === 'result') this.beginFriendReconnect(error, 'room-poll');
      try {
        if (typeof console !== 'undefined' && console.warn) console.warn('[game-backend-friend-room-poll]', error);
      } catch (logError) { /* lobby polling is best effort */ }
    }).then(() => {
      this.friendRoomRequestInFlight = false;
    });
  }

  friendOpponentName() {
    if (this.friendBotName) return this.friendBotName;
    const players = this.friendRoom && Array.isArray(this.friendRoom.players)
      ? this.friendRoom.players
      : [];
    const currentPlayerID = this.backendAuth && this.backendAuth.user
      ? String(this.backendAuth.user.id || this.backendAuth.user.user_id || '')
      : 'local-player';
    const opponent = players.find((player) => String(player && (player.user_id || player.id) || '') !== currentPlayerID);
    return String(opponent && (opponent.nickname || opponent.name) || '').trim() || '对手';
  }

  friendOpponentState() {
    if (this.friendLocalFallback) {
      const elapsed = this.friendTimeLimit() - Math.max(0, Number(this.timeLeft) || 0);
      const plan = friendMatch.buildOpponentPlan(this.friendRoom && this.friendRoom.room_seed, this.friendQuestionCount(), this.friendBotDifficulty || 'standard');
      return friendMatch.opponentSnapshot(plan, elapsed, this.friendQuestionCount(), false);
    }
    const currentUserID = this.backendAuth && this.backendAuth.user
      ? String(this.backendAuth.user.id || this.backendAuth.user.user_id || '')
      : 'local-player';
    const players = this.friendMatchProgress && Array.isArray(this.friendMatchProgress.players)
      ? this.friendMatchProgress.players
      : [];
    const opponent = players.find((player) => String(player && (player.user_id || player.id) || '') !== currentUserID);
    return {
      solved: Math.max(0, Number(opponent && opponent.solved || 0)),
      score: Math.max(0, Number(opponent && opponent.score || 0)),
      elapsed: Math.max(0, Number(opponent && opponent.elapsed_ms || 0) / 1000),
      finished: Boolean(opponent && opponent.finished),
    };
  }

  pollFriendMatchProgress() {
    if (this.mode !== 'friend' || !this.backendAuth || this.backendAuth.status !== 'ready') return;
    if (!['game', 'result'].includes(this.screen)) return;
    if (this.friendConnectionState === 'reconnecting' || this.friendConnectionState === 'reconnect_timeout' || this.friendRoomExpired) return;
    if (!apiClient.getFriendMatchProgress || this.friendProgressRequestInFlight) return;
    const roomCode = this.friendRoom && String(this.friendRoom.room_code || '').trim();
    if (!roomCode || Date.now() - this.friendProgressLastPollAt < 1500) return;
    this.friendProgressLastPollAt = Date.now();
    this.friendProgressRequestInFlight = true;
    apiClient.getFriendMatchProgress(roomCode).then((progress) => {
      if (!['game', 'result'].includes(this.screen)
        || String(this.friendRoom && this.friendRoom.room_code || '') !== roomCode) return;
      this.friendMatchProgress = progress || { players: [] };
      this.markFriendConnectionRecovered();
      this.resolvePendingFriendMatch();
    }).catch((error) => {
      if (this.isFriendRoomTerminalError(error)) this.markFriendRoomExpired('expired');
      else if (this.screen === 'game' || this.screen === 'result') this.beginFriendReconnect(error, 'progress-poll');
      try {
        if (typeof console !== 'undefined' && console.warn) console.warn('[game-backend-friend-progress]', error);
      } catch (logError) { /* 对手进度读取失败不影响当前题目 */ }
    }).then(() => {
      this.friendProgressRequestInFlight = false;
    });
  }

  sendFriendMatchProgress(finished = false) {
    if (this.mode !== 'friend' || !this.backendAuth || this.backendAuth.status !== 'ready') return;
    if (!apiClient.updateFriendMatchProgress) return;
    const roomCode = this.friendRoom && String(this.friendRoom.room_code || '').trim();
    if (!roomCode) return;
    const timeLimit = this.friendTimeLimit();
    const elapsedMs = Math.round(clamp(timeLimit - Math.max(0, this.timeLeft), 0, timeLimit) * 1000);
    const lastAttempt = Array.isArray(this.friendAttempts) && this.friendAttempts.length
      ? this.friendAttempts[this.friendAttempts.length - 1]
      : null;
    const payload = {
      question_index: Math.max(0, Math.min(this.friendQuestionCount() - 1, Number(this.currentQuestion || 0))),
      solved: Math.max(0, Math.min(this.friendQuestionCount(), Number(this.friendPlayerSolved || 0))),
      score: Math.max(0, Math.floor(Number(this.score) || 0)),
      elapsed_ms: elapsedMs,
      finished: Boolean(finished),
      match_id: String(this.friendMatchContract && this.friendMatchContract.match_id || ''),
      question_hash: String(this.friendMatchContract && this.friendMatchContract.question_hash || ''),
      event_id: String(lastAttempt && lastAttempt.event_id || ''),
      attempt: lastAttempt,
    };
    const key = `${payload.question_index}:${payload.solved}:${payload.score}:${payload.elapsed_ms}:${payload.finished ? 1 : 0}`;
    if (key === this.friendProgressLastSentKey) return;
    this.friendProgressLastSentKey = key;
    apiClient.updateFriendMatchProgress(roomCode, payload).then((progress) => {
      if (progress) this.friendMatchProgress = progress;
      this.markFriendConnectionRecovered();
    }).catch((error) => {
      this.friendProgressLastSentKey = '';
      if (this.isFriendRoomTerminalError(error)) this.markFriendRoomExpired('expired');
      else this.beginFriendReconnect(error, 'progress-submit');
      try {
        if (typeof console !== 'undefined' && console.warn) console.warn('[game-backend-friend-progress-submit]', error);
      } catch (logError) { /* 进度上报失败可在下一次题目结算时重试 */ }
    });
  }

  applyFriendMatchResult(matchResult, elapsed, bonusLabels) {
    if (!this.friendLocalFallback || !matchResult || matchResult.outcome === 'pending' || this.friendMatchResolutionApplied) return 0;
    const record = this.progress.friend_matches || { date: '', played: 0, wins: 0, best_score: 0, best_time_ms: 0 };
    record.date = storage.todayKey();
    record.played = safeNumber(record.played) + 1;
    if (matchResult.outcome === 'win') record.wins = safeNumber(record.wins) + 1;
    record.best_score = Math.max(safeNumber(record.best_score), this.score);
    const elapsedMs = Math.round(Math.max(0, Number(elapsed) || 0) * 1000);
    record.best_time_ms = record.best_time_ms > 0 ? Math.min(record.best_time_ms, elapsedMs) : elapsedMs;
    this.progress.friend_matches = record;
    const reward = storage.claimFriendReward(this.progress, matchResult.outcome, storage.todayKey(), friendMatch.DAILY_REWARD_MATCH_LIMIT);
    if (reward <= 0 && Array.isArray(bonusLabels)) bonusLabels.push('今日对战奖励已达上限');
    this.friendMatchResolutionApplied = true;
    storage.save(this.progress);
    return reward;
  }

  normalizeServerFriendResult(payload) {
    const source = payload && (payload.match_result || payload.matchResult || payload.result || payload.data || payload);
    if (!source || typeof source !== 'object') return null;
    const player = source.player || source.self || {};
    const opponent = source.opponent || source.other || {};
    const outcome = String(source.outcome || source.status || '').toLowerCase();
    if (!['win', 'lose', 'draw'].includes(outcome)) return null;
    return {
      outcome,
      player_solved: Math.max(0, Number(source.player_solved ?? player.solved ?? 0)),
      player_score: Math.max(0, Number(source.player_score ?? player.score ?? 0)),
      player_mistakes: Math.max(0, Number(source.player_mistakes ?? player.mistakes ?? 0)),
      player_elapsed: Math.max(0, Number(source.player_elapsed ?? player.elapsed ?? 0)),
      opponent_solved: Math.max(0, Number(source.opponent_solved ?? opponent.solved ?? 0)),
      opponent_score: Math.max(0, Number(source.opponent_score ?? opponent.score ?? 0)),
      opponent_mistakes: Math.max(0, Number(source.opponent_mistakes ?? opponent.mistakes ?? 0)),
      opponent_elapsed: Math.max(0, Number(source.opponent_elapsed ?? opponent.elapsed ?? 0)),
    };
  }

  applyServerFriendMatchResult(payload) {
    if (this.friendLocalFallback || this.friendMatchResolutionApplied) return false;
    const matchResult = this.normalizeServerFriendResult(payload);
    if (!matchResult || !this.result) return false;
    this.friendServerResult = payload;
    this.result.matchResult = matchResult;
    this.result.serverVerified = true;
    // 对战结算以服务端数值为准，避免本地预测分数覆盖真实胜负。
    this.result.passed = matchResult.outcome === 'win';
    this.result.score = matchResult.player_score;
    this.result.mistakes = matchResult.player_mistakes;
    this.result.reason = matchResult.outcome === 'win' ? '服务端确认胜利' : matchResult.outcome === 'draw' ? '服务端确认平局' : '服务端确认惜败';
    this.result.next = false;
    const rewardValue = payload && (payload.reward_coins ?? payload.rewardCoins);
    const reward = Number.isFinite(Number(rewardValue)) ? Math.max(0, Number(rewardValue)) : 0;
    this.result.rewardCoins = reward;
    if (reward > 0 && Array.isArray(this.result.bonusLabels)) this.result.bonusLabels.push(`好友对战奖励 +${reward}`);
    const record = this.progress.friend_matches || { date: '', played: 0, wins: 0, best_score: 0, best_time_ms: 0 };
    record.date = storage.todayKey();
    record.played = safeNumber(record.played) + 1;
    if (matchResult.outcome === 'win') record.wins = safeNumber(record.wins) + 1;
    record.best_score = Math.max(safeNumber(record.best_score), matchResult.player_score);
    record.best_time_ms = record.best_time_ms > 0
      ? Math.min(record.best_time_ms, matchResult.player_elapsed * 1000)
      : matchResult.player_elapsed * 1000;
    this.progress.friend_matches = record;
    if (payload && payload.progress) this.progress = storage.mergeServerProgress(this.progress, payload.progress, { authoritative: true });
    if (payload && Number.isFinite(Number(payload.coins))) this.progress.coins = Math.max(0, Math.floor(Number(payload.coins)));
    else if (reward > 0) storage.addCoins(this.progress, reward);
    this.friendMatchResolutionApplied = true;
    storage.save(this.progress);
    return true;
  }

  resolvePendingFriendMatch() {
    if (this.mode !== 'friend' || this.screen !== 'result' || !this.result || !this.result.matchResult || this.result.matchResult.outcome !== 'pending') return;
    const remoteResult = this.friendMatchProgress && (this.friendMatchProgress.match_result || this.friendMatchProgress.matchResult || this.friendMatchProgress.result);
    if (remoteResult && this.applyServerFriendMatchResult(remoteResult)) return;
    if (!this.friendLocalFallback) return;
    const opponent = this.friendOpponentState();
    if (!opponent.finished) return;
    const timeLimit = this.friendTimeLimit();
    const elapsed = clamp(timeLimit - Math.max(0, this.timeLeft), 0, timeLimit);
    const matchResult = friendMatch.calculateResult(this.friendPlayerSolved, this.score, this.mistakes, elapsed, opponent);
    this.result.matchResult = matchResult;
    this.result.rewardCoins = safeNumber(this.result.rewardCoins) + this.applyFriendMatchResult(matchResult, elapsed, this.result.bonusLabels);
    storage.save(this.progress);
  }

  loadRemoteLeaderboard(mode, scope = 'global') {
    if (mode !== leaderboardService.MODE_CAMPAIGN && mode !== leaderboardService.MODE_DAILY && mode !== leaderboardService.MODE_ENDLESS && mode !== leaderboardService.MODE_FRIEND) return;
    if (!this.backendAuth || this.backendAuth.status !== 'ready' || !apiClient.getLeaderboard) return;
    const key = `${scope}:${mode}`;
    if (this.leaderboardRemote[key] || this.leaderboardRemoteLoading[key]) return;
    const failedAt = Number(this.leaderboardRemoteFailedAt[key] || 0);
    // 失败后不要在每一帧反复请求；保留手动刷新入口，弱网下也不会打满接口。
    if (failedAt > 0 && Date.now() - failedAt < 15000) return;
    this.leaderboardRemoteLoading[key] = true;
    apiClient.getLeaderboard(mode, scope).then((data) => {
      this.leaderboardRemote[key] = data || { entries: [] };
      this.leaderboardRemoteFailedAt[key] = 0;
      this.leaderboardRemoteLoading[key] = false;
    }).catch((error) => {
      this.leaderboardRemoteFailedAt[key] = Date.now();
      try {
        if (typeof console !== 'undefined' && console.warn) console.warn('[game-backend-leaderboard]', error);
      } catch (logError) { /* leaderboard failure does not block local play */ }
      this.leaderboardRemoteLoading[key] = false;
    });
  }

  refreshLeaderboard() {
    const mode = this.leaderboardMode;
    const scope = this.leaderboardBoard === leaderboardService.BOARD_FRIENDS ? 'friends' : 'global';
    const key = `${scope}:${mode}`;
    if (!this.backendAuth || this.backendAuth.status !== 'ready' || !apiClient.getLeaderboard) {
      this.triggerFeedback('info', '排行榜服务尚未连接，当前显示本机成绩');
      return;
    }
    if (this.leaderboardRemoteLoading[key]) {
      this.triggerFeedback('info', '排行榜正在刷新，请稍候');
      return;
    }
    delete this.leaderboardRemote[key];
    delete this.leaderboardRemoteFailedAt[key];
    this.triggerFeedback('info', '正在刷新排行榜');
    this.loadRemoteLeaderboard(mode, scope);
  }

  submitCampaignLevelCompletion(levelID, score, stars) {
    if (!this.backendAuth || this.backendAuth.status !== 'ready') return;
    const applyServerResult = (serverResult) => {
      if (!serverResult) return;
      this.leaderboardRemote = {};
      this.leaderboardRemoteFailedAt = {};
      const serverLevelID = String(Number(serverResult.level_id));
      const serverProgress = serverResult.progress && typeof serverResult.progress === 'object'
        ? serverResult.progress
        : {
          coins: Number(serverResult.coins || 0),
          unlocked_level: Number(serverResult.unlocked_level || 0),
          levels: {
            [serverLevelID]: {
              stars: Number(serverResult.stars || 0),
              best_score: Number(serverResult.best_score || serverResult.score || 0),
              completed: true,
            },
          },
          level_rewards: Number(serverResult.reward_coins || 0) > 0 ? { [serverLevelID]: true } : {},
        };
      this.progress = storage.mergeServerProgress(this.progress, serverProgress, { authoritative: true });
      if (this.result && serverResult.validated !== undefined) {
        this.result.serverVerified = Boolean(serverResult.validated);
        this.result.rewardCoins = Number(serverResult.reward_coins || 0);
        if (this.result.rewardCoins > 0 && Array.isArray(this.result.bonusLabels)) {
          this.result.bonusLabels.push(`闯关服务端奖励 +${this.result.rewardCoins}`);
        }
      }
    };

    if (this.campaignRun && apiClient.submitCampaignRun) {
      const runID = String(this.campaignRun.run_id || this.campaignRun.runId || '');
      if (!runID) return;
      const attempts = (Array.isArray(this.campaignAttempts) ? this.campaignAttempts : []).map((attempt) => ({
        puzzle_id: String(attempt.puzzle_id || ''),
        question_index: Math.max(0, Math.floor(Number(attempt.question_index) || 0)),
        elapsed_ms: Math.max(0, Math.floor(Number(attempt.elapsed_ms) || 0)),
        solved: Boolean(attempt.solved),
        mistakes: Math.max(0, Math.floor(Number(attempt.mistakes) || 0)),
        hints: Math.max(0, Math.floor(Number(attempt.hints) || 0)),
        score: Math.max(0, Math.min(100, Math.floor(Number(attempt.score) || 0))),
        score_delta: Math.floor(Number(attempt.score_delta) || 0),
        combo: Math.max(0, Math.floor(Number(attempt.combo) || 0)),
        solution_steps: Array.isArray(attempt.solution_steps) ? attempt.solution_steps : [],
      }));
      const lastAttempt = attempts[attempts.length - 1] || {};
      const elapsedMS = attempts.reduce((total, attempt) => total + attempt.elapsed_ms, 0);
      apiClient.submitCampaignRun(runID, {
        protocol_version: 1,
        idempotency_key: `campaign_${runID}`,
        run_id: runID,
        level_id: Math.max(0, Math.floor(Number(levelID) || 0)),
        attempts,
        summary: {
          questions: attempts.length,
          score: Math.max(0, Math.min(100, Math.floor(Number(score) || 0))),
          elapsed_ms: elapsedMS,
          mistakes: Math.max(0, Math.floor(Number(lastAttempt.mistakes) || 0)),
          hints: Math.max(0, Math.floor(Number(lastAttempt.hints) || 0)),
          best_combo: Math.max(0, Math.floor(Number(this.maxCombo) || 0)),
          stars: Math.max(1, Math.min(3, Math.floor(Number(stars) || 1))),
        },
        client_authoritative: false,
      }).then(applyServerResult).catch((error) => {
        try {
          if (typeof console !== 'undefined' && console.warn) console.warn('[game-backend-campaign-submit]', error);
        } catch (logError) { /* 闯关本地结果仍可展示 */ }
      });
      return;
    }

    if (!apiClient.completeLevel) return;
    const idempotencyKey = `level_${levelID}_${Date.now()}_${Math.random().toString(36).slice(2, 12)}`;
    apiClient.completeLevel(levelID, {
      idempotency_key: idempotencyKey,
      score: Math.max(0, Math.min(100, Math.floor(Number(score) || 0))),
      stars: Math.max(1, Math.min(3, Math.floor(Number(stars) || 1))),
    }).then(applyServerResult).catch((error) => {
      try {
        if (typeof console !== 'undefined' && console.warn) console.warn('[game-backend-level-complete]', error);
      } catch (logError) { /* legacy compatibility path */ }
    });
  }

  submitDailyChallengeCompletion(score) {
    if (!this.backendAuth || this.backendAuth.status !== 'ready') return;
    if (this.dailyRun && apiClient.submitDailyRun) {
      const runID = String(this.dailyRun.run_id || this.dailyRun.runId || '');
      const dateKey = String(this.dailyRun.date_key || this.dailyRun.dateKey || storage.todayKey());
      if (!runID) return;
      const attempts = (Array.isArray(this.dailyAttempts) ? this.dailyAttempts : []).map((attempt) => ({
        puzzle_id: String(attempt.puzzle_id || ''),
        question_index: Math.max(0, Math.floor(Number(attempt.question_index) || 0)),
        elapsed_ms: Math.max(0, Math.floor(Number(attempt.elapsed_ms) || 0)),
        solved: Boolean(attempt.solved),
        mistakes: Math.max(0, Math.floor(Number(attempt.mistakes) || 0)),
        hints: Math.max(0, Math.floor(Number(attempt.hints) || 0)),
        score: Math.max(0, Math.floor(Number(attempt.score) || 0)),
        score_delta: Math.max(0, Math.floor(Number(attempt.score_delta) || 0)),
        combo: Math.max(1, Math.floor(Number(attempt.combo) || 1)),
        solution_steps: Array.isArray(attempt.solution_steps) ? attempt.solution_steps : [],
      }));
      const lastAttempt = attempts[attempts.length - 1] || {};
      const elapsedMS = attempts.reduce((total, attempt) => total + attempt.elapsed_ms, 0);
      apiClient.submitDailyRun(runID, {
        protocol_version: 1,
        idempotency_key: `daily_${dateKey}`,
        run_id: runID,
        date_key: dateKey,
        attempts,
        summary: {
          date_key: dateKey,
          questions: attempts.length,
          score: Math.max(0, Math.floor(Number(score) || 0)),
          elapsed_ms: elapsedMS,
          mistakes: Math.max(0, Math.floor(Number(lastAttempt.mistakes) || 0)),
          hints: Math.max(0, Math.floor(Number(lastAttempt.hints) || 0)),
          best_combo: Math.max(0, Math.floor(Number(this.maxCombo) || 0)),
        },
        client_authoritative: false,
      }).then((serverResult) => {
        if (!serverResult) return;
        this.leaderboardRemote = {};
        this.leaderboardRemoteFailedAt = {};
        const serverDateKey = String(serverResult.date_key || dateKey);
        const serverProgress = serverResult.progress && typeof serverResult.progress === 'object'
          ? serverResult.progress
          : {
            coins: Number(serverResult.coins || 0),
            daily: {
              last_date: serverDateKey,
              streak: Number(serverResult.streak || 0),
              best_score: Number(serverResult.best_score || serverResult.score || 0),
              completed: { [serverDateKey]: true },
              reward_claimed: { [serverDateKey]: true },
            },
          };
        this.progress = storage.mergeServerProgress(this.progress, serverProgress, { authoritative: true });
        if (this.result && serverResult.validated !== undefined) {
          this.result.serverVerified = Boolean(serverResult.validated);
          this.result.rewardCoins = Number(serverResult.reward_coins || 0);
          if (this.result.rewardCoins > 0 && Array.isArray(this.result.bonusLabels)) {
            this.result.bonusLabels.push(`每日挑战服务端奖励 +${this.result.rewardCoins}`);
          }
        }
      }).catch((error) => {
        try { if (typeof console !== 'undefined' && console.warn) console.warn('[game-backend-daily-submit]', error); } catch (logError) { /* daily local result remains visible */ }
      });
      return;
    }
    if (!apiClient.completeDaily) return;
    const idempotencyKey = `daily_${storage.todayKey()}_${Date.now()}_${Math.random().toString(36).slice(2, 12)}`;
    apiClient.completeDaily({
      idempotency_key: idempotencyKey,
      score: Math.max(0, Math.min(10000000, Math.floor(Number(score) || 0))),
    }).then((serverResult) => {
      if (!serverResult) return;
      this.leaderboardRemote = {};
      this.leaderboardRemoteFailedAt = {};
      const dateKey = String(serverResult.date_key || storage.todayKey());
      this.progress = storage.mergeServerProgress(this.progress, {
        coins: Number(serverResult.coins || 0),
        daily: {
          last_date: dateKey,
          streak: Number(serverResult.streak || 0),
          best_score: Number(serverResult.best_score || 0),
          completed: { [dateKey]: true },
          reward_claimed: { [dateKey]: true },
        },
      });
    }).catch((error) => {
      try {
        if (typeof console !== 'undefined' && console.warn) console.warn('[game-backend-daily-complete]', error);
      } catch (logError) { /* 鍚庡彴涓嶅彲鐢ㄦ椂淇濈暀鏈湴姣忔棩鎸戞垬 */ }
    });
  }

  submitServerLeaderboardScore(mode, data = {}) {
    if (!this.backendAuth || this.backendAuth.status !== 'ready' || !apiClient.submitLeaderboardScore) return;
    apiClient.submitLeaderboardScore(mode, data).then(() => {
      this.leaderboardRemote = {};
      this.leaderboardRemoteFailedAt = {};
    }).catch((error) => {
      try {
        if (typeof console !== 'undefined' && console.warn) console.warn('[game-backend-leaderboard-submit]', error);
      } catch (logError) { /* local leaderboard remains available */ }
    });
  }

  submitEndlessLeaderboard(score, questions, elapsedMs) {
    const runID = String(this.endlessRunId || `endless-${Date.now()}`);
    if (this.endlessRun && !this.endlessRunLoading && apiClient.submitEndlessRun) {
      const attempts = Array.isArray(this.endlessAttempts) ? this.endlessAttempts.map((attempt) => ({
        puzzle_id: String(attempt.puzzle_id || ''),
        question_index: Number(attempt.question_index || 0),
        elapsed_ms: Math.max(0, Math.floor(Number(attempt.elapsed_ms) || 0)),
        solved: Boolean(attempt.solved),
        mistakes: Math.max(0, Math.floor(Number(attempt.mistakes) || 0)),
        score: Math.max(0, Math.floor(Number(attempt.score) || 0)),
        score_delta: Math.max(0, Math.floor(Number(attempt.score_delta) || 0)),
        combo: Math.max(0, Math.floor(Number(attempt.combo) || 0)),
        solution_steps: Array.isArray(attempt.solution_steps) ? attempt.solution_steps : [],
      })) : [];
      const totalElapsedMS = attempts.reduce((total, attempt) => total + Math.max(0, Number(attempt.elapsed_ms) || 0), 0);
      apiClient.submitEndlessRun(runID, {
        protocol_version: 1,
        idempotency_key: `endless_${runID}`,
        run_id: runID,
        attempts,
        summary: {
          questions: Math.max(0, Math.floor(Number(questions) || 0)),
          score: Math.max(0, Math.floor(Number(score) || 0)),
          elapsed_ms: Math.max(0, Math.floor(Number(totalElapsedMS || elapsedMs) || 0)),
          mistakes: Math.max(0, Math.floor(Number(this.mistakes) || 0)),
          best_combo: Math.max(0, Math.floor(Number(this.maxCombo) || 0)),
        },
        client_authoritative: false,
      }).then((serverResult) => {
        this.endlessServerResult = serverResult || null;
        if (serverResult && serverResult.progress) {
          this.progress = storage.mergeServerProgress(this.progress, serverResult.progress, { authoritative: true });
        }
        if (this.result && serverResult) {
          this.result.serverVerified = Boolean(serverResult.validated);
          this.result.rewardCoins = Number(serverResult.reward_coins || 0);
          if (this.result.rewardCoins > 0 && Array.isArray(this.result.bonusLabels)) this.result.bonusLabels.push(`无尽模式服务端奖励 +${this.result.rewardCoins}`);
        }
        this.leaderboardRemote = {};
        this.leaderboardRemoteFailedAt = {};
      }).catch((error) => {
        try { if (typeof console !== 'undefined' && console.warn) console.warn('[game-backend-endless-submit]', error); } catch (logError) { /* best effort */ }
      });
      return;
    }
    this.submitServerLeaderboardScore(leaderboardService.MODE_ENDLESS, {
      idempotency_key: runID,
      score: Math.max(0, Math.floor(Number(score) || 0)),
      questions: Math.max(0, Math.floor(Number(questions) || 0)),
      elapsed_ms: Math.max(0, Math.floor(Number(elapsedMs) || 0)),
      metadata: { run_id: runID },
    });
  }

  submitFriendLeaderboard(result, room, submission = null) {
    const matchID = `${String(this.friendMatch && this.friendMatch.match_id || room && room.room_id || 'friend')}_${Number(this.friendStartedAt || Date.now())}`;
    const match = result || {};
    if (submission && this.backendAuth && this.backendAuth.status === 'ready' && apiClient.submitFriendMatch) {
      const payload = Object.assign({}, submission, {
        idempotency_key: String(submission.idempotency_key || `friend_${matchID}`),
      });
      const roomCode = String(room && room.room_code || '').trim();
      apiClient.submitFriendMatch(roomCode, payload).then((serverResult) => {
        if (!this.applyServerFriendMatchResult(serverResult)) {
          this.triggerFeedback('info', '对局已提交，等待服务端结算');
        }
        this.leaderboardRemote = {};
        this.leaderboardRemoteFailedAt = {};
      }).catch((error) => {
        if (this.isFriendRoomTerminalError(error)) this.markFriendRoomExpired('expired');
        else this.beginFriendReconnect(error, 'result-submit');
        try {
          this.triggerFeedback('error', '对局校验失败，成绩未上传');
          if (typeof console !== 'undefined' && console.warn) console.warn('[game-backend-friend-match-submit]', error);
        } catch (logError) { /* 校验错误不能影响结算页 */ }
      });
      return;
    }
    this.submitServerLeaderboardScore(leaderboardService.MODE_FRIEND, {
      idempotency_key: `friend_${matchID}`,
      score: Math.max(0, Math.floor(Number(match.player_score) || 0)),
      questions: Math.max(0, Math.floor(Number(match.player_solved) || 0)),
      elapsed_ms: Math.max(0, Math.floor((Number(match.player_elapsed) || 0) * 1000)),
      room_id: String(room && room.room_id || ''),
      outcome: ['win', 'lose', 'draw'].includes(String(match.outcome || '')) ? String(match.outcome) : '',
      metadata: { player_mistakes: Math.max(0, Math.floor(Number(match.player_mistakes) || 0)) },
    });
  }

  pauseForBackground() {
    if (this.gamePaused) return;
    this.gamePaused = true;
    this.backgroundPausedAt = Date.now();
    if (this.audio && this.audio.pause) this.audio.pause();
    try { this.progress = storage.save(this.progress); } catch (error) { /* 后台保存失败不阻断游戏 */ }
  }

  resumeFromBackground() {
    if (!this.gamePaused) return;
    this.gamePaused = false;
    const pausedSeconds = this.backgroundPausedAt > 0 ? Math.max(0, (Date.now() - this.backgroundPausedAt) / 1000) : 0;
    this.backgroundPausedAt = 0;
    this.lastFrame = Date.now();
    if (this.friendCountdownActive && pausedSeconds > 0) this.friendCountdownUntil += pausedSeconds * 1000;
    if (this.audio && this.audio.resume) this.audio.resume();
    this.friendRoomLastPollAt = 0;
    this.friendProgressLastPollAt = 0;
    this.syncDateScopedState();
    if (this.isFriendBackendSession() && pausedSeconds > 1 && (this.screen === 'friend_lobby' || this.screen === 'game' || this.screen === 'result')) {
      this.beginFriendReconnect(null, 'resume');
    }
    if (this.mode === 'friend' && !this.friendLocalFallback && this.screen === 'game' && pausedSeconds > 0) {
      this.timeLeft = Math.max(0, this.timeLeft - pausedSeconds);
      if (this.timeLeft <= 0) {
        this.finish(false, '对战时间已结束');
        return;
      }
    }
    if (this.screen === 'game' && pausedSeconds > 0) {
      this.status = '已暂停，回来后继续计算';
      this.triggerFeedback('info', '后台暂停期间不扣时间');
    }
  }

  syncDateScopedState() {
    const currentDate = storage.todayKey();
    if (currentDate === this.dateKey) return;
    this.dateKey = currentDate;
    this.progress = storage.load();
    this.ads.configure(this.progress.ads || {}, currentDate);
  }

  applyRenderTransform() {
    const scale = this.dpr * this.renderScale;
    const offsetX = this.dpr * this.renderOffsetX;
    const offsetY = this.dpr * this.renderOffsetY;
    if (this.ctx.setTransform) this.ctx.setTransform(scale, 0, 0, scale, offsetX, offsetY);
    else this.ctx.scale(scale, scale);
  }

  readFriendLaunchParams() {
    try {
      const launch = wx.getEnterOptionsSync ? wx.getEnterOptionsSync() : (wx.getLaunchOptionsSync ? wx.getLaunchOptionsSync() : {});
      return this.handleLaunchOptions(launch);
    } catch (error) {
      // 启动参数读取失败不影响正常进入首页。
      return false;
    }
  }

  handleLaunchOptions(options = {}) {
    try {
      const query = options && options.query && typeof options.query === 'object' ? options.query : {};
      const signature = [query.mode, query.room, query.room_code, query.seed, query.room_seed, query.debug, query.diagnostics]
        .map((value) => String(value || '')).join('|');
      if (signature && signature === this.lastLaunchSignature) return false;
      if (String(query.debug || query.diagnostics || '') === '1') {
        this.diagnosticsEnabled = true;
        this.popup = 'diagnostics';
      }
      const invite = shareService.parseLaunchParams(query);
      if (invite.mode !== 'friend' || !invite.room_code) return;
      if (this.screen === 'game' && this.mode !== 'friend') {
        this.triggerFeedback('info', '当前游戏进行中，请结束后再加入好友房间');
        return false;
      }
      if (this.screen === 'game' && this.mode === 'friend' && this.friendRoom
        && String(this.friendRoom.room_code || '') !== String(invite.room_code)) {
        this.triggerFeedback('info', '当前正在进行好友对战，未切换到新房间');
        return false;
      }
      this.lastLaunchSignature = signature;
      this.gameRequestToken += 1;
      this.friendRoom = friendMatch.createLocalRoom(invite.room_code);
      if (invite.room_seed > 0) this.friendRoom.room_seed = invite.room_seed;
      this.friendRules = friendMatch.rules();
      this.friendRoomFromInvite = true;
      this.friendLobbyView = 'room';
      this.friendSelfReady = false;
      this.friendServerStartAt = 0;
      this.friendLocalFallback = !(apiClient.isConfigured && apiClient.isConfigured());
      this.friendRoomBackendStatus = this.friendLocalFallback ? 'local' : 'idle';
      this.screen = 'friend_lobby';
      // 分享进入要先让玩家看到房间，首次引导回到首页后再展示，不能盖住好友房间。
      this.popup = '';
      if (this.backendAuth && this.backendAuth.status === 'ready') this.joinBackendFriendRoom();
      return true;
    } catch (error) {
      // 启动参数读取失败不影响正常进入首页。
      return false;
    }
  }

  makeStars() {
    const result = [];
    for (let i = 0; i < 86; i += 1) {
      result.push({
        x: Math.random(),
        y: Math.random(),
        r: 0.8 + Math.random() * 1.8,
        alpha: 0.25 + Math.random() * 0.58,
        phase: Math.random() * Math.PI * 2,
        speed: 0.28 + Math.random() * 0.92,
      });
    }
    return result;
  }

  makeFloatNumbers() {
    return [
      ['2', 0.055, 0.195, 64, -0.05],
      ['+', 0.49, 0.095, 54, 0],
      ['×', 0.875, 0.19, 58, -0.08],
      ['4', 0.92, 0.255, 60, 0.04],
      ['+', 0.785, 0.305, 44, 0.11],
      ['+', 0.06, 0.44, 44, -0.08],
      ['−', 0.18, 0.46, 46, -0.09],
      ['÷', 0.79, 0.47, 48, 0],
      ['8', 0.91, 0.48, 66, 0],
    ].map(([text, x, y, size, rotate], index) => ({ text, x, y, size, rotate, phase: index * 0.74 }));
  }

  loop() {
    const now = Date.now();
    const delta = Math.min(0.1, (now - this.lastFrame) / 1000);
    this.lastFrame = now;
    try {
      this.syncDateScopedState();
      this.syncAudioScene();
      this.updateFriendMatchmaking();
      this.updateFriendCountdown();
      this.updateFriendReconnect();
      if (this.mode === 'friend') this.pollBackendFriendRoom();
      if (this.screen === 'game' && !this.gamePaused && !this.friendCountdownActive && !this.transitioning) {
        // 真机上偶尔会丢失最后一次触摸回调，导致画面停在只剩 24 的状态。
        // 每帧检查最终状态，保证核心玩法不依赖某一个事件或定时器。
        const solved = this.checkSolvedState();
        if (!solved && this.screen === 'game' && !this.transitioning) {
          if (this.mode === 'friend') {
            const opponent = this.friendOpponentState();
            if (opponent.finished) {
              this.finish(false, '对手已完成全部题目');
            }
          }
        }
        if (this.screen === 'game' && !solved && !this.transitioning) {
          this.timeLeft -= delta;
          this.audio.updateCountdown(this.timeLeft);
          if (this.timeLeft <= 0) this.finish(false, '时间到啦');
        }
      }
      // 自动跳题同时由 setTimeout 和主循环兜底。某些微信运行环境会延迟
      // setTimeout，但主循环仍会继续运行，因此不能只依赖定时器回调。
      if (this.screen === 'game' && !this.gamePaused && this.transitioning && !this.settling && this.autoNextAt > 0 && now >= this.autoNextAt) {
        this.nextQuestion();
      }
      this.draw(now / 1000);
    } catch (error) {
      this.handleRuntimeError(error, 'loop');
    } finally {
      // 任何一帧绘制或结算异常都不能让小游戏主循环永久停止。
      setTimeout(this.loop, 16);
    }
  }

  syncAudioScene() {
    if (!this.audio || !this.audio.setMusicScene) return;
    const nextScene = this.screen === 'game' ? 'game' : 'home';
    if (this.audioScene === nextScene) return;
    this.audioScene = nextScene;
    this.audio.setMusicScene(nextScene);
  }

  handleRuntimeError(error, stage = 'runtime') {
    const message = error && (error.stack || error.message) ? String(error.stack || error.message) : String(error || 'unknown error');
    this.lastRuntimeError = { stage, message: message.slice(0, 500), time: Date.now() };
    try { storage.appendErrorLog(stage, error, { mode: this.mode, screen: this.screen }); } catch (logError) { /* 日志失败不能影响恢复 */ }
    try { if (typeof console !== 'undefined' && console.error) console.error(`[24点挑战][${stage}]`, error); } catch (logError) { /* 静默降级 */ }

    const solved = this.screen === 'game' && Array.isArray(this.cards) && this.cards.length === 1
      && this.cards[0] && Math.abs(Number(this.cards[0].value) - 24) < 0.000001;
    if (solved && this.transitioning) {
      // 如果异常发生在答对后的短暂过渡中，下一帧继续走自动跳题兜底。
      this.autoNextAt = Date.now();
      this.status = '答对啦！正在进入下一题';
      return;
    }

    this.transitioning = false;
    this.autoNextAt = 0;
    this.autoNextFallbackMs = 0;
    if (this.screen === 'game' || this.screen === 'result') {
      this.result = this.result || {
        passed: solved,
        score: safeNumber(this.score),
        stars: solved ? 1 : 0,
        starDetails: [],
        combo: safeNumber(this.maxCombo),
        mistakes: safeNumber(this.mistakes),
        reason: solved ? '本题已完成' : '页面发生异常，请重新开始',
        rewardCoins: 0,
        bonusLabels: [],
        levelComplete: false,
        next: false,
      };
      this.screen = 'result';
      // 如果是绘制阶段出错，使用极简恢复画面，避免旧画面看起来像死机。
      this.renderRecovery = true;
    }
  }

  drawRuntimeRecovery() {
    this.buttons = [];
    const width = safeNumber(this.width, 750);
    const height = safeNumber(this.height, 1334);
    try {
      const ctx = this.ctx;
      if (ctx.setTransform) ctx.setTransform(this.dpr || 1, 0, 0, this.dpr || 1, 0, 0);
      ctx.fillStyle = '#090735';
      ctx.fillRect(0, 0, width, height);
      ctx.fillStyle = '#ffffff';
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.font = 'bold 34px sans-serif';
      ctx.fillText('本局可以继续', width / 2, height * 0.34);
      ctx.font = '20px sans-serif';
      ctx.fillStyle = '#8cf6ff';
      ctx.fillText('点击“重新开始”恢复游戏', width / 2, height * 0.42);
    } catch (drawError) {
      try { if (typeof console !== 'undefined' && console.error) console.error('[24点挑战][recovery-draw]', drawError); } catch (logError) { /* 静默降级 */ }
    }
    this.addHitArea(48, height * 0.52, width - 96, 72, () => this.restartMode(), { key: 'runtime-restart' });
    this.addHitArea(48, height * 0.62, width - 96, 72, () => this.goHome(), { key: 'runtime-home' });
  }

  activeSkinId() {
    if (this.previewSkinId && this.previewSkinUntil > Date.now()) return this.previewSkinId;
    if (this.previewSkinId && this.previewSkinUntil <= Date.now()) this.clearSkinPreview(false);
    return String(this.progress && this.progress.equipped_skin || 'classic');
  }

  formatDailyRuleTitle(title) {
    return String(title || '').replace(/^今日规则[:：]\s*/, '');
  }

  campaignBlockGateScore() {
    return 6000;
  }

  campaignBlockScore(blockIndex, progress = this.progress) {
    const block = Math.max(0, Math.floor(Number(blockIndex) || 0));
    const start = block * 100;
    const end = Math.min(200, start + 100);
    let total = 0;
    for (let index = start; index < end; index += 1) {
      const record = progress && progress.levels && progress.levels[String(index)];
      total += clamp(Math.floor(safeNumber(record && record.best_score, 0)), 0, 100);
    }
    return total;
  }

  isCampaignBlockUnlocked(blockIndex) {
    const block = Math.max(0, Math.floor(Number(blockIndex) || 0));
    if (block <= 0) return true;
    // 已经进入过下一大章节的旧存档不回锁，避免升级规则影响已有玩家。
    if (safeNumber(this.progress && this.progress.unlocked_level) > block * 100) return true;
    return this.campaignBlockScore(block - 1) >= this.campaignBlockGateScore();
  }

  isCampaignLevelUnlocked(index) {
    const levelIndex = Math.floor(Number(index));
    if (!Number.isFinite(levelIndex) || levelIndex < 0 || levelIndex >= 200) return false;
    return levelIndex <= safeNumber(this.progress && this.progress.unlocked_level)
      && this.isCampaignBlockUnlocked(Math.floor(levelIndex / 100));
  }

  highestPlayableLevelNumber() {
    let highest = 1;
    for (let index = 0; index < 200; index += 1) {
      if (this.isCampaignLevelUnlocked(index)) highest = index + 1;
      else if (index > safeNumber(this.progress && this.progress.unlocked_level)) break;
    }
    return highest;
  }

  campaignProgressScore(solvedQuestions = 0) {
    const totalQuestions = Math.max(1, Array.isArray(this.puzzles) ? this.puzzles.length : 3);
    const solved = clamp(Math.floor(Number(solvedQuestions) || 0), 0, totalQuestions);
    const base = (100 * solved) / totalQuestions;
    const mistakePenalty = Math.max(0, Math.floor(safeNumber(this.mistakes))) * 20;
    const hintPenalty = this.hintUsed ? 10 : 0;
    return clamp(Math.round(base - mistakePenalty - hintPenalty), 0, 100);
  }

  equippedCosmetic(category) {
    const equipped = this.progress && this.progress.equipped_cosmetics || {};
    const fallback = { card: 'card_classic', operator: 'operator_classic', result: 'result_classic' }[category];
    return skinCatalog.getCosmetic(equipped[category] || fallback);
  }

  startSkinPreview(skin) {
    if (!skin || !skin.id || this.shopActionInFlight) return;
    if (!this.previewSkinId) this.previewSkinPrevious = String(this.progress && this.progress.equipped_skin || 'classic');
    this.previewSkinId = skin.id;
    this.previewSkinUntil = Date.now() + 10000;
    this.shopNotice = `正在试用「${skin.name}」· 10 秒后恢复`;
    this.triggerFeedback('info', `已开始试用「${skin.name}」`);
  }

  clearSkinPreview(showNotice = true) {
    const previous = this.previewSkinPrevious;
    this.previewSkinId = '';
    this.previewSkinUntil = 0;
    this.previewSkinPrevious = '';
    if (showNotice && previous) this.shopNotice = '主题试用已结束，已恢复已装备主题';
  }

  clear() {
    if (this.ctx.setTransform) this.ctx.setTransform(1, 0, 0, 1, 0, 0);
    this.ctx.fillStyle = GAME_UI.bgTop;
    this.ctx.fillRect(0, 0, this.canvas.width, this.canvas.height);
    this.applyRenderTransform();
    const theme = skinCatalog.getSkin(this.activeSkinId()).theme || {};
    const gradient = this.ctx.createLinearGradient(0, 0, 0, this.height);
    gradient.addColorStop(0, this.screen === 'home' ? (theme.bg || '#070637') : (theme.bg || GAME_UI.bgTop));
    gradient.addColorStop(0.35, theme.surface_2 || GAME_UI.bgMid);
    gradient.addColorStop(0.65, theme.surface || '#15125D');
    gradient.addColorStop(1, theme.bg || GAME_UI.bgBottom);
    this.ctx.fillStyle = gradient;
    this.ctx.fillRect(0, 0, this.width, this.height);
    const centerGlow = this.ctx.createRadialGradient(this.width * 0.5, this.height * 0.30, 18, this.width * 0.5, this.height * 0.30, 560);
    centerGlow.addColorStop(0, 'rgba(38,60,195,0.30)');
    centerGlow.addColorStop(0.46, 'rgba(25,30,145,0.16)');
    centerGlow.addColorStop(1, 'rgba(10,8,64,0)');
    this.ctx.fillStyle = centerGlow;
    this.ctx.fillRect(0, 0, this.width, this.height);
    const lowerGlow = this.ctx.createRadialGradient(this.width * 0.56, this.height * 0.82, 40, this.width * 0.56, this.height * 0.82, 520);
    lowerGlow.addColorStop(0, 'rgba(87,42,190,0.20)');
    lowerGlow.addColorStop(0.62, 'rgba(17,13,92,0.05)');
    lowerGlow.addColorStop(1, 'rgba(10,8,64,0)');
    this.ctx.fillStyle = lowerGlow;
    this.ctx.fillRect(0, 0, this.width, this.height);
  }

  drawLegacy(time) {
    if (this.renderRecovery) {
      this.drawRuntimeRecovery();
      return;
    }
    try {
      screenRenderer.renderFrame(this, time);
      return;
    this.clear();
    // 每帧重新登记当前页面的触摸区域，避免关闭设置后仍能拖动上一帧的音量条。
    this.volumeDragAreas = {};
    this.drawStars(time);
    if (this.screen === 'home') this.drawHome(time);
    else if (this.screen === 'levels') this.drawLevels();
    else if (this.screen === 'game') this.drawGame();
    else if (this.screen === 'result') this.drawResult();
    else if (this.screen === 'friend_matchmaking') this.drawFriendMatchmaking();
    else if (this.screen === 'friend_lobby') this.drawFriendLobby();
    else if (this.screen === 'shop') this.drawShop();
    else if (this.screen === 'achievements') this.drawAchievements();
    else if (this.screen === 'leaderboard') this.drawLeaderboard();
    else if (this.screen === 'records') this.drawRecords();
    if (this.screen === 'game' && this.friendCountdownActive) this.drawFriendCountdown();
    if (this.popup) this.drawPopup();
    if (this.hintPopup) this.drawHintPopup();
    if (this.resultHelpPopup) this.drawResultHelpPopup();
    if ((this.screen === 'game' || this.screen === 'result' || this.screen === 'friend_lobby')
      && (this.friendConnectionState === 'reconnecting' || this.friendConnectionState === 'reconnect_timeout' || this.friendRoomExpired)) {
      this.drawFriendConnectionOverlay();
    }
    this.drawFeedback();
    this.drawTouchEffect(time);
    } catch (error) {
      this.handleRuntimeError(error, 'draw');
      this.drawRuntimeRecovery();
    }
  }

  drawFriendConnectionOverlay() {
    const expired = this.friendRoomExpired || this.friendConnectionState === 'expired';
    const timedOut = this.friendConnectionState === 'reconnect_timeout';
    const width = Math.min(610, this.width - 64);
    const height = expired ? 366 : 390;
    const x = (this.width - width) / 2;
    const y = this.modalTop(height);
    this.ctx.save();
    this.ctx.fillStyle = 'rgba(2,3,24,0.82)';
    this.ctx.fillRect(0, 0, this.width, this.height);
    this.drawGamePanel(x, y, width, height, expired ? 'violet' : 'cyan', {
      radius: 30,
      shadowColor: expired ? 'rgba(160,100,255,0.40)' : 'rgba(40,233,255,0.34)',
      shadowBlur: 28,
      shadowOffsetY: 8,
      stroke: expired ? 'rgba(190,180,255,0.76)' : 'rgba(104,244,255,0.78)',
    });
    const title = expired ? (this.friendRoomError || '房间已过期') : timedOut ? '连接超时' : '正在重新连接';
    const subtitle = expired
      ? '请重新创建房间，或返回好友对战入口'
      : timedOut
        ? '网络暂时不可用，对局没有被判负'
        : '正在恢复房间状态，对局不会立即结束';
    this.drawFitText(title, this.width / 2, y + 68, width - 72, uiFont(29, 900), expired ? GAME_UI.violetLight : GAME_UI.cyanLight);
    this.drawFitText(subtitle, this.width / 2, y + 113, width - 72, uiFont(16, 600), GAME_UI.secondary);
    if (!expired) {
      const elapsed = Math.max(0, Math.floor((Date.now() - Number(this.friendReconnectStartedAt || Date.now())) / 1000));
      const left = Math.max(0, 15 - elapsed);
      this.drawGamePanel(x + 34, y + 146, width - 68, 78, 'dark', { radius: 22, shadow: false, stroke: 'rgba(40,233,255,0.30)' });
      this.drawFitText(timedOut ? '可以点击下方按钮继续' : `自动重试中 · ${left} 秒`, this.width / 2, y + 185, width - 106, uiFont(20, 800), GAME_UI.text);
    } else {
      this.drawGamePanel(x + 34, y + 146, width - 68, 78, 'dark', { radius: 22, shadow: false, stroke: 'rgba(190,180,255,0.30)' });
      this.drawFitText('本局状态已安全保存', this.width / 2, y + 185, width - 106, uiFont(19, 800), GAME_UI.secondary);
    }
    const firstY = y + height - 116;
    const secondY = y + height - 54;
    if (expired) {
      this.drawNeonButton(x + 30, firstY, width - 60, 50, '重新创建房间', () => this.showFriendRoom('create'), 'cyan', { fontSize: 18, radius: 21, key: 'friend-expired-create' });
    } else {
      this.drawNeonButton(x + 30, firstY, width - 60, 50, '立即重连', () => this.retryFriendConnection(), 'cyan', { fontSize: 18, radius: 21, key: 'friend-reconnect' });
    }
    this.drawNeonButton(x + 30, secondY, width - 60, 44, expired ? '返回好友对战' : '退出对战', () => this.showFriendLobby(), 'violet', { fontSize: 17, radius: 19, key: 'friend-connection-exit' });
    this.ctx.restore();
  }

  drawFriendCountdown() {
    const remainingMs = Math.max(0, Number(this.friendCountdownUntil || 0) - Date.now());
    const number = Math.min(3, Math.max(1, Math.ceil(remainingMs / 1000)));
    this.ctx.save();
    this.ctx.fillStyle = 'rgba(4,3,32,0.72)';
    this.ctx.fillRect(0, 0, this.width, this.height);
    const width = Math.min(560, this.width - 96);
    const x = (this.width - width) / 2;
    const y = this.modalTop(300);
    this.drawGamePanel(x, y, width, 300, 'magenta', {
      radius: 30,
      shadowColor: 'rgba(255,80,205,0.34)',
      shadowBlur: 24,
      shadowOffsetY: 0,
      stroke: 'rgba(255,153,226,0.72)',
    });
    this.drawFitText('\u51c6\u5907\u5f00\u59cb', this.width / 2, y + 58, width - 70, uiFont(24, 900), GAME_UI.magentaLight);
    this.ctx.save();
    this.ctx.shadowColor = 'rgba(40,233,255,0.72)';
    this.ctx.shadowBlur = 24;
    this.drawText(String(number), this.width / 2, y + 158, uiFont(84, 900), GAME_UI.cyanLight);
    this.ctx.restore();
    this.drawFitText('\u53cc\u65b9\u4f7f\u7528\u540c\u4e00\u7ec4\u9898\u76ee', this.width / 2, y + 238, width - 80, uiFont(16, 600), GAME_UI.secondary);
    this.ctx.restore();
  }

  drawFeedback() {
    if (!this.feedback || Date.now() > this.feedback.until) return;
    const remaining = clamp((this.feedback.until - Date.now()) / 650, 0, 1);
    const color = this.feedback.type === 'success' ? COLORS.green : this.feedback.type === 'error' ? COLORS.danger : COLORS.cyan;
    this.ctx.save();
    this.ctx.globalAlpha = remaining;
    const toastY = this.visibleBottom(106);
    this.drawPanel(148, toastY, 424, 58, 'rgba(13,11,43,0.94)', `${color}bb`, 22, { shadowColor: `${color}55`, shadowBlur: 16 });
    this.drawFitText(this.feedback.text, this.width / 2, toastY + 29, 382, uiFont(17, 800), color);
    this.ctx.restore();
  }

  drawHintPopup() {
    const hint = this.hintPopup;
    if (!hint) return;
    this.ctx.save();
    this.ctx.fillStyle = 'rgba(2,3,24,0.78)';
    this.ctx.fillRect(0, 0, this.width, this.height);
    const width = Math.min(610, this.width - 72);
    const height = 360;
    const x = (this.width - width) / 2;
    const y = this.modalTop(height);
    this.drawModalFrame(x, y, width, height, '第一步提示', '点击屏幕任意位置关闭', 'magenta');
    this.drawGamePanel(x + 34, y + 132, width - 68, 132, 'cyan', {
      radius: 24,
      shadowColor: 'rgba(40,233,255,0.30)',
      shadowBlur: 18,
      shadowOffsetY: 0,
    });
    const firstLabel = hint.firstIndex === undefined ? `数字 ${hint.first}` : `第 ${hint.firstIndex + 1} 张 · ${hint.first}`;
    const secondLabel = hint.secondIndex === undefined ? `数字 ${hint.second}` : `第 ${hint.secondIndex + 1} 张 · ${hint.second}`;
    const operator = hint.operator || '运算符';
    this.drawFitText(`先点击 ${firstLabel}`, this.width / 2, y + 170, width - 102, uiFont(21, 900), GAME_UI.text);
    this.drawFitText(`再点击 ${operator}，最后点击 ${secondLabel}`, this.width / 2, y + 216, width - 102, uiFont(18, 800), GAME_UI.cyanLight);
    this.drawFitText('提示只告诉你第一步，后面的计算自己完成', this.width / 2, y + 300, width - 90, uiFont(14, 500), GAME_UI.secondary);
    this.ctx.restore();
  }

  drawResultHelpPopup() {
    this.addHitArea(0, 0, this.width, this.height, () => { this.resultHelpPopup = false; }, { key: 'result-help-overlay' });
    this.ctx.save();
    this.ctx.fillStyle = 'rgba(2,3,24,0.78)';
    this.ctx.fillRect(0, 0, this.width, this.height);
    const width = Math.min(610, this.width - 72);
    const height = 432;
    const x = (this.width - width) / 2;
    const y = this.modalTop(height);
    this.drawModalFrame(x, y, width, height, '本局说明', '点击任意位置关闭', 'violet', () => { this.resultHelpPopup = false; });
    const result = this.result || {};
    const lines = [
      `本局结果：${result.passed ? '挑战成功' : '时间结束或未得到 24'}`,
      '每个数字必须使用且只能使用一次。',
      '除法不能除以 0，第一版只允许整数结果。',
      this.mode === 'daily' ? '今日挑战还附带了专属规则，请按规则作答。' : '先点数字，再点运算符，最后点第二个数字。',
    ];
    lines.forEach((line, index) => this.drawFitText(line, x + 34, y + 146 + index * 42, width - 68, uiFont(index === 0 ? 20 : 16, index === 0 ? 900 : 500), index === 0 ? GAME_UI.gold : GAME_UI.text, 'left'));
    this.drawNeonButton(x + 42, y + height - 78, width - 84, 52, '知道了', () => { this.resultHelpPopup = false; }, 'cyan', { fontSize: 18, radius: 18, key: 'result-help-ok' });
    this.ctx.restore();
  }

  checkSolvedState() {
    if (this.screen !== 'game' || this.transitioning || this.settling || !Array.isArray(this.cards) || this.cards.length !== 1) return false;
    const onlyCard = this.cards[0];
    if (!onlyCard || !Number.isFinite(Number(onlyCard.value)) || Math.abs(Number(onlyCard.value) - 24) > 0.000001) return false;
    const rules = (this.currentPuzzle && this.currentPuzzle.rules) || {};
    const requiredOperator = rules.requiredOperator || rules.required_operator || '';
    const usedOperators = Array.isArray(this.questionOperators) ? this.questionOperators : [];
    if (requiredOperator && !usedOperators.includes(requiredOperator)) return false;
    this.selectedIndex = -1;
    this.selectedOperator = '';
    const isLastQuestion = this.mode === 'campaign' || this.mode === 'daily'
      ? this.currentQuestion >= (Array.isArray(this.puzzles) ? this.puzzles.length - 1 : 0)
      : this.mode === 'friend'
        ? this.currentQuestion >= this.friendQuestionCount() - 1
        : false;
    // 纯逻辑测试/服务端校验对象没有主循环，保持同步语义；真机 GameApp
    // 始终有 loop，会走下面的异步轻量结算，避免点击回调卡顿。
    if (!Object.prototype.hasOwnProperty.call(this, 'loop') && isLastQuestion) {
      this.finish(true, '完成本题');
      return true;
    }
    // 最后一笔运算只负责确认“已经得到 24”，把存档、任务、排行榜和结算
    // 放到下一轮事件循环，避免微信 Canvas 在点击回调里同步做大量工作而短暂卡顿。
    this.settling = true;
    this.transitioning = true;
    const settleToken = ++this.settleToken;
    this.autoNextAt = Date.now() + 260;
    this.status = '答对啦！正在进入下一步';
    setTimeout(() => {
      if (settleToken !== this.settleToken || this.screen !== 'game' || !this.settling) return;
      this.settling = false;
      this.transitioning = false;
      this.finish(true, '完成本题');
    }, 0);
    return true;
  }

  drawTouchEffect(time) {
    if (!this.touchEffect) return;
    const age = time - this.touchEffect.time;
    if (age < 0 || age > 0.32) { if (age > 0.32) this.touchEffect = null; return; }
    const progress = age / 0.32;
    this.ctx.save();
    this.ctx.globalAlpha = (1 - progress) * 0.72;
    this.ctx.strokeStyle = this.touchEffect.color || COLORS.cyan;
    this.ctx.lineWidth = 4;
    this.ctx.beginPath();
    this.ctx.arc(this.touchEffect.x, this.touchEffect.y, 12 + progress * 34, 0, Math.PI * 2);
    this.ctx.stroke();
    this.ctx.restore();
  }

  triggerFeedback(type, text) {
    this.feedback = { type, text, until: Date.now() + 650 };
  }

  invokeTouchAction(action, key = 'action') {
    if (typeof action !== 'function') return false;
    try {
      const result = action();
      if (result && typeof result.catch === 'function') {
        result.catch((error) => this.handleTouchError(error, key));
      }
      return true;
    } catch (error) {
      this.handleTouchError(error, key);
      return false;
    }
  }

  sharePayload(payload) {
    this.status = '正在打开分享面板…';
    return platform.share(payload).then((success) => {
      this.triggerFeedback(success ? 'success' : 'error', success ? '分享面板已打开' : '分享暂时不可用');
      this.status = success ? '可以选择好友分享' : '分享暂时不可用，请稍后再试';
      return success;
    }).catch((error) => {
      this.status = '分享暂时不可用，请稍后再试';
      this.triggerFeedback('error', this.status);
      try { if (typeof console !== 'undefined' && console.warn) console.warn('[share]', error); } catch (logError) { /* 分享失败不影响游戏 */ }
      return false;
    });
  }

  handleTouchError(error, key = 'action') {
    const message = String(error && (error.stack || error.message) || error || 'unknown touch error');
    this.lastRuntimeError = { stage: `touch:${key}`, message: message.slice(0, 500), time: Date.now() };
    try { storage.appendErrorLog(`touch:${key}`, error, { mode: this.mode, screen: this.screen }); } catch (logError) { /* 日志失败不能影响按钮 */ }
    try { if (typeof console !== 'undefined' && console.error) console.error(`[24点挑战][touch:${key}]`, error); } catch (logError) { /* 静默降级 */ }
    this.status = '操作暂时失败，请再试一次';
    this.triggerFeedback('error', this.status);
  }

  showRewarded(rewardType, onSuccess) {
    if (!this.ads.isAvailable()) {
      this.status = '今日激励次数已用完，明天再来吧';
      return;
    }
    this.status = '正在准备激励广告…';
    this.ads.showRewarded(rewardType, platform).then((success) => {
      if (!success) { this.status = '广告暂未准备好，本次不消耗次数'; return; }
      this.progress.ads = this.ads.usage();
      storage.save(this.progress);
      onSuccess();
    }).catch((error) => {
      this.status = '广告暂未准备好，本次不消耗次数';
      try { if (typeof console !== 'undefined' && console.warn) console.warn('[rewarded-ad]', error); } catch (logError) { /* 广告失败不影响游戏 */ }
    });
  }

  drawStars(time) {
    const ctx = this.ctx;
    if (this.screen === 'home') {
      this.drawHomeBackdropDetails(time);
      return;
    }
    this.stars.forEach((star) => {
      const alpha = (star.alpha || 0.45) * (0.60 + 0.30 * Math.abs(Math.sin(time * star.speed + star.phase)));
      ctx.save();
      ctx.globalAlpha = alpha;
      ctx.fillStyle = '#d9edff';
      ctx.shadowColor = 'rgba(120,160,255,0.45)';
      ctx.shadowBlur = 4;
      ctx.beginPath();
      ctx.arc(star.x * this.width, star.y * this.height, Math.max(0.8, star.r * 0.78), 0, Math.PI * 2);
      ctx.fill();
      ctx.restore();
    });
    this.floatNumbers.forEach((item) => {
      ctx.save();
      ctx.globalAlpha = 0.12;
      ctx.fillStyle = 'rgba(115,110,230,0.60)';
      ctx.font = scaleFont(uiFont(item.size || 58, 900));
      ctx.translate(item.x * this.width, (item.y + Math.sin(time * 0.45 + item.phase) * 0.006) * this.height);
      ctx.rotate(item.rotate || 0);
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillText(item.text, 0, 0);
      ctx.restore();
    });
    ctx.globalAlpha = 1;
  }

  drawHomeBackdropDetails(time) {
    const ctx = this.ctx;
    this.stars.forEach((star, index) => {
      const twinkle = 0.68 + 0.32 * Math.sin(time * star.speed + star.phase);
      const x = star.x * this.width;
      const y = star.y * this.height;
      ctx.save();
      ctx.globalAlpha = star.alpha * twinkle;
      ctx.fillStyle = index % 7 === 0 ? '#fff8ff' : '#dce9ff';
      ctx.shadowColor = index % 7 === 0 ? 'rgba(255,255,255,0.85)' : 'rgba(90,180,255,0.55)';
      ctx.shadowBlur = index % 9 === 0 ? 8 : 3;
      if (index % 11 === 0) {
        this.drawSparkle(x, y, 4 + star.r * 2, '#ffffff', ctx.globalAlpha);
      } else {
        ctx.beginPath();
        ctx.arc(x, y, star.r, 0, Math.PI * 2);
        ctx.fill();
      }
      ctx.restore();
    });

    ctx.save();
    ctx.scale(1, this.homeYScale || 1);
    this.floatNumbers.forEach((item) => {
      const x = item.x * 750;
      const y = (item.y + Math.sin(time * 0.45 + item.phase) * 0.007) * 1584;
      ctx.save();
      ctx.translate(x, y);
      ctx.rotate(item.rotate + Math.sin(time * 0.35 + item.phase) * 0.02);
      ctx.globalAlpha = 0.12;
      ctx.fillStyle = 'rgba(115,110,235,0.62)';
      ctx.font = scaleFont(uiFont(item.size, 900));
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillText(item.text, 0, 0);
      ctx.restore();
    });

    this.drawThinOrbit(104, 560, 365, 93, -0.22, 'rgba(28,194,255,0.78)', 'rgba(28,194,255,0.32)');
    this.drawThinOrbit(285, 452, 270, 76, -0.27, 'rgba(56,91,255,0.44)', 'rgba(68,80,255,0.18)');
    this.drawThinOrbit(442, 435, 240, 72, -0.55, 'rgba(104,45,255,0.30)', 'rgba(104,45,255,0.12)');
    ctx.restore();
  }

  addButton(x, y, width, height, label, action, options = {}) {
    let variant = options.variant || 'violet';
    const fillText = String(options.fill || '').toLowerCase();
    if (!options.variant && (fillText.includes('3d8694') || fillText.includes('365f98') || fillText.includes('3e668f') || fillText.includes('4f7d8f'))) variant = 'cyan';
    if (!options.variant && (fillText.includes('a04e79') || fillText.includes('754f88') || fillText.includes('73528f') || fillText.includes('b14f82'))) variant = 'magenta';
    if (!options.variant && (fillText.includes('ffd') || fillText.includes('gold'))) variant = 'gold';
    this.drawNeonButton(x, y, width, height, label, action, variant, {
      disabled: options.disabled,
      fontSize: options.fontSize || 22,
      radius: options.radius || Math.min(24, height / 2),
      key: options.key || label,
      textColor: options.textColor,
      shadowBlur: options.shadowBlur,
    });
  }

  addHitArea(x, y, width, height, action, options = {}) {
    this.buttons.push({ x, y, width, height, action, disabled: Boolean(options.disabled), key: options.key || '', dragType: options.dragType || '' });
  }

  drawPanel(x, y, width, height, fill, stroke = 'rgba(255,255,255,0.16)', radius = 24, options = {}) {
    this.ctx.save();
    if (options.shadow !== false) {
      this.ctx.shadowColor = options.shadowColor || 'rgba(4, 2, 25, 0.34)';
      this.ctx.shadowBlur = options.shadowBlur || 14;
      this.ctx.shadowOffsetY = options.shadowOffsetY || 4;
    }
    this.roundRect(x, y, width, height, radius, fill, stroke, options.lineWidth || 2);
    this.ctx.restore();
  }

  drawText(text, x, y, font, color = COLORS.text, align = 'center', baseline = 'middle') {
    this.ctx.save();
    this.ctx.fillStyle = color;
    this.ctx.font = scaleFont(font);
    this.ctx.textAlign = align;
    this.ctx.textBaseline = baseline;
    this.ctx.fillText(text, x, y);
    this.ctx.restore();
  }

  drawFitText(text, x, y, maxWidth, font, color = COLORS.text, align = 'center', baseline = 'middle') {
    const value = String(text ?? '');
    const baseFont = scaleFont(font);
    const match = baseFont.match(/^(.*?)(\d+(?:\.\d+)?)px(.*)$/);
    if (!match || !this.ctx.measureText) {
      this.drawText(value, x, y, font, color, align, baseline);
      return;
    }
    const baseSize = Number(match[2]);
    this.ctx.save();
    this.ctx.font = baseFont;
    const measured = this.ctx.measureText(value).width;
    const ratio = measured > maxWidth ? clamp(maxWidth / Math.max(1, measured), 0.48, 1) : 1;
    const measureFont = resizeFont(baseFont, baseSize * ratio);
    this.ctx.font = measureFont;
    let output = value;
    if (this.ctx.measureText(output).width > maxWidth && output.length > 1) {
      const ellipsis = '…';
      let low = 0;
      let high = output.length;
      while (low < high) {
        const mid = Math.ceil((low + high) / 2);
        const candidate = `${output.slice(0, mid)}${ellipsis}`;
        if (this.ctx.measureText(candidate).width <= maxWidth) low = mid;
        else high = mid - 1;
      }
      output = `${output.slice(0, Math.max(1, low))}${ellipsis}`;
    }
    this.ctx.restore();
    this.drawText(output, x, y, resizeFont(font, (baseSize * ratio) / FONT_SCALE), color, align, baseline);
  }

  drawSparkle(x, y, size, color = '#ffffff', alpha = 1) {
    const ctx = this.ctx;
    ctx.save();
    ctx.globalAlpha *= alpha;
    ctx.fillStyle = color;
    ctx.shadowColor = color;
    ctx.shadowBlur = size * 1.2;
    ctx.beginPath();
    ctx.moveTo(x, y - size);
    ctx.quadraticCurveTo(x + size * 0.22, y - size * 0.22, x + size, y);
    ctx.quadraticCurveTo(x + size * 0.22, y + size * 0.22, x, y + size);
    ctx.quadraticCurveTo(x - size * 0.22, y + size * 0.22, x - size, y);
    ctx.quadraticCurveTo(x - size * 0.22, y - size * 0.22, x, y - size);
    ctx.closePath();
    ctx.fill();
    ctx.restore();
  }

  drawThinOrbit(cx, cy, width, height, rotation, stroke, glow) {
    const ctx = this.ctx;
    ctx.save();
    ctx.translate(cx, cy);
    ctx.rotate(rotation);
    ctx.strokeStyle = stroke;
    ctx.lineWidth = 2.4;
    ctx.shadowColor = glow || stroke;
    ctx.shadowBlur = 12;
    ctx.beginPath();
    if (ctx.ellipse) {
      ctx.ellipse(0, 0, width / 2, height / 2, 0, Math.PI * 0.1, Math.PI * 1.72);
    } else {
      ctx.scale(1, height / width);
      ctx.arc(0, 0, width / 2, Math.PI * 0.1, Math.PI * 1.72);
    }
    ctx.stroke();
    ctx.restore();
  }

  drawGlassCard(x, y, width, height, radius, fill, stroke, options = {}) {
    const ctx = this.ctx;
    ctx.save();
    if (options.shadow !== false) {
      ctx.shadowColor = options.shadowColor || 'rgba(0,0,0,0.36)';
      ctx.shadowBlur = options.shadowBlur || 18;
      ctx.shadowOffsetY = options.shadowOffsetY || 6;
    }
    this.roundRect(x, y, width, height, radius, fill, stroke, options.lineWidth || 2);
    ctx.restore();

    if (width > 8 && height > 8) {
      ctx.save();
      ctx.globalAlpha = options.innerAlpha || 0.18;
      this.roundRect(x + 2, y + 2, width - 4, height - 4, Math.max(0, radius - 2), 'rgba(255,255,255,0.018)', 'rgba(255,255,255,0.22)', 1);
      ctx.restore();
    }

    if (options.hotspot) {
      const glow = ctx.createRadialGradient(options.hotspot.x, options.hotspot.y, 2, options.hotspot.x, options.hotspot.y, options.hotspot.r);
      glow.addColorStop(0, options.hotspot.color);
      glow.addColorStop(1, 'rgba(255,255,255,0)');
      ctx.save();
      ctx.globalCompositeOperation = 'lighter';
      ctx.fillStyle = glow;
      this.roundRect(x, y, width, height, radius, glow, null, 0);
      ctx.restore();
    }
  }

  glassFill(x, y, width, height, variant = 'violet') {
    const ctx = this.ctx;
    const fill = ctx.createLinearGradient(x, y, x + width, y + height);
    const variants = {
      cyan: ['rgba(18,86,145,0.66)', 'rgba(13,28,91,0.58)'],
      magenta: ['rgba(119,21,108,0.62)', 'rgba(39,18,83,0.60)'],
      violet: [GAME_UI.panelA, GAME_UI.panelB],
      gold: ['rgba(106,75,29,0.58)', 'rgba(39,24,71,0.58)'],
      dark: ['rgba(16,15,58,0.82)', 'rgba(8,8,36,0.74)'],
      modal: [GAME_UI.modalA, GAME_UI.modalB],
      white: ['#FFFFFF', '#EEF4FF'],
    };
    const colors = variants[variant] || variants.violet;
    fill.addColorStop(0, colors[0]);
    fill.addColorStop(1, colors[1]);
    return fill;
  }

  accentColor(variant = 'violet') {
    return {
      cyan: GAME_UI.cyan,
      magenta: GAME_UI.magenta,
      violet: GAME_UI.violet,
      gold: GAME_UI.gold,
      success: GAME_UI.success,
      dark: 'rgba(150,150,255,0.28)',
    }[variant] || GAME_UI.violet;
  }

  drawGamePanel(x, y, width, height, variant = 'violet', options = {}) {
    const accent = options.stroke || this.accentColor(variant);
    const strokeAlpha = variant === 'dark' ? 'rgba(150,150,255,0.28)' : accent;
    this.drawGlassCard(x, y, width, height, options.radius || GAME_UI.radiusMd, options.fill || this.glassFill(x, y, width, height, variant), strokeAlpha, {
      lineWidth: options.lineWidth || (variant === 'dark' ? 1.2 : 1.6),
      shadow: options.shadow,
      shadowColor: options.shadowColor || (variant === 'cyan' ? 'rgba(0,220,255,0.20)' : variant === 'magenta' ? 'rgba(255,45,190,0.18)' : variant === 'gold' ? 'rgba(255,190,40,0.18)' : 'rgba(130,65,255,0.15)'),
      shadowBlur: options.shadowBlur || 14,
      shadowOffsetY: options.shadowOffsetY === undefined ? 3 : options.shadowOffsetY,
      innerAlpha: options.innerAlpha || 0.10,
      hotspot: options.hotspot,
    });
  }

  drawNeonButton(x, y, width, height, label, action, variant = 'violet', options = {}) {
    const disabled = Boolean(options.disabled);
    const active = this.isHomeButtonActive(options.key || label);
    this.addButtonHit(x, y, width, height, action, { disabled, key: options.key || label });
    const ctx = this.ctx;
    ctx.save();
    ctx.globalAlpha = disabled ? 0.42 : 1;
    if (active) {
      ctx.translate(x + width / 2, y + height / 2);
      ctx.scale(0.97, 0.97);
      ctx.translate(-(x + width / 2), -(y + height / 2));
    }
    const accent = this.accentColor(variant);
    const fill = ctx.createLinearGradient(x, y, x + width, y + height);
    if (variant === 'cyan') {
      fill.addColorStop(0, 'rgba(20,150,190,0.76)');
      fill.addColorStop(1, 'rgba(17,68,145,0.68)');
    } else if (variant === 'magenta') {
      fill.addColorStop(0, 'rgba(184,46,148,0.76)');
      fill.addColorStop(1, 'rgba(96,31,124,0.72)');
    } else if (variant === 'gold') {
      fill.addColorStop(0, 'rgba(255,190,44,0.92)');
      fill.addColorStop(1, 'rgba(198,123,26,0.82)');
    } else {
      fill.addColorStop(0, 'rgba(104,77,199,0.76)');
      fill.addColorStop(1, 'rgba(57,42,132,0.76)');
    }
    this.drawGlassCard(x, y, width, height, options.radius || Math.min(26, height / 2), fill, disabled ? 'rgba(170,160,220,0.26)' : `${accent}bb`, {
      lineWidth: options.lineWidth || 1.6,
      shadowColor: disabled ? 'rgba(0,0,0,0.12)' : `${accent}55`,
      shadowBlur: options.shadowBlur || 13,
      shadowOffsetY: 0,
      innerAlpha: 0.07,
    });
    if (options.icon) {
      this.drawText(options.icon, x + 32, y + height / 2 + 1, uiFont(options.fontSize || 25, 900), options.textColor || GAME_UI.text);
      this.drawFitText(label, x + width / 2 + 14, y + height / 2 + 1, width - 72, uiFont(options.fontSize || 25, 800), options.textColor || GAME_UI.text);
    } else {
      this.drawFitText(label, x + width / 2, y + height / 2 + 1, width - 24, uiFont(options.fontSize || 25, 800), options.textColor || (variant === 'gold' ? '#fffaf0' : GAME_UI.text));
    }
    ctx.restore();
  }

  addButtonHit(x, y, width, height, action, options = {}) {
    this.buttons.push({ x, y, width, height, action, disabled: Boolean(options.disabled), key: options.key || '', dragType: options.dragType || '' });
  }

  pageTop() {
    const scale = Math.max(0.001, this.renderScale);
    // 真机上右上角胶囊会占用从 top 到 bottom 的整段区域。标题栏必须
    // 放在胶囊底部之后，不能只参考胶囊 top，否则返回按钮会和系统菜单重叠。
    const menuBottom = this.menuButton ? (safeNumber(this.menuButton.bottom, 0) - this.renderOffsetY) / scale : 0;
    const safeTop = safeNumber(this.safeTop, 24) / scale;
    const targetTop = menuBottom > 0 ? menuBottom + 14 : safeTop + 10;
    return Math.round(clamp(targetTop, 42, 210));
  }

  // 所有带自定义标题栏的页面都从标题栏下方开始排版，避免小屏手机上
  // 面板直接压到“返回”和标题。这里集中计算，后续页面不要再写固定的 108/118 坐标。
  screenContentTop(gap = 76) {
    return this.pageTop() + gap;
  }

  contentHeight() {
    return Math.max(1280, this.height);
  }

  visibleBottom(padding = 24) {
    const safeBottom = safeNumber(this.safeBottom, 24) / Math.max(0.001, this.renderScale);
    return Math.max(720, this.visibleHeight - safeBottom - padding);
  }

  modalTop(modalHeight, preferred = null) {
    const topLimit = this.pageTop() + 76;
    const bottomLimit = this.visibleBottom(18);
    const centered = preferred === null ? (this.visibleHeight - modalHeight) / 2 : preferred;
    return Math.round(clamp(centered, topLimit, Math.max(topLimit, bottomLimit - modalHeight)));
  }

  gameLayout() {
    const headerY = this.pageTop();
    const statsY = headerY + 76;
    const infoY = statsY + 94;
    const hasInfo = this.mode === 'friend' || this.mode === 'daily';
    const baseContentY = hasInfo ? infoY + 82 : statsY + 94;
    // 375×667 等常见手机在 750 逻辑宽度下可见高度约为 1334，采用紧凑卡片
    // 才能让“运算符/操作按钮/底部导航”依次排列，避免底部按钮压住运算区。
    const compact = this.visibleHeight < 1500;
    const ultraCompact = this.visibleHeight < 1320;
    const cardWidth = compact ? 318 : 326;
    const cardHeight = ultraCompact ? 100 : compact ? 112 : 126;
    const gapX = compact ? 16 : 18;
    const gapY = ultraCompact ? 10 : compact ? 12 : 16;
    // 每日/好友模式上方多一条规则或对战信息，适当上移数字区，
    // 把省出的空间留给更大的卡片和更容易点击的按钮。
    const cardOffset = ultraCompact ? 226 : compact ? (hasInfo ? 226 : 230) : 244;
    const operatorHeight = ultraCompact ? 58 : compact ? 64 : 72;
    const actionHeight = ultraCompact ? 54 : compact ? 56 : 62;
    const bottomButtonHeight = compact ? 58 : 64;
    const cardStartY = baseContentY + cardOffset;
    const cardRows = Math.max(1, Math.ceil(Math.max(1, this.cards ? this.cards.length : 4) / 2));
    const opTitleY = cardStartY + cardHeight * cardRows + gapY + (ultraCompact ? 22 : compact ? 24 : 30);
    const actionY = opTitleY + (ultraCompact ? 84 : compact ? 88 : 110);
    const actionButtonTop = actionY + 22;
    const bottomGap = ultraCompact ? 22 : compact ? 26 : 34;
    const bottomY = actionButtonTop + actionHeight + bottomGap;
    const footerGap = ultraCompact ? 16 : compact ? 20 : 26;
    const footerHeight = compact ? 52 : 62;
    const footerY = bottomY + bottomButtonHeight + footerGap;
    const contentY = baseContentY;
    return {
      headerY,
      statsY,
      infoY,
      contentY,
      bottomY,
      footerY,
      footerHeight,
      cardWidth,
      cardHeight,
      gapX,
      gapY,
      cardStartY,
      cardRows,
      opTitleY,
      actionY,
      operatorHeight,
      actionHeight,
      bottomButtonHeight,
    };
  }

  drawGameHeader(title, backLabel, backAction, rightLabel = '') {
    const top = this.pageTop();
    const buttonY = top;
    const backWidth = backLabel === '‹ 首页' ? 128 : 112;
    this.drawNeonButton(28, buttonY, backWidth, 58, backLabel, backAction, 'violet', { fontSize: 18, radius: 26, key: `header-${title}` });
    this.drawFitText(title, this.width / 2, buttonY + 30, 250, uiFont(30, 800), GAME_UI.text);
    if (rightLabel) this.drawFitText(rightLabel, this.width - 24, buttonY + 30, 150, uiFont(14, 800), GAME_UI.gold, 'right');
  }

  drawStatCard(x, y, width, height, label, value, variant = 'violet') {
    this.drawGamePanel(x, y, width, height, variant, { radius: 18, shadowBlur: 10, shadowOffsetY: 0 });
    const compact = String(value || '').length > 5;
    this.drawFitText(label, x + width / 2, y + 24, width - 14, uiFont(14, 500), GAME_UI.secondary);
    this.drawFitText(value, x + width / 2, y + 62, width - 14, uiFont(compact ? 21 : 25, 900), this.accentColor(variant));
  }

  drawStatsRow(y, cards) {
    const x = GAME_UI.edge;
    const gap = 18;
    const width = (this.width - GAME_UI.edge * 2 - gap * 2) / 3;
    cards.forEach((card, index) => {
      this.drawStatCard(x + index * (width + gap), y, width, 84, card.label, card.value, card.variant);
    });
  }

  drawGameTimer(y, value, danger = false) {
    const x = (this.width - 330) / 2;
    this.drawGamePanel(x, y, 330, 72, 'cyan', {
      radius: 33,
      fill: danger ? 'rgba(105,42,76,0.64)' : 'rgba(7,100,165,0.58)',
      stroke: danger ? GAME_UI.magenta : GAME_UI.cyan,
      shadowColor: danger ? 'rgba(255,80,205,0.40)' : 'rgba(25,220,255,0.40)',
      shadowBlur: 18,
      shadowOffsetY: 0,
    });
    this.drawText(value, this.width / 2, y + 37, uiFont(39, 900), danger ? GAME_UI.gold : GAME_UI.cyanLight);
  }

  drawProgressLine(x, y, width, ratio, variant = 'gold', height = 13) {
    this.drawGamePanel(x, y, width, height, 'dark', {
      radius: 999,
      fill: 'rgba(255,255,255,0.10)',
      stroke: 'rgba(255,255,255,0.13)',
      shadow: false,
      innerAlpha: 0.01,
    });
    const fillWidth = width * clamp(ratio, 0, 1);
    if (fillWidth <= 0) return;
    const gradient = this.ctx.createLinearGradient(x, y, x + fillWidth, y);
    if (variant === 'cyan') {
      gradient.addColorStop(0, GAME_UI.cyan);
      gradient.addColorStop(1, '#5EC5FF');
    } else {
      gradient.addColorStop(0, '#FFD64C');
      gradient.addColorStop(1, '#FFB932');
    }
    this.ctx.save();
    this.ctx.shadowColor = variant === 'cyan' ? 'rgba(0,220,255,0.65)' : 'rgba(255,205,90,0.62)';
    this.ctx.shadowBlur = 7;
    this.roundRect(x, y, Math.max(height, fillWidth), height, 999, gradient, null, 0);
    this.ctx.restore();
  }

  drawQuestionPanel(y, title, progressRatio, hint, detail) {
    const x = GAME_UI.edge;
    const width = this.width - GAME_UI.edge * 2;
    this.drawGamePanel(x, y, width, 138, 'violet', { radius: 24, shadowBlur: 10, shadowOffsetY: 0 });
    this.drawFitText(title, x + 28, y + 33, width - 170, uiFont(24, 900), GAME_UI.text, 'left');
    this.drawGamePanel(x + width - 118, y + 18, 104, 38, 'gold', {
      radius: 19,
      fill: this.glassFill(x + width - 118, y + 18, 104, 38, 'gold'),
      stroke: GAME_UI.gold,
      shadowColor: 'rgba(255,198,65,0.36)',
      shadowBlur: 8,
      shadowOffsetY: 0,
    });
    this.drawText('目标 24', x + width - 66, y + 38, uiFont(16, 900), '#fffaf0');
    this.drawProgressLine(x + 32, y + 65, width - 64, progressRatio, 'gold', 13);
    this.drawFitText(hint, x + 28, y + 98, width - 56, uiFont(15, 500), GAME_UI.secondary, 'left');
    this.drawFitText(detail, x + 28, y + 121, width - 56, uiFont(14, 500), GAME_UI.muted, 'left');
  }

  drawNumberTile(x, y, width, height, value, selected = false) {
    const ctx = this.ctx;
    const pulse = selected ? Math.sin(Date.now() / 120) * 2 : 0;
    const cardStyle = this.equippedCosmetic('card');
    const styleName = cardStyle && cardStyle.preview || 'classic';
    let cardFill = ctx.createLinearGradient(x, y, x + width, y + height);
    cardFill.addColorStop(0, 'rgba(239,250,255,0.98)');
    cardFill.addColorStop(0.52, 'rgba(207,237,255,0.97)');
    cardFill.addColorStop(1, 'rgba(178,202,255,0.96)');
    let cardStroke = '#8de7ff';
    let shadowColor = 'rgba(0,0,40,0.24)';
    if (styleName === 'neon') {
      cardFill = ctx.createLinearGradient(x, y, x + width, y + height);
      cardFill.addColorStop(0, 'rgba(220,255,255,0.98)');
      cardFill.addColorStop(0.52, 'rgba(188,239,255,0.96)');
      cardFill.addColorStop(1, 'rgba(111,180,255,0.94)');
      cardStroke = '#42f3ff';
      shadowColor = 'rgba(40,233,255,0.42)';
    } else if (styleName === 'candy') {
      cardFill = ctx.createLinearGradient(x, y, x + width, y + height);
      cardFill.addColorStop(0, 'rgba(255,244,255,0.98)');
      cardFill.addColorStop(0.55, 'rgba(255,221,248,0.96)');
      cardFill.addColorStop(1, 'rgba(191,220,255,0.96)');
      cardStroke = '#ff9edb';
      shadowColor = 'rgba(255,105,220,0.32)';
    }
    ctx.save();
    ctx.shadowColor = selected ? 'rgba(40,233,255,0.62)' : shadowColor;
    ctx.shadowBlur = selected ? 20 : 14;
    ctx.shadowOffsetY = selected ? 0 : 5;
    this.roundRect(x - pulse / 2, y - pulse / 2, width + pulse, height + pulse, 28, cardFill, selected ? GAME_UI.cyan : cardStroke, selected ? 4 : 2);
    ctx.restore();
    const theme = skinCatalog.getSkin(this.activeSkinId()).theme || {};
    // 数字卡片始终使用浅色卡面。对旧存档或未来主题配置做兜底，
    // 避免浅色文字和卡面融为一体，出现“数字消失”的问题。
    const configuredText = String(theme.card_text || '').trim().toLowerCase();
    const lightText = configuredText === '#fff' || configuredText === '#ffffff' || configuredText === 'white';
    const numberColor = !configuredText || lightText ? '#17163E' : theme.card_text;
    this.drawText(String(value), x + width / 2, y + height / 2 + 6, uiFont(value >= 10 ? 52 : 64, 900), numberColor);
    if (selected) {
      this.drawGlassCard(x + 16, y + 13, 92, 30, 15, 'rgba(8,87,145,0.86)', 'rgba(140,247,255,0.90)', {
        shadowColor: 'rgba(40,233,255,0.42)', shadowBlur: 8, shadowOffsetY: 0, innerAlpha: 0.02,
      });
      this.drawText(this.selectedOperator ? '等待第二个' : '第一个数字', x + 62, y + 28, uiFont(13, 800), GAME_UI.cyanLight);
    }
  }

  drawOperatorButton(x, y, width, height, operator, action, selected = false, disabled = false) {
    const operatorStyle = this.equippedCosmetic('operator');
    const styleName = operatorStyle && operatorStyle.preview || 'classic';
    const baseVariant = styleName === 'bubble' ? 'magenta' : styleName === 'prism' ? 'cyan' : 'violet';
    this.drawNeonButton(x, y, width, height, operator, action, selected ? 'cyan' : baseVariant, {
      disabled,
      fontSize: 40,
      radius: styleName === 'bubble' ? 30 : 22,
      key: `operator-${operator}`,
      shadowBlur: selected ? 16 : 9,
    });
  }

  drawLockIcon(cx, cy, scale = 1, color = GAME_UI.gold) {
    const ctx = this.ctx;
    ctx.save();
    ctx.strokeStyle = color;
    ctx.fillStyle = color;
    ctx.lineWidth = 4 * scale;
    ctx.lineCap = 'round';
    ctx.shadowColor = 'rgba(255,211,77,0.45)';
    ctx.shadowBlur = 8;
    ctx.beginPath();
    ctx.arc(cx, cy - 5 * scale, 14 * scale, Math.PI, Math.PI * 2);
    ctx.stroke();
    this.roundRect(cx - 18 * scale, cy - 2 * scale, 36 * scale, 31 * scale, 5 * scale, color, 'rgba(255,240,151,0.9)', 1 * scale);
    ctx.fillStyle = '#77500b';
    ctx.beginPath();
    ctx.arc(cx, cy + 10 * scale, 3.5 * scale, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();
  }

  drawQuestionIcon(cx, cy, scale = 1) {
    const ctx = this.ctx;
    ctx.save();
    ctx.shadowColor = 'rgba(154,100,255,0.6)';
    ctx.shadowBlur = 12;
    ctx.beginPath();
    ctx.arc(cx, cy, 22 * scale, 0, Math.PI * 2);
    ctx.fillStyle = 'rgba(72,52,160,0.72)';
    ctx.fill();
    ctx.strokeStyle = 'rgba(200,164,255,0.75)';
    ctx.lineWidth = 2 * scale;
    ctx.stroke();
    ctx.fillStyle = '#dcd2ff';
    ctx.font = scaleFont(uiFont(30 * scale, 900));
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText('?', cx, cy + 2 * scale);
    ctx.restore();
  }

  drawTargetIcon(cx, cy, scale = 1, color = GAME_UI.violetLight) {
    const ctx = this.ctx;
    ctx.save();
    ctx.strokeStyle = color;
    ctx.lineWidth = 3 * scale;
    ctx.shadowColor = color;
    ctx.shadowBlur = 10;
    [25, 15, 5].forEach((radius) => {
      ctx.beginPath();
      ctx.arc(cx, cy, radius * scale, 0, Math.PI * 2);
      ctx.stroke();
    });
    ctx.beginPath();
    ctx.moveTo(cx - 8 * scale, cy + 8 * scale);
    ctx.lineTo(cx + 28 * scale, cy - 28 * scale);
    ctx.stroke();
    ctx.restore();
  }

  drawLightningIcon(cx, cy, scale = 1, color = GAME_UI.violetLight) {
    const ctx = this.ctx;
    ctx.save();
    ctx.fillStyle = color;
    ctx.shadowColor = color;
    ctx.shadowBlur = 12;
    ctx.beginPath();
    ctx.moveTo(cx + 3 * scale, cy - 31 * scale);
    ctx.lineTo(cx - 16 * scale, cy + 4 * scale);
    ctx.lineTo(cx + 1 * scale, cy + 4 * scale);
    ctx.lineTo(cx - 6 * scale, cy + 31 * scale);
    ctx.lineTo(cx + 20 * scale, cy - 7 * scale);
    ctx.lineTo(cx + 4 * scale, cy - 7 * scale);
    ctx.closePath();
    ctx.fill();
    ctx.restore();
  }

  drawClockIcon(cx, cy, scale = 1, color = GAME_UI.violetLight) {
    const ctx = this.ctx;
    ctx.save();
    ctx.strokeStyle = color;
    ctx.lineWidth = 4 * scale;
    ctx.lineCap = 'round';
    ctx.shadowColor = color;
    ctx.shadowBlur = 10;
    ctx.beginPath();
    ctx.arc(cx, cy, 23 * scale, 0, Math.PI * 2);
    ctx.stroke();
    ctx.beginPath();
    ctx.moveTo(cx, cy);
    ctx.lineTo(cx, cy - 13 * scale);
    ctx.moveTo(cx, cy);
    ctx.lineTo(cx + 12 * scale, cy + 8 * scale);
    ctx.stroke();
    ctx.restore();
  }

  drawStarIcon(cx, cy, scale = 1, color = GAME_UI.gold) {
    const ctx = this.ctx;
    ctx.save();
    ctx.fillStyle = color;
    ctx.strokeStyle = GAME_UI.goldLight;
    ctx.lineWidth = 2 * scale;
    ctx.shadowColor = color;
    ctx.shadowBlur = 14;
    ctx.beginPath();
    for (let point = 0; point < 10; point += 1) {
      const radius = (point % 2 === 0 ? 32 : 13) * scale;
      const angle = -Math.PI / 2 + point * Math.PI / 5;
      const x = cx + Math.cos(angle) * radius;
      const y = cy + Math.sin(angle) * radius;
      if (point === 0) ctx.moveTo(x, y);
      else ctx.lineTo(x, y);
    }
    ctx.closePath();
    ctx.fill();
    ctx.stroke();
    ctx.restore();
  }

  drawCoinIcon(cx, cy, radius) {
    const ctx = this.ctx;
    const outer = ctx.createRadialGradient(cx - radius * 0.32, cy - radius * 0.38, 2, cx, cy, radius);
    outer.addColorStop(0, '#fff7a8');
    outer.addColorStop(0.35, '#ffd84b');
    outer.addColorStop(0.72, '#ff9e1e');
    outer.addColorStop(1, '#ff6f13');
    ctx.save();
    ctx.shadowColor = 'rgba(255,189,40,0.88)';
    ctx.shadowBlur = 16;
    ctx.fillStyle = outer;
    ctx.beginPath();
    ctx.arc(cx, cy, radius, 0, Math.PI * 2);
    ctx.fill();
    ctx.lineWidth = 3;
    ctx.strokeStyle = '#fff277';
    ctx.stroke();
    ctx.beginPath();
    ctx.arc(cx, cy, radius * 0.72, 0, Math.PI * 2);
    ctx.strokeStyle = 'rgba(255,128,18,0.72)';
    ctx.stroke();
    ctx.shadowBlur = 0;
    ctx.fillStyle = '#a25b00';
    ctx.font = scaleFont(uiFont(radius * 1.13, 900));
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText('¥', cx, cy + radius * 0.06);
    ctx.restore();
  }

  drawGearIcon(cx, cy, radius, color = '#dbe3ff') {
    const ctx = this.ctx;
    ctx.save();
    ctx.translate(cx, cy);
    ctx.strokeStyle = color;
    ctx.shadowColor = 'rgba(210,225,255,0.45)';
    ctx.shadowBlur = 8;
    ctx.lineWidth = radius * 0.25;
    ctx.lineCap = 'round';
    for (let index = 0; index < 8; index += 1) {
      ctx.save();
      ctx.rotate((Math.PI * 2 * index) / 8);
      ctx.beginPath();
      ctx.moveTo(0, -radius * 0.9);
      ctx.lineTo(0, -radius * 1.18);
      ctx.stroke();
      ctx.restore();
    }
    ctx.lineWidth = radius * 0.25;
    ctx.beginPath();
    ctx.arc(0, 0, radius * 0.64, 0, Math.PI * 2);
    ctx.stroke();
    ctx.lineWidth = radius * 0.2;
    ctx.beginPath();
    ctx.arc(0, 0, radius * 0.22, 0, Math.PI * 2);
    ctx.stroke();
    ctx.restore();
  }

  drawFlagIcon(cx, cy, scale = 1) {
    const ctx = this.ctx;
    ctx.save();
    ctx.shadowColor = 'rgba(80,240,255,0.78)';
    ctx.shadowBlur = 18;
    ctx.lineCap = 'round';
    ctx.strokeStyle = '#f3ffff';
    ctx.lineWidth = 7 * scale;
    ctx.beginPath();
    ctx.moveTo(cx - 45 * scale, cy - 54 * scale);
    ctx.lineTo(cx - 45 * scale, cy + 62 * scale);
    ctx.stroke();
    ctx.fillStyle = '#f6ffff';
    ctx.beginPath();
    ctx.arc(cx - 45 * scale, cy - 58 * scale, 8 * scale, 0, Math.PI * 2);
    ctx.fill();
    ctx.beginPath();
    ctx.moveTo(cx - 34 * scale, cy - 48 * scale);
    ctx.bezierCurveTo(cx + 8 * scale, cy - 68 * scale, cx + 42 * scale, cy - 18 * scale, cx + 82 * scale, cy - 36 * scale);
    ctx.bezierCurveTo(cx + 72 * scale, cy - 10 * scale, cx + 64 * scale, cy + 12 * scale, cx + 82 * scale, cy + 36 * scale);
    ctx.bezierCurveTo(cx + 38 * scale, cy + 54 * scale, cx + 12 * scale, cy + 4 * scale, cx - 34 * scale, cy + 22 * scale);
    ctx.closePath();
    ctx.fill();
    ctx.fillStyle = '#15aee7';
    ctx.beginPath();
    ctx.moveTo(cx + 8 * scale, cy - 18 * scale);
    ctx.lineTo(cx + 42 * scale, cy);
    ctx.lineTo(cx + 8 * scale, cy + 18 * scale);
    ctx.closePath();
    ctx.fill();
    ctx.restore();
  }

  drawVsStar(cx, cy, scale = 1) {
    const ctx = this.ctx;
    const outer = 78 * scale;
    const inner = 31 * scale;
    ctx.save();
    ctx.translate(cx, cy);
    ctx.shadowColor = 'rgba(255,84,200,0.86)';
    ctx.shadowBlur = 20;
    const starGradient = ctx.createLinearGradient(-outer, -outer, outer, outer);
    starGradient.addColorStop(0, '#ff77d8');
    starGradient.addColorStop(0.48, '#bf248d');
    starGradient.addColorStop(1, '#692077');
    ctx.fillStyle = starGradient;
    ctx.strokeStyle = 'rgba(255,130,230,0.95)';
    ctx.lineWidth = 4 * scale;
    ctx.beginPath();
    for (let point = 0; point < 10; point += 1) {
      const radius = point % 2 === 0 ? outer : inner;
      const angle = -Math.PI / 2 + point * Math.PI / 5;
      const x = Math.cos(angle) * radius;
      const y = Math.sin(angle) * radius;
      if (point === 0) ctx.moveTo(x, y);
      else ctx.lineTo(x, y);
    }
    ctx.closePath();
    ctx.fill();
    ctx.stroke();
    ctx.font = scaleFont(`900 ${58 * scale}px Arial Black,${UI_FONT}`);
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.lineWidth = 4 * scale;
    ctx.strokeStyle = 'rgba(255,255,255,0.76)';
    ctx.strokeText('VS', 0, 11 * scale);
    ctx.fillStyle = '#fff3fb';
    ctx.fillText('VS', 0, 11 * scale);
    ctx.restore();
  }

  drawInfinityIcon(cx, cy, scale = 1) {
    const ctx = this.ctx;
    ctx.save();
    ctx.shadowColor = 'rgba(188,92,255,0.95)';
    ctx.shadowBlur = 24;
    ctx.font = scaleFont(`900 ${112 * scale}px Arial Black,${UI_FONT}`);
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.lineWidth = 4 * scale;
    ctx.strokeStyle = 'rgba(255,255,255,0.55)';
    ctx.strokeText('∞', cx, cy);
    ctx.fillStyle = '#fff8ff';
    ctx.fillText('∞', cx, cy);
    ctx.restore();
  }

  drawCalendarIcon(cx, cy, scale = 1) {
    const ctx = this.ctx;
    ctx.save();
    ctx.shadowColor = 'rgba(255,218,96,0.72)';
    ctx.shadowBlur = 15;
    const fill = ctx.createLinearGradient(cx, cy - 58 * scale, cx, cy + 56 * scale);
    fill.addColorStop(0, '#fff5a9');
    fill.addColorStop(1, '#ffbf40');
    this.roundRect(cx - 45 * scale, cy - 48 * scale, 90 * scale, 92 * scale, 12 * scale, fill, 'rgba(255,245,170,0.95)', 2 * scale);
    ctx.strokeStyle = '#9c5810';
    ctx.lineWidth = 7 * scale;
    ctx.lineCap = 'round';
    ctx.beginPath();
    ctx.moveTo(cx - 20 * scale, cy + 4 * scale);
    ctx.lineTo(cx - 2 * scale, cy + 24 * scale);
    ctx.lineTo(cx + 30 * scale, cy - 13 * scale);
    ctx.stroke();
    ctx.strokeStyle = '#fff0a6';
    ctx.lineWidth = 8 * scale;
    ctx.beginPath();
    ctx.moveTo(cx - 24 * scale, cy - 56 * scale);
    ctx.lineTo(cx - 24 * scale, cy - 34 * scale);
    ctx.moveTo(cx + 24 * scale, cy - 56 * scale);
    ctx.lineTo(cx + 24 * scale, cy - 34 * scale);
    ctx.stroke();
    ctx.restore();
  }

  drawClipboardIcon(cx, cy, scale = 1) {
    const ctx = this.ctx;
    ctx.save();
    ctx.shadowColor = 'rgba(133,160,255,0.70)';
    ctx.shadowBlur = 14;
    const fill = ctx.createLinearGradient(cx, cy - 58 * scale, cx, cy + 58 * scale);
    fill.addColorStop(0, '#eff7ff');
    fill.addColorStop(1, '#99b7ff');
    this.roundRect(cx - 42 * scale, cy - 48 * scale, 84 * scale, 92 * scale, 9 * scale, fill, 'rgba(220,232,255,0.96)', 2 * scale);
    this.roundRect(cx - 24 * scale, cy - 61 * scale, 48 * scale, 25 * scale, 8 * scale, '#ccd9ff', 'rgba(255,255,255,0.9)', 2 * scale);
    ctx.strokeStyle = '#24318f';
    ctx.lineWidth = 7 * scale;
    ctx.lineCap = 'round';
    ctx.beginPath();
    ctx.moveTo(cx - 23 * scale, cy - 2 * scale);
    ctx.lineTo(cx - 4 * scale, cy + 20 * scale);
    ctx.lineTo(cx + 29 * scale, cy - 21 * scale);
    ctx.stroke();
    ctx.restore();
  }

  drawBarsIcon(cx, cy, scale = 1) {
    const ctx = this.ctx;
    const fill = ctx.createLinearGradient(cx - 48 * scale, cy - 56 * scale, cx + 48 * scale, cy + 50 * scale);
    fill.addColorStop(0, '#fff4ff');
    fill.addColorStop(1, '#b35cff');
    ctx.save();
    ctx.shadowColor = 'rgba(190,94,255,0.80)';
    ctx.shadowBlur = 18;
    const widths = [23, 23, 23].map((value) => value * scale);
    const heights = [45, 72, 104].map((value) => value * scale);
    [-36, 0, 36].forEach((offset, index) => {
      this.roundRect(cx + offset * scale - widths[index] / 2, cy + 55 * scale - heights[index], widths[index], heights[index], 5 * scale, fill, 'rgba(255,225,255,0.82)', 1.5 * scale);
    });
    ctx.restore();
  }

  isHomeButtonActive(key) {
    return this.homeMotion && this.homeMotion.activeButton === key && Date.now() < this.homeMotion.activeUntil;
  }

  withHomePressEffect(key, x, y, width, height, draw) {
    const ctx = this.ctx;
    const active = this.isHomeButtonActive(key);
    ctx.save();
    if (active) {
      ctx.translate(x + width / 2, y + height / 2);
      ctx.scale(0.97, 0.97);
      ctx.translate(-(x + width / 2), -(y + height / 2));
    }
    draw();
    ctx.restore();
  }

  addHomeHitArea(key, x, y, width, height, action, options = {}) {
    const scaleY = this.homeYScale || 1;
    this.addHitArea(x, y * scaleY, width, height * scaleY, action, Object.assign({}, options, { key }));
  }

  drawFloatingNumberCard(text, x, y, width, height, rotation, fillA, fillB, stroke) {
    const ctx = this.ctx;
    ctx.save();
    ctx.translate(x + width / 2, y + height / 2);
    ctx.rotate(rotation);
    const fill = ctx.createLinearGradient(-width / 2, -height / 2, width / 2, height / 2);
    fill.addColorStop(0, fillA);
    fill.addColorStop(1, fillB);
    ctx.shadowColor = stroke;
    ctx.shadowBlur = 18;
    this.roundRect(-width / 2, -height / 2, width, height, 13, fill, stroke, 2);
    ctx.fillStyle = '#c9f5ff';
    ctx.font = scaleFont(uiFont(42, 900));
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.shadowBlur = 10;
    ctx.fillText(text, 0, 4);
    ctx.restore();
  }

  drawGoldTitle(text, x, y, font, align = 'left') {
    const ctx = this.ctx;
    ctx.save();
    ctx.font = scaleFont(font);
    ctx.textAlign = align;
    ctx.textBaseline = 'middle';
    ctx.lineWidth = 3;
    ctx.strokeStyle = 'rgba(143,78,0,0.56)';
    ctx.shadowColor = 'rgba(255,196,30,0.42)';
    ctx.shadowBlur = 9;
    ctx.strokeText(text, x, y + 3);
    const gradient = ctx.createLinearGradient(x, y - 36, x, y + 32);
    gradient.addColorStop(0, '#fff487');
    gradient.addColorStop(0.45, '#ffd643');
    gradient.addColorStop(1, '#ff9c22');
    ctx.fillStyle = gradient;
    ctx.fillText(text, x, y);
    ctx.restore();
  }

  drawHeroLogo(time) {
    const ctx = this.ctx;
    const glow = 0.88 + Math.sin(time * 2.2) * 0.08;
    ctx.save();
    ctx.shadowColor = `rgba(50,190,255,${glow})`;
    ctx.shadowBlur = 26;
    ctx.font = `900 260px Arial Black,${UI_FONT}`;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.lineJoin = 'round';
    ctx.lineWidth = 8;
    ctx.strokeStyle = 'rgba(33,168,255,0.95)';
    ctx.strokeText('24', 333, 470);
    ctx.lineWidth = 3;
    ctx.strokeStyle = '#ffffff';
    ctx.strokeText('24', 333, 455);
    const fill = ctx.createLinearGradient(0, 324, 0, 590);
    fill.addColorStop(0, '#ffffff');
    fill.addColorStop(0.45, '#ecf9ff');
    fill.addColorStop(1, '#b8ecff');
    ctx.fillStyle = fill;
    ctx.fillText('24', 333, 455);
    ctx.shadowBlur = 10;
    ctx.shadowColor = 'rgba(0,112,255,0.72)';
    ctx.fillStyle = 'rgba(69,170,255,0.20)';
    ctx.fillText('24', 333, 470);
    ctx.restore();

    this.drawGoldTitle(HOME_TITLE.slice(0, 2), 506, 457, uiFont(68, 900), 'left');
    this.drawGoldTitle(HOME_TITLE.slice(2), 506, 529, uiFont(40, 900), 'left');
  }

  drawTopHud() {
    const menuBottom = this.menuButton
      ? (safeNumber(this.menuButton.bottom, 0) - this.renderOffsetY) / this.renderScale
      : 80;
    const homeScale = this.homeYScale || 1;
    const topY = Math.max(126, (menuBottom + 22) / homeScale);
    const settingX = 558;
    const settingWidth = 156;
    // 金币条和设置按钮使用同一个 top/height，保证真机上两者的视觉中心和上下边缘都对齐。
    const settingHeight = 70;
    const settingY = Math.round(topY);

    this.drawGlassCard(43, topY, 240, 70, 36, 'rgba(30,24,75,0.75)', 'rgba(255,205,77,0.82)', {
      shadowColor: 'rgba(255,190,50,0.34)', shadowBlur: 15, shadowOffsetY: 0, innerAlpha: 0.08,
    });
    this.drawCoinIcon(87, topY + 35, 31);
    this.drawFitText(String(safeNumber(this.progress.coins)), 165, topY + 37, 146, uiFont(39, 900), '#ffffff');

    this.withHomePressEffect('settings', settingX, settingY, settingWidth, settingHeight, () => {
      this.drawGlassCard(settingX, settingY, settingWidth, settingHeight, 31, 'rgba(23,21,71,0.76)', 'rgba(210,210,255,0.55)', {
        shadowColor: 'rgba(120,125,255,0.18)', shadowBlur: 10, shadowOffsetY: 0, innerAlpha: 0.08,
      });
      this.drawGearIcon(settingX + 35, settingY + 31, 13, '#dce4ff');
      this.drawText('设置', settingX + 101, settingY + 32, uiFont(26, 900), '#ffffff');
    });
    this.addHomeHitArea('settings', settingX, settingY, settingWidth, settingHeight, () => { this.popup = this.popup === 'settings' ? '' : 'settings'; });
  }

  drawPrimaryModeCard(key, x, y, width, height, theme, drawIcon, title, subtitle, action, footer = '') {
    this.withHomePressEffect(key, x, y, width, height, () => {
      const fill = this.ctx.createLinearGradient(x, y, x, y + height);
      fill.addColorStop(0, theme.top);
      fill.addColorStop(1, theme.bottom);
      this.drawGlassCard(x, y, width, height, 29, fill, theme.stroke, {
        lineWidth: 2.4,
        shadowColor: theme.shadow,
        shadowBlur: theme.shadowBlur || 22,
        shadowOffsetY: 0,
        hotspot: { x: x + width * 0.4, y: y + height * 0.17, r: width * 0.88, color: theme.hotspot || 'rgba(255,255,255,0.08)' },
      });
      drawIcon(x + width / 2, y + 96);
      this.drawFitText(title, x + width / 2, y + 207, width - 30, uiFont(36, 900), '#ffffff');
      this.drawFitText(subtitle, x + width / 2, y + 253, width - 30, uiFont(23, 700), 'rgba(255,255,255,0.80)');
      if (footer) {
        this.drawGlassCard(x + 20, y + height - 67, width - 40, 50, 20, 'rgba(15,59,90,0.42)', theme.footerStroke || theme.stroke, {
          shadowColor: theme.shadow, shadowBlur: 9, shadowOffsetY: 0, innerAlpha: 0.06,
        });
        this.drawFitText(footer, x + width / 2, y + height - 42, width - 56, uiFont(23, 900), '#aaf7ff');
      }
    });
    this.addHomeHitArea(key, x, y, width, height, action);
  }

  drawSecondaryCard(key, x, y, width, height, theme, drawIcon, title, subtitle, action, badge = '', options = {}) {
    this.withHomePressEffect(key, x, y, width, height, () => {
      const fill = this.ctx.createLinearGradient(x, y, x, y + height);
      fill.addColorStop(0, theme.top);
      fill.addColorStop(1, theme.bottom);
      this.drawGlassCard(x, y, width, height, 25, fill, theme.stroke, {
        lineWidth: 1.8,
        shadowColor: theme.shadow,
        shadowBlur: 18,
        shadowOffsetY: 0,
        hotspot: { x: x + width * 0.5, y: y + height * 0.2, r: width * 0.72, color: theme.hotspot || 'rgba(255,255,255,0.06)' },
      });
      drawIcon(x + width / 2, y + 64);
      if (badge === 'NEW') {
        this.drawGlassCard(x + width - 72, y + 13, 69, 38, 18, '#ff673a', 'rgba(255,232,190,0.55)', {
          shadowColor: 'rgba(255,90,40,0.62)', shadowBlur: 9, shadowOffsetY: 0, innerAlpha: 0.05,
        });
        this.drawText('NEW', x + width - 38, y + 32, uiFont(22, 900), '#ffffff');
      } else if (badge === 'DONE') {
        this.drawGlassCard(x + width - 105, y + 13, 92, 38, 18, 'rgba(46,92,118,0.82)', 'rgba(135,239,220,0.72)', {
          shadowColor: 'rgba(82,221,196,0.30)', shadowBlur: 9, shadowOffsetY: 0, innerAlpha: 0.05,
        });
        this.drawText('已完成', x + width - 59, y + 32, uiFont(18, 900), '#b7ffe9');
      } else if (badge === '+20') {
        this.drawGlassCard(x + width - 86, y + 15, 72, 34, 17, 'rgba(48,38,112,0.82)', 'rgba(255,221,78,0.75)', {
          shadowColor: 'rgba(255,203,40,0.35)', shadowBlur: 8, shadowOffsetY: 0, innerAlpha: 0.04,
        });
        this.drawCoinIcon(x + width - 70, y + 32, 13);
        this.drawText('+20', x + width - 39, y + 33, uiFont(19, 900), '#ffde57');
      }
      this.drawFitText(title, x + width / 2, y + 127, width - 30, uiFont(29, 900), '#ffffff');
      this.drawFitText(subtitle, x + width / 2, y + 166, width - 30, uiFont(20, 600), theme.subColor || 'rgba(255,255,255,0.72)');
    });
    this.addHomeHitArea(key, x, y, width, height, action, { disabled: Boolean(options.disabled) });
  }

  drawProgressPanel() {
    const x = 40;
    const y = 1385;
    const width = 670;
    const height = 160;
    this.drawGlassCard(x, y, width, height, 29, 'rgba(30,30,110,0.38)', 'rgba(120,135,255,0.62)', {
      shadowColor: 'rgba(65,75,220,0.28)', shadowBlur: 18, shadowOffsetY: 0, innerAlpha: 0.08,
    });
    const unlocked = this.highestPlayableLevelNumber();
    this.drawFitText(`闯关进度 · 已解锁 1-${unlocked} 关`, this.width / 2, y + 50, width - 58, uiFont(30, 900), '#ffffff');
    const barX = x + 57;
    const barY = y + 96;
    const barWidth = width - 114;
    const barHeight = 28;
    this.drawGlassCard(barX, barY, barWidth, barHeight, 999, 'rgba(12,10,70,0.75)', 'rgba(190,190,255,0.60)', {
      shadow: false, shadowBlur: 0, shadowOffsetY: 0, innerAlpha: 0.02,
    });
    const fillWidth = barWidth * clamp(unlocked / 200, 0.02, 1);
    const fill = this.ctx.createLinearGradient(barX, barY, barX + fillWidth, barY);
    fill.addColorStop(0, '#10dfff');
    fill.addColorStop(0.58, '#2fcbff');
    fill.addColorStop(1, '#5ec5ff');
    this.ctx.save();
    this.ctx.shadowColor = 'rgba(0,220,255,0.75)';
    this.ctx.shadowBlur = 12;
    this.roundRect(barX, barY, fillWidth, barHeight, 999, fill, 'rgba(145,250,255,0.8)', 1.5);
    this.ctx.restore();
    this.ctx.save();
    this.ctx.globalAlpha = 0.65;
    this.ctx.strokeStyle = 'rgba(255,255,255,0.72)';
    this.ctx.lineWidth = 2;
    this.ctx.beginPath();
    this.ctx.moveTo(barX + fillWidth, barY + 2);
    this.ctx.lineTo(barX + fillWidth, barY + barHeight - 2);
    this.ctx.stroke();
    this.ctx.restore();
    this.drawSparkle(barX + 19, barY + barHeight / 2, 21, '#baffff', 0.95);
  }

  drawModeCard(x, y, width, height, options, action) {
    this.drawPanel(x, y, width, height, options.fill, options.stroke || `${options.accent}99`, 28, {
      shadowColor: `${options.accent}55`, shadowBlur: 18, shadowOffsetY: 5,
    });
    this.ctx.save();
    this.ctx.globalAlpha = 0.16;
    this.ctx.fillStyle = '#ffffff';
    this.ctx.beginPath();
    this.ctx.arc(x + width * 0.2, y + height * 0.16, width * 0.33, 0, Math.PI * 2);
    this.ctx.fill();
    this.ctx.restore();
    if (options.compact) {
      this.drawText(options.icon, x + width / 2, y + 40, options.iconFont || 'bold 36px sans-serif', COLORS.text);
      this.drawText(options.title, x + width / 2, y + height - 29, options.titleFont || 'bold 19px sans-serif', COLORS.text);
    } else {
      this.drawText(options.icon, x + width / 2, y + 72, options.iconFont || 'bold 52px sans-serif', COLORS.text);
      this.drawText(options.title, x + width / 2, y + 156, 'bold 24px sans-serif', COLORS.text);
      if (options.subtitle) this.drawText(options.subtitle, x + width / 2, y + 190, '16px sans-serif', 'rgba(255,255,255,0.82)');
      if (options.progress) this.drawText(options.progress, x + width / 2, y + height - 34, '18px sans-serif', 'rgba(255,255,255,0.94)');
    }
    this.addHitArea(x, y, width, height, action);
  }

  drawStatChip(x, y, width, height, caption, value, accent) {
    this.drawPanel(x, y, width, height, 'rgba(37,33,91,0.88)', `${accent}80`, 20, { shadowBlur: 8, shadowOffsetY: 2 });
    this.drawText(caption, x + width / 2, y + 18, '13px sans-serif', COLORS.muted);
    this.drawText(value, x + width / 2, y + 46, 'bold 20px sans-serif', accent);
  }

  drawHome(time) {
    this.buttons = [];
    const ctx = this.ctx;
    const unlocked = this.highestPlayableLevelNumber();
    const dailyCompleted = storage.isDailyCompleted(this.progress, storage.todayKey());
    ctx.save();
    ctx.scale(1, this.homeYScale);
    this.drawTopHud();

    this.drawFloatingNumberCard('3', 54, 405, 73, 95, -0.36, 'rgba(22,135,255,0.86)', 'rgba(36,45,158,0.82)', 'rgba(69,205,255,0.92)');
    this.drawFloatingNumberCard('6', 483, 267, 77, 86, 0.42, 'rgba(223,91,255,0.86)', 'rgba(67,36,142,0.82)', 'rgba(242,87,255,0.85)');
    this.drawFloatingNumberCard('8', 668, 565, 69, 80, 0.34, 'rgba(55,156,255,0.84)', 'rgba(47,50,156,0.84)', 'rgba(83,179,255,0.86)');

    this.drawHeroLogo(time);
    this.drawText('4个数字，算出24！', this.width / 2, 606, uiFont(31, 900), '#ffffff');
    this.drawText('每天一局，越算越快', this.width / 2, 654, uiFont(27, 700), 'rgba(255,255,255,0.72)');

    const cardY = 733;
    const cardWidth = 215;
    const cardHeight = 370;
    this.drawPrimaryModeCard('campaign', 31, cardY, cardWidth, cardHeight, {
      top: 'rgba(17,190,230,0.48)',
      bottom: 'rgba(5,90,145,0.50)',
      stroke: 'rgba(70,240,255,0.92)',
      shadow: 'rgba(30,230,255,0.52)',
      shadowBlur: 28,
      footerStroke: 'rgba(70,240,255,0.68)',
      hotspot: 'rgba(72,238,255,0.14)',
    }, (cx, cy) => {
      this.drawFlagIcon(cx, cy - 6, 0.82);
      this.drawThinOrbit(cx, cy + 73, 145, 38, 0, 'rgba(61,235,255,0.44)', 'rgba(61,235,255,0.28)');
    }, '闯关模式', '从简单开始', () => this.showLevels(), `继续第 ${Math.min(unlocked, 200)} 关`);

    this.drawPrimaryModeCard('friend', 267, cardY + 8, cardWidth, cardHeight - 8, {
      top: 'rgba(194,38,157,0.44)',
      bottom: 'rgba(110,25,130,0.50)',
      stroke: 'rgba(255,105,220,0.90)',
      shadow: 'rgba(255,60,190,0.33)',
      shadowBlur: 24,
      hotspot: 'rgba(255,83,199,0.12)',
    }, (cx, cy) => this.drawVsStar(cx, cy - 5, 0.76), '好友对战', '看看谁算得快', () => this.showFriendLobby());

    this.drawPrimaryModeCard('endless', 503, cardY + 8, cardWidth, cardHeight - 8, {
      top: 'rgba(125,70,220,0.47)',
      bottom: 'rgba(72,34,150,0.52)',
      stroke: 'rgba(180,120,255,0.92)',
      shadow: 'rgba(155,78,255,0.38)',
      shadowBlur: 26,
      hotspot: 'rgba(161,80,255,0.13)',
    }, (cx, cy) => {
      this.drawInfinityIcon(cx, cy + 5, 0.78);
      this.drawThinOrbit(cx, cy + 78, 140, 38, 0, 'rgba(166,80,255,0.34)', 'rgba(166,80,255,0.22)');
    }, '无尽模式', '挑战你的极限', () => this.startEndless());

    const smallY = 1150;
    const smallWidth = 216;
    const smallHeight = 219;
    this.drawSecondaryCard('daily', 35, smallY, smallWidth, smallHeight, {
      top: 'rgba(160,100,45,0.47)',
      bottom: 'rgba(92,57,70,0.47)',
      stroke: 'rgba(255,200,70,0.92)',
      shadow: 'rgba(255,187,56,0.35)',
      subColor: '#fff1a2',
      hotspot: 'rgba(255,202,72,0.12)',
    }, (cx, cy) => this.drawCalendarIcon(cx, cy, 0.82), '每日挑战', dailyCompleted ? '今日已完成' : '今日题目已更新', () => this.startDaily(), dailyCompleted ? 'DONE' : 'NEW', { disabled: dailyCompleted });

    this.drawSecondaryCard('tasks', 267, smallY, smallWidth, smallHeight, {
      top: 'rgba(72,82,205,0.44)',
      bottom: 'rgba(44,48,145,0.49)',
      stroke: 'rgba(112,138,255,0.88)',
      shadow: 'rgba(98,126,255,0.32)',
      hotspot: 'rgba(123,150,255,0.12)',
    }, (cx, cy) => this.drawClipboardIcon(cx, cy, 0.78), '每日任务', '完成领取奖励', () => { this.popup = 'tasks'; }, '+20');

    this.drawSecondaryCard('more', 500, smallY, smallWidth, smallHeight, {
      top: 'rgba(117,44,171,0.46)',
      bottom: 'rgba(62,28,128,0.50)',
      stroke: 'rgba(180,82,255,0.78)',
      shadow: 'rgba(164,74,255,0.32)',
      hotspot: 'rgba(182,83,255,0.13)',
    }, (cx, cy) => this.drawBarsIcon(cx, cy + 2, 0.82), '更多功能', '排行榜 · 成就', () => { this.popup = this.popup === 'more' ? '' : 'more'; });

    this.drawProgressPanel();
    ctx.restore();
  }

  drawLevels() {
    this.buttons = [];
    const w = this.width;
    this.drawGameHeader('选择关卡', '‹ 首页', () => this.goHome(), '闯关模式');
    const chapter = levelCatalog.CHAPTERS[this.menuPage] || levelCatalog.CHAPTERS[0];
    const chapterY = this.screenContentTop(92);
    this.drawGamePanel(28, chapterY, w - 56, 156, 'cyan', {
      radius: 26,
      shadowColor: 'rgba(0,220,255,0.28)',
      shadowBlur: 18,
      hotspot: { x: 110, y: chapterY + 34, r: 180, color: 'rgba(40,233,255,0.12)' },
    });
    this.drawGamePanel(500, chapterY + 60, 178, 54, 'violet', { radius: 14, shadowBlur: 8, shadowOffsetY: 0 });
    this.drawText('展开', 589, chapterY + 89, uiFont(23, 800), GAME_UI.text);
    this.addHitArea(492, chapterY + 52, 194, 70, () => { this.popup = this.popup === 'chapter_info' ? '' : 'chapter_info'; }, { key: 'chapter-expand' });
    this.drawFitText(`${this.menuPage + 1}. ${chapter[0]}`, 64, chapterY + 42, 408, uiFont(30, 900), GAME_UI.cyan, 'left');
    this.drawFitText(chapter[1], 64, chapterY + 78, 410, uiFont(17, 600), GAME_UI.text, 'left');
    const chapterStart = this.menuPage * 20;
    const blockIndex = Math.floor(chapterStart / 100);
    const previousBlockScore = blockIndex > 0 ? this.campaignBlockScore(blockIndex - 1) : 0;
    const blockGatePassed = this.isCampaignBlockUnlocked(blockIndex);
    const unlockedInChapter = clamp(safeNumber(this.progress.unlocked_level) - chapterStart, 0, 20);
    this.drawFitText(`章节进度 ${unlockedInChapter} / 20 · ${chapter[4] || '整数与明显解法'}`, 64, chapterY + 119, w - 100, uiFont(16, 500), GAME_UI.secondary, 'left');
    this.drawFitText(
      blockIndex === 0 ? '第一大章节 · 完成前 100 关后进入下一阶段' : `前 100 关累计 ${previousBlockScore} / ${this.campaignBlockGateScore()} 分 · ${blockGatePassed ? '已开放' : '未达到门槛'}`,
      64, chapterY + 144, w - 100, uiFont(13, 700), blockIndex === 0 || blockGatePassed ? GAME_UI.cyanLight : GAME_UI.gold, 'left',
    );
    this.drawFitText('操作：数字 → 运算符 → 第二个数字 · 每道题程序验证有解', 36, chapterY + 196, w - 72, uiFont(17, 500), GAME_UI.secondary, 'left');
    this.drawText('选择关卡', 36, chapterY + 236, uiFont(23, 900), GAME_UI.gold, 'left');

    const start = this.menuPage * 20;
    const gap = 16;
    const cardWidth = 154;
    const cardHeight = 104;
    for (let slot = 0; slot < 20; slot += 1) {
      const index = start + slot;
      const col = slot % 4;
      const row = Math.floor(slot / 4);
      const x = 36 + col * (cardWidth + gap);
      const y = chapterY + 272 + row * (cardHeight + gap);
      const unlocked = this.isCampaignLevelUnlocked(index);
      const levelData = this.progress.levels[String(index)] || {};
      const current = index === safeNumber(this.progress.last_level, -1) && unlocked;
      const variant = current ? 'gold' : unlocked ? 'cyan' : (index % 5 === 4 ? 'magenta' : 'cyan');
      this.drawGamePanel(x, y, cardWidth, cardHeight, variant, {
        radius: 16,
        fill: unlocked ? this.glassFill(x, y, cardWidth, cardHeight, current ? 'gold' : 'cyan') : 'rgba(13,25,72,0.58)',
        stroke: current ? GAME_UI.gold : unlocked ? 'rgba(45,230,255,0.55)' : 'rgba(80,120,205,0.45)',
        lineWidth: current ? 3 : 1.4,
        shadowColor: current ? 'rgba(255,205,60,0.42)' : unlocked ? 'rgba(40,233,255,0.22)' : 'rgba(65,45,170,0.18)',
        shadowBlur: current ? 16 : 9,
        shadowOffsetY: 0,
      });
      this.addHitArea(x, y, cardWidth, cardHeight, () => this.startCampaign(index), { disabled: !unlocked, key: `level-${index + 1}` });
      if (unlocked) {
        this.drawText(String(index + 1), x + cardWidth / 2, y + 38, uiFont(34, 900), current ? GAME_UI.text : '#eef8ff');
        const stars = Math.max(0, Math.min(3, Number(levelData.stars || 0)));
        for (let star = 0; star < 3; star += 1) {
          this.drawStarIcon(x + cardWidth / 2 - 24 + star * 24, y + 74, 0.21, star < stars ? GAME_UI.gold : 'rgba(255,211,77,0.32)');
        }
      } else {
        this.drawLockIcon(x + cardWidth / 2, y + 38, 0.78);
        this.drawFitText(index >= 100 && !blockGatePassed ? `需 ${this.campaignBlockGateScore()} 分` : '未解锁', x + cardWidth / 2, y + 76, cardWidth - 16, uiFont(15, 700), GAME_UI.secondary);
      }
    }
    // pageY 是按钮的顶部坐标，安全区返回的是底部坐标，不能直接把二者
    // 用 Math.max 连接，否则按钮会被推到屏幕外并与底部手势区重叠。
    const pageY = Math.min(chapterY + 272 + 5 * (cardHeight + gap) + 16, this.visibleBottom(64) - 56);
    this.drawNeonButton(36, pageY, 150, 56, '‹ 上一页', () => { this.menuPage = Math.max(0, this.menuPage - 1); }, 'violet', { disabled: this.menuPage === 0, fontSize: 18, radius: 20 });
    this.drawText(`第 ${this.menuPage + 1} / 10 页`, w / 2, pageY + 30, uiFont(18, 800), GAME_UI.text);
    this.drawNeonButton(w - 186, pageY, 150, 56, '下一页 ›', () => { this.menuPage = Math.min(9, this.menuPage + 1); }, 'cyan', { disabled: this.menuPage === 9, fontSize: 18, radius: 20 });
  }

  gameContentTop() {
    return this.gameLayout().contentY;
  }

  drawGame() {
    this.buttons = [];
    this.gameCardHitAreas = [];
    this.gameCardHitCardCount = Array.isArray(this.cards) ? this.cards.length : 0;
    const layout = this.gameLayout();
    const title = this.mode === 'campaign' ? `第 ${this.currentLevel + 1} 关` : this.mode === 'daily' ? '每日挑战' : this.mode === 'endless' ? '无尽模式' : '好友对战';
    const friend = this.mode === 'friend';
    const modeVariant = friend ? 'magenta' : this.mode === 'endless' ? 'violet' : this.mode === 'daily' ? 'gold' : 'cyan';
    this.drawGameHeader(title, '‹ 返回', () => this.backFromGame(), friend ? '同题竞速' : '');
    this.drawStatsRow(layout.statsY, [
      { label: friend ? '对战模式' : '当前关卡', value: friend ? '同题竞速' : title, variant: modeVariant },
      { label: '本局得分', value: `得分 ${this.score}`, variant: 'violet' },
      { label: '连续表现', value: `连击 ${this.combo}`, variant: 'gold' },
    ]);

    if (this.mode === 'daily' && this.dailyChallenge) {
      const ruleTitle = this.formatDailyRuleTitle(this.dailyChallenge.rule_title);
      this.drawGamePanel(32, layout.infoY, this.width - 64, 72, 'gold', {
        radius: 22,
        shadowColor: 'rgba(255,198,65,0.26)',
        shadowBlur: 10,
        shadowOffsetY: 0,
      });
      this.drawFitText(`今日规则：${ruleTitle}`, this.width / 2, layout.infoY + 25, this.width - 86, uiFont(17, 900), GAME_UI.gold);
      this.drawFitText(this.dailyChallenge.rule_text, this.width / 2, layout.infoY + 52, this.width - 86, uiFont(13, 500), GAME_UI.secondary);
    }

    if (friend) {
      this.pollFriendMatchProgress();
      const opponent = this.friendOpponentState();
      this.drawGamePanel(32, layout.infoY, this.width - 64, 72, 'magenta', {
        radius: 22,
        shadowColor: 'rgba(255,80,205,0.26)',
        shadowBlur: 10,
        shadowOffsetY: 0,
      });
      this.drawFitText(`我 ${this.friendPlayerSolved} / ${this.friendQuestionCount()} 题   ·   ${this.friendOpponentName()} ${opponent.solved} / ${this.friendQuestionCount()} 题`, this.width / 2, layout.infoY + 25, this.width - 86, uiFont(15, 800), GAME_UI.text);
      this.drawFitText('同一套题目 · 答错扣 5 秒 · 不允许提示', this.width / 2, layout.infoY + 52, this.width - 86, uiFont(13, 500), GAME_UI.secondary);
    }

    const contentTop = layout.contentY;
    const ratio = clamp(this.timeLeft / Math.max(1, this.timerLimit), 0, 1);
    const timerY = contentTop;
    this.drawGameTimer(timerY, `${Math.ceil(Math.max(0, this.timeLeft))} 秒`, ratio < 0.22);

    const progressRatio = this.mode === 'endless' ? ((this.currentQuestion % 3) + 1) / 3 : (this.currentQuestion + 1) / Math.max(1, this.puzzles.length);
    const questionTitle = `第 ${this.currentQuestion + 1} / ${this.mode === 'endless' ? '∞' : this.puzzles.length} 题`;
    const endlessConfig = this.mode === 'endless' ? puzzle.endlessConfig(this.currentQuestion) : null;
    const stageQuestion = this.currentQuestion % 3 + 1;
    const hint = this.mode === 'endless'
      ? `当前阶段：${endlessMode.stageName(endlessConfig.stage)} · 第 ${stageQuestion} / 3 题`
      : '操作顺序：先点数字 → 再点运算符 → 最后点数字';
    const nextMilestone = this.mode === 'endless' ? endlessMode.nextMilestoneForQuestions(this.currentQuestion + 1) : null;
    const detail = this.status || (nextMilestone ? `下一里程碑：连续答对 ${nextMilestone} 题` : '不显示计算过程，合并后继续寻找 24');
    this.drawQuestionPanel(timerY + 88, questionTitle, progressRatio, hint, detail);

    const cardWidth = layout.cardWidth;
    const cardHeight = layout.cardHeight;
    const gapX = layout.gapX;
    const gapY = layout.gapY;
    const cardStartX = (this.width - cardWidth * 2 - gapX) / 2;
    const cardStartY = layout.cardStartY;
    this.cards.forEach((card, index) => {
      const rect = this.cardRect(index, cardStartX, cardStartY, cardWidth, cardHeight, gapX, gapY);
      this.drawNumberTile(rect.x, rect.y, rect.width, rect.height, card.value, this.selectedIndex === index);
      this.gameCardHitAreas.push({ ...rect, index });
      // 数字卡片加入统一命中层。真机上不再依赖 handleGameTouch 的兜底坐标判断，
      // 并且每次重绘都会同步当前卡片顺序和位置。
      this.addHitArea(rect.x, rect.y, rect.width, rect.height, () => this.selectCard(index), { key: `game-card-${index}` });
    });

    const operators = ['+', '−', '×', '÷'];
    const opTitleY = layout.opTitleY;
    this.drawFitText('第二步 · 选择运算符', 48, opTitleY, this.width - 96, uiFont(15, 800), GAME_UI.gold, 'left');
    operators.forEach((operator, index) => {
      const x = 48 + index * ((this.width - 96 - 18 * 3) / 4 + 18);
      const width = (this.width - 96 - 18 * 3) / 4;
      const normalizedOperator = operator === '−' ? '-' : operator;
      const puzzleRules = (this.currentPuzzle && this.currentPuzzle.rules) || {};
      const forbiddenOperator = this.mode === 'campaign' ? '' : (puzzleRules.forbiddenOperator || puzzleRules.forbidden_operator || '');
      this.drawOperatorButton(x, opTitleY + 22, width, layout.operatorHeight, operator, () => this.selectOperator(normalizedOperator), this.selectedOperator === normalizedOperator, Boolean(forbiddenOperator && normalizedOperator === forbiddenOperator));
    });

    const actionY = layout.actionY;
    this.drawFitText('第三步 · 点击第二个数字完成合成', 48, actionY, this.width - 96, uiFont(14, 500), GAME_UI.secondary, 'left');
    const actionWidth = (this.width - 112) / 3;
    const undoDisabled = this.mode === 'daily' && this.dailyChallenge && this.dailyChallenge.rule_id === 'no_undo';
    this.drawNeonButton(44, actionY + 22, actionWidth, layout.actionHeight, undoDisabled ? '撤销 · 禁用' : `撤销${this.freeUndo ? ' · 免费' : ''}`, () => this.undo(), 'cyan', { fontSize: 19, radius: 20, disabled: undoDisabled, key: 'game-undo' });
    this.drawNeonButton(56 + actionWidth, actionY + 22, actionWidth, layout.actionHeight, friend ? '提示 · 禁用' : `提示${this.freeHint ? ' · 免费' : ''}`, () => this.hint(), 'magenta', { fontSize: 19, radius: 20, disabled: friend, key: 'game-hint' });
    this.drawNeonButton(68 + actionWidth * 2, actionY + 22, actionWidth, layout.actionHeight, '重置本题', () => this.resetPuzzle(), 'violet', { fontSize: 19, radius: 20, key: 'game-reset' });

    this.drawNeonButton(44, layout.bottomY, 300, layout.bottomButtonHeight, '重新开始本关', () => this.restartMode(), 'cyan', { fontSize: 19, radius: 20, key: 'game-restart' });
    const leaveLabel = this.mode === 'campaign' ? '返回关卡' : friend ? '退出对战' : '返回首页';
    this.drawNeonButton(376, layout.bottomY, 300, layout.bottomButtonHeight, leaveLabel, () => this.backFromGame(), 'violet', { fontSize: 19, radius: 20, key: 'game-leave' });
    this.drawGamePanel(44, layout.footerY, this.width - 88, layout.footerHeight, 'dark', { radius: 20, shadow: false });
    this.drawFitText(`目标 24   ·   已使用 ${4 - this.cards.length}/4 个数字`, this.width / 2, layout.footerY + layout.footerHeight / 2, this.width - 110, uiFont(14, 500), GAME_UI.secondary);
  }

  cardRect(index, startX, startY, cardWidth, cardHeight, gapX, gapY) {
    const col = index % 2;
    const row = Math.floor(index / 2);
    return { x: startX + col * (cardWidth + gapX), y: startY + row * (cardHeight + gapY), width: cardWidth, height: cardHeight };
  }

  drawFriendResult() {
    this.buttons = [];
    this.pollFriendMatchProgress();
    const result = this.result || {};
    const match = result.matchResult || {
      outcome: result.passed ? 'win' : 'lose',
      player_solved: this.friendPlayerSolved,
      player_score: result.score || this.score,
      player_mistakes: result.mistakes || this.mistakes,
      player_elapsed: Math.max(0, this.friendTimeLimit() - this.timeLeft),
      opponent_solved: 0,
      opponent_score: 0,
      opponent_mistakes: 0,
      opponent_elapsed: 0,
    };
    const pending = match.outcome === 'pending';
    const outcome = pending ? 'pending' : match.outcome === 'win' ? 'win' : match.outcome === 'draw' ? 'draw' : 'lose';
    const outcomeTitle = pending ? '等待结算' : outcome === 'win' ? '本局胜利' : outcome === 'draw' ? '平局' : '本局惜败';
    const outcomeSubtitle = pending
      ? '正在等待对手提交最终成绩'
      : outcome === 'win'
        ? '你先完成全部题目，赢得本场对战'
        : outcome === 'draw'
          ? '双方完成进度相同，再来一局分出胜负'
          : '对手先完成全部题目，下次再赢回来';
    const variant = outcome === 'win' ? 'cyan' : outcome === 'draw' ? 'gold' : outcome === 'pending' ? 'violet' : 'magenta';
    const accent = outcome === 'win' ? GAME_UI.success : outcome === 'draw' ? GAME_UI.gold : outcome === 'pending' ? GAME_UI.violetLight : GAME_UI.magentaLight;
    const panelWidth = this.width - 76;
    const panelHeight = 500;
    const primaryHeight = 72;
    const secondaryHeight = 60;
    const contentHeight = panelHeight + 30 + primaryHeight + 16 + secondaryHeight;
    const top = this.pageTop() + 70;
    const bottom = this.visibleBottom(16);
    const panelY = Math.round(clamp(top + (bottom - top - contentHeight) / 2, this.screenContentTop(48), Math.max(this.screenContentTop(48), bottom - contentHeight)));
    const panelX = (this.width - panelWidth) / 2;
    this.drawGameHeader('对战结果', '‹ 首页', () => this.goHome(), pending ? '等待结算' : outcomeTitle);
    this.drawGamePanel(panelX, panelY, panelWidth, panelHeight, variant, {
      radius: 32,
      shadowColor: outcome === 'win' ? 'rgba(40,233,255,0.30)' : outcome === 'lose' ? 'rgba(255,80,205,0.28)' : 'rgba(154,100,255,0.24)',
      shadowBlur: 24,
      shadowOffsetY: 0,
      hotspot: { x: this.width / 2, y: panelY + 70, r: 270, color: `${accent}18` },
    });
    this.drawFitText(outcomeTitle, this.width / 2, panelY + 52, panelWidth - 100, uiFont(34, 900), accent);
    this.drawFitText(result.serverVerified ? '服务端已校验' : pending ? '等待服务端确认' : '本地对战结果', this.width / 2, panelY + 88, panelWidth - 120, uiFont(14, 700), result.serverVerified ? GAME_UI.success : GAME_UI.secondary);

    this.drawGamePanel(panelX + 24, panelY + 112, panelWidth - 48, 98, variant, { radius: 24, shadowBlur: 10, shadowOffsetY: 0, fill: 'rgba(7,8,48,0.55)' });
    this.drawFitText('本局得分', this.width / 2, panelY + 142, panelWidth - 100, uiFont(15, 600), GAME_UI.secondary);
    this.drawFitText(String(Math.max(0, Math.floor(Number(result.score ?? match.player_score) || 0))), this.width / 2, panelY + 180, panelWidth - 130, uiFont(48, 900), GAME_UI.text);

    const cardGap = 14;
    const cardWidth = (panelWidth - 48 - cardGap) / 2;
    const cardY = panelY + 232;
    const drawPlayerCard = (x, title, solved, score, mistakes, elapsed, cardVariant) => {
      this.drawGamePanel(x, cardY, cardWidth, 164, cardVariant, { radius: 22, shadowBlur: 9, shadowOffsetY: 0, fill: 'rgba(7,8,48,0.52)' });
      this.drawFitText(title, x + cardWidth / 2, cardY + 28, cardWidth - 20, uiFont(18, 900), this.accentColor(cardVariant));
      this.drawFitText(`${Math.max(0, Math.floor(Number(solved) || 0))}/${this.friendQuestionCount()} 题`, x + cardWidth / 2, cardY + 68, cardWidth - 20, uiFont(27, 900), GAME_UI.text);
      this.drawFitText(`${Math.max(0, Math.floor(Number(score) || 0))} 分 · ${Math.max(0, Number(elapsed) || 0).toFixed(1)} 秒`, x + cardWidth / 2, cardY + 105, cardWidth - 20, uiFont(13, 600), GAME_UI.secondary);
      this.drawFitText(`错误 ${Math.max(0, Math.floor(Number(mistakes) || 0))} 次`, x + cardWidth / 2, cardY + 137, cardWidth - 20, uiFont(13, 600), GAME_UI.muted);
    };
    drawPlayerCard(panelX + 24, '我', match.player_solved, match.player_score, match.player_mistakes, match.player_elapsed, 'cyan');
    drawPlayerCard(panelX + 24 + cardWidth + cardGap, this.friendOpponentName(), match.opponent_solved, match.opponent_score, match.opponent_mistakes, match.opponent_elapsed, 'magenta');

    this.drawFitText(outcomeSubtitle, this.width / 2, panelY + 425, panelWidth - 72, uiFont(15, 700), GAME_UI.text);
    if (safeNumber(result.rewardCoins) > 0) {
      this.drawFitText(`对战奖励 +${safeNumber(result.rewardCoins)} 金币`, this.width / 2, panelY + 459, panelWidth - 100, uiFont(16, 800), GAME_UI.gold);
    } else {
      this.drawFitText(result.reason || '本局已结束', this.width / 2, panelY + 459, panelWidth - 100, uiFont(13, 600), GAME_UI.muted);
    }

    const primaryY = panelY + panelHeight + 30;
    this.drawNeonButton(48, primaryY, this.width - 96, primaryHeight, pending ? '等待服务端结算' : '再来一局', () => {
      if (pending) this.triggerFeedback('info', '正在等待服务端确认结果');
      else this.restartMode();
    }, outcome === 'win' ? 'cyan' : 'violet', { fontSize: 23, radius: 25, disabled: pending, key: 'friend-result-retry' });
    const secondaryY = primaryY + primaryHeight + 16;
    this.drawNeonButton(48, secondaryY, (this.width - 114) / 2, secondaryHeight, '返回好友对战', () => this.showFriendLobby(), 'magenta', { fontSize: 18, radius: 20, key: 'friend-result-lobby' });
    this.drawNeonButton(66 + (this.width - 114) / 2, secondaryY, (this.width - 114) / 2, secondaryHeight, '分享战绩', () => {
      this.sharePayload(shareService.createMatchResultPayload(match, this.friendRoom));
    }, 'cyan', { fontSize: 18, radius: 20, key: 'friend-result-share' });
  }

  drawResult() {
    if (this.mode === 'friend') {
      this.drawFriendResult();
      return;
    }
    this.buttons = [];
    this.pollFriendMatchProgress();
    const result = this.result || {};
    this.drawGameHeader('本局结算', '‹ 首页', () => this.goHome(), result.passed ? '挑战完成' : '继续练习');
    const resultPanelHeight = this.mode === 'friend' ? 690 : 650;
    const resultPrimaryHeight = 78;
    const resultSecondaryHeight = 66;
    const resultContentHeight = resultPanelHeight + 42 + resultPrimaryHeight + 18 + resultSecondaryHeight;
    const resultTop = this.pageTop() + 70;
    const resultBottom = this.visibleBottom(16);
    const centeredPanelY = resultTop + (resultBottom - resultTop - resultContentHeight) / 2;
    const panelY = Math.round(clamp(centeredPanelY, this.screenContentTop(48), Math.max(this.screenContentTop(48), resultBottom - resultContentHeight)));
    const variant = this.mode === 'friend' ? 'magenta' : result.passed ? 'cyan' : 'violet';
    this.drawGamePanel(38, panelY, this.width - 76, resultPanelHeight, variant, {
      radius: 32,
      shadowColor: result.passed ? 'rgba(40,233,255,0.24)' : 'rgba(160,100,255,0.20)',
      shadowBlur: 20,
      shadowOffsetY: 0,
      hotspot: { x: this.width / 2, y: panelY + 78, r: 250, color: result.passed ? 'rgba(40,233,255,0.10)' : 'rgba(255,80,205,0.08)' },
    });
    this.drawFitText(result.passed ? '挑战成功' : '再试一次', this.width / 2, panelY + 78, this.width - 130, uiFont(39, 900), result.passed ? GAME_UI.success : GAME_UI.magentaLight);
    this.drawFitText(String(result.score || 0), this.width / 2, panelY + 178, this.width - 160, uiFont(72, 900), GAME_UI.text);
    this.drawFitText(`得分 · ${result.reason || ''}`, this.width / 2, panelY + 232, this.width - 130, uiFont(18, 500), GAME_UI.secondary);
    if (result.passed) {
      const starCount = clamp(safeNumber(result.stars, 0), 0, 3);
      for (let index = 0; index < 3; index += 1) {
        this.drawStarIcon(this.width / 2 - 54 + index * 54, panelY + 303, 0.42, index < starCount ? GAME_UI.gold : 'rgba(255,211,77,0.26)');
      }
      this.drawResultEffect(this.width / 2, panelY + 303, this.equippedCosmetic('result'), starCount);
      if (result.starSummary) this.drawFitText(result.starSummary, this.width / 2, panelY + 333, this.width - 96, uiFont(12, 600), GAME_UI.secondary);
      if (Array.isArray(result.starDetails) && result.starDetails.length) {
        const failed = result.starDetails.filter((item) => !item.met).slice(0, 3);
        failed.forEach((item, index) => {
          const prefix = item.star >= 3 ? '三星' : item.star === 2 ? '二星' : '一星';
          this.drawFitText(`${prefix}未达成：${item.label}`, this.width / 2, panelY + 358 + index * 22, this.width - 104, uiFont(12, 600), GAME_UI.muted);
        });
      }
    } else {
      this.addButtonHit(this.width / 2 - 64, panelY + 243, 128, 128, () => { this.resultHelpPopup = true; }, { key: 'result-help' });
      this.drawQuestionIcon(this.width / 2, panelY + 303, 0.92);
    }
    const failedStarCount = result.passed && Array.isArray(result.starDetails)
      ? Math.min(3, result.starDetails.filter((item) => !item.met).length)
      : 0;
    const statY = panelY + (failedStarCount ? 390 + failedStarCount * 22 : 352);
    this.drawStatCard(72, statY, 182, 82, '最高连击', String(result.combo || 0), 'gold');
    this.drawStatCard(284, statY, 182, 82, '错误次数', String(result.mistakes || 0), result.mistakes ? 'magenta' : 'cyan');
    this.drawStatCard(496, statY, 182, 82, '金币奖励', `+${safeNumber(result.rewardCoins)}`, 'gold');
    let noteY = statY + 116;
    if (this.mode === 'endless') {
      const reached = this.currentQuestion + (result.passed ? 1 : 0);
      this.drawFitText(`无尽阶段 ${Math.floor(Math.max(0, reached - 1) / 3) + 1} · 全局答题 ${reached}`, this.width / 2, noteY, this.width - 130, uiFont(17, 800), GAME_UI.violetLight);
      noteY += 36;
    }
    if (this.mode === 'friend' && result.matchResult) {
      const match = result.matchResult;
      const outcomeText = match.outcome === 'pending' ? '等待好友完成' : match.outcome === 'win' ? '胜利' : match.outcome === 'draw' ? '平局' : '惜败，再来一局';
      const verificationLabel = result.serverVerified ? '服务端已校验' : match.outcome === 'pending' ? '等待服务端结算' : '本地体验结果';
      this.drawGamePanel(72, noteY, this.width - 144, 126, 'magenta', { radius: 22, shadowBlur: 8, shadowOffsetY: 0 });
      this.drawFitText(`好友对战：${outcomeText}`, this.width / 2, noteY + 31, this.width - 190, uiFont(21, 900), match.outcome === 'win' ? GAME_UI.success : match.outcome === 'draw' ? GAME_UI.gold : GAME_UI.magentaLight);
      this.drawFitText(`我 ${match.player_solved}/${this.friendQuestionCount()} 题 · ${match.player_score} 分 · ${match.player_elapsed.toFixed(1)} 秒 · 错 ${match.player_mistakes || 0}`, this.width / 2, noteY + 62, this.width - 190, uiFont(13, 500), GAME_UI.secondary);
      this.drawFitText(`${this.friendOpponentName()} ${match.opponent_solved}/${this.friendQuestionCount()} 题 · ${match.opponent_score} 分 · ${match.opponent_elapsed.toFixed(1)} 秒 · 错 ${match.opponent_mistakes || 0}`, this.width / 2, noteY + 86, this.width - 190, uiFont(13, 500), GAME_UI.secondary);
      this.drawFitText(verificationLabel, this.width / 2, noteY + 110, this.width - 190, uiFont(12, 700), result.serverVerified ? GAME_UI.success : GAME_UI.muted);
      noteY += 142;
    }
    if (Array.isArray(result.bonusLabels) && result.bonusLabels.length) {
      const labels = result.bonusLabels.map(uiSafeText).filter(Boolean).slice(0, 4);
      labels.forEach((label, index) => this.drawFitText(label, this.width / 2, noteY + index * 24, this.width - 130, uiFont(14, 700), index === 0 ? GAME_UI.success : GAME_UI.gold));
    } else if (result.rewardCoins) {
      this.drawFitText(`${this.mode === 'friend' ? '对战奖励' : '奖励'} +${result.rewardCoins} 金币`, this.width / 2, noteY, this.width - 130, uiFont(17, 800), GAME_UI.gold);
    }
    const primaryY = panelY + resultPanelHeight + 42;
    const pendingFriendResult = this.mode === 'friend' && result.matchResult && result.matchResult.outcome === 'pending';
    this.drawNeonButton(48, primaryY, this.width - 96, resultPrimaryHeight, pendingFriendResult ? '等待对手结算' : result.next ? (result.levelComplete ? '下一关' : '下一题') : '返回首页', () => {
      if (pendingFriendResult) {
        this.triggerFeedback('info', '正在等待服务端确认结果');
      } else if (!result.next) this.goHome();
      else if (result.levelComplete) this.startCampaign(this.currentLevel + 1);
      else this.nextQuestion();
    }, result.passed ? 'cyan' : 'violet', { fontSize: 24, radius: 26, disabled: pendingFriendResult, key: 'result-primary' });
    const resultActionY = primaryY + resultPrimaryHeight + 18;
    const dailyCompleted = this.mode === 'daily' && storage.isDailyCompleted(this.progress, storage.todayKey());
    this.drawNeonButton(48, resultActionY, (this.width - 114) / 2, resultSecondaryHeight, pendingFriendResult ? '返回好友对战' : dailyCompleted ? '今日已完成' : '再来一局', () => {
      if (pendingFriendResult) this.showFriendLobby();
      else if (dailyCompleted) this.goHome();
      else this.restartMode();
    }, 'magenta', { fontSize: 20, radius: 21, key: 'result-retry' });
    this.drawNeonButton(66 + (this.width - 114) / 2, resultActionY, (this.width - 114) / 2, resultSecondaryHeight, '分享成绩', () => {
      if (this.mode === 'friend' && result.matchResult) this.sharePayload(shareService.createMatchResultPayload(result.matchResult, this.friendRoom));
      else this.sharePayload({ title: '来挑战《三火算术练习》！' });
    }, 'cyan', { fontSize: 20, radius: 21, key: 'result-share' });
  }

  drawResultEffect(cx, cy, style, starCount) {
    const preview = style && style.preview || 'classic';
    if (preview === 'classic' || starCount <= 0) return;
    const pulse = 0.72 + Math.sin(Date.now() / 240) * 0.12;
    const colors = preview === 'fireworks'
      ? [GAME_UI.gold, GAME_UI.cyan, GAME_UI.magenta, GAME_UI.violet]
      : [GAME_UI.cyan, GAME_UI.magenta, GAME_UI.gold];
    const count = preview === 'fireworks' ? 12 : 8;
    for (let index = 0; index < count; index += 1) {
      const angle = (Math.PI * 2 * index) / count;
      const radius = (preview === 'fireworks' ? 88 : 70) * pulse;
      this.drawSparkle(cx + Math.cos(angle) * radius, cy + Math.sin(angle) * radius * 0.52, preview === 'fireworks' ? 5 : 3.5, colors[index % colors.length], 0.75);
    }
    if (preview === 'burst') {
      this.drawThinOrbit(cx, cy, 218, 78, -0.08, 'rgba(255,211,77,0.64)', 'rgba(255,211,77,0.24)');
    }
  }

  drawHeader(title, backAction) {
    this.drawGameHeader(title, '‹ 返回', backAction);
  }

  drawModalFrame(x, y, width, height, title, subtitle = '', variant = 'violet', closeAction = null) {
    this.drawGamePanel(x, y, width, height, 'modal', {
      radius: 28,
      stroke: variant === 'cyan' ? 'rgba(45,230,255,0.70)' : variant === 'magenta' ? 'rgba(255,80,205,0.68)' : 'rgba(190,180,255,0.65)',
      shadowColor: 'rgba(0,0,25,0.50)',
      shadowBlur: 26,
      shadowOffsetY: 10,
      hotspot: { x: x + width * 0.25, y: y + 42, r: width * 0.7, color: variant === 'cyan' ? 'rgba(40,233,255,0.08)' : variant === 'magenta' ? 'rgba(255,80,205,0.08)' : 'rgba(154,100,255,0.08)' },
    });
    const accent = this.accentColor(variant);
    this.ctx.save();
    this.ctx.globalAlpha = 0.86;
    this.ctx.shadowColor = accent;
    this.ctx.shadowBlur = 10;
    this.roundRect(x + 28, y + 68, 74, 4, 999, accent, null, 0);
    this.ctx.restore();
    this.drawProgressLine(x + 26, y + 106, width - 52, 1, variant === 'magenta' ? 'cyan' : 'gold', 2);
    this.drawFitText(title, x + 28, y + 48, width - 122, uiFont(26, 900), GAME_UI.text, 'left');
    if (subtitle) this.drawFitText(subtitle, x + 28, y + 88, width - 122, uiFont(14, 500), GAME_UI.secondary, 'left');
    this.drawModalClose(x + width - 61, y + 18, closeAction || (() => { this.popup = ''; }));
  }

  drawModalClose(x, y, action) {
    this.addButtonHit(x, y, 42, 42, action, { key: `modal-close-${x}-${y}` });
    this.drawGlassCard(x, y, 42, 42, 21, this.glassFill(x, y, 42, 42, 'violet'), 'rgba(200,164,255,0.76)', {
      lineWidth: 1.6,
      shadowColor: 'rgba(154,100,255,0.48)',
      shadowBlur: 10,
      shadowOffsetY: 0,
      innerAlpha: 0.08,
    });
    const ctx = this.ctx;
    ctx.save();
    ctx.strokeStyle = '#fff4ff';
    ctx.lineWidth = 3;
    ctx.lineCap = 'round';
    ctx.shadowColor = 'rgba(255,255,255,0.45)';
    ctx.shadowBlur = 5;
    ctx.beginPath();
    ctx.moveTo(x + 15, y + 15);
    ctx.lineTo(x + 27, y + 27);
    ctx.moveTo(x + 27, y + 15);
    ctx.lineTo(x + 15, y + 27);
    ctx.stroke();
    ctx.restore();
  }

  drawThemePreview(x, y, size, skin, active = false) {
    const theme = (skin && skin.theme) || {};
    const accent = active ? GAME_UI.gold : (theme.accent || GAME_UI.cyan);
    const fill = this.ctx.createLinearGradient(x, y, x + size, y + size);
    fill.addColorStop(0, theme.surface_2 || 'rgba(22,62,130,0.72)');
    fill.addColorStop(1, theme.surface || 'rgba(38,22,116,0.72)');
    this.drawGlassCard(x, y, size, size, 22, fill, accent, {
      lineWidth: active ? 3 : 2,
      shadowColor: `${accent}66`,
      shadowBlur: active ? 18 : 12,
      shadowOffsetY: 0,
      innerAlpha: 0.12,
    });
    this.drawSparkle(x + size * 0.23, y + size * 0.28, 4.5, GAME_UI.cyanLight, 0.9);
    this.drawSparkle(x + size * 0.78, y + size * 0.22, 3.2, GAME_UI.violetLight, 0.75);
    this.drawSparkle(x + size * 0.76, y + size * 0.72, 4.2, GAME_UI.goldLight, 0.72);
    this.drawText('24', x + size / 2, y + size / 2 + size * 0.05, uiFont(size * 0.41, 900), theme.card || GAME_UI.text);
  }

  drawProgressTaskCard(x, y, width, height, task, claimed = false) {
    const ratio = clamp(safeNumber(task.value) / Math.max(1, safeNumber(task.target, 1)), 0, 1);
    this.drawGamePanel(x, y, width, height, claimed ? 'gold' : 'violet', {
      radius: 18,
      fill: claimed ? this.glassFill(x, y, width, height, 'gold') : this.glassFill(x, y, width, height, 'violet'),
      stroke: claimed ? 'rgba(255,211,77,0.72)' : 'rgba(160,120,255,0.42)',
      shadow: false,
    });
    this.drawText(claimed ? '完成' : `${task.value}/${task.target}`, x + 54, y + 38, uiFont(claimed ? 18 : 19, 900), claimed ? GAME_UI.success : GAME_UI.cyan);
    this.drawFitText(task.title, x + 104, y + 27, width - 138, uiFont(17, 800), GAME_UI.text, 'left');
    this.drawFitText(`奖励 +${task.reward} 金币${claimed ? ' · 已领取' : ''}`, x + 104, y + 54, width - 138, uiFont(13, 500), claimed ? GAME_UI.success : GAME_UI.gold, 'left');
    this.drawProgressLine(x + 104, y + 70, width - 132, ratio, claimed ? 'gold' : 'cyan', 8);
  }

  drawAchievementBadge(x, y, unlocked, index = 0) {
    if (unlocked) {
      this.drawStarIcon(x, y, 0.48, GAME_UI.gold);
      return;
    }
    this.drawQuestionIcon(x, y, 0.72 + (index % 2) * 0.04);
  }

  drawDashboardMetric(x, y, drawIcon, label, value, variant = 'cyan') {
    const accent = this.accentColor(variant);
    this.ctx.save();
    if (drawIcon) drawIcon(x + 28, y + 28, 0.48, accent);
    this.ctx.restore();
    this.drawText(label, x + 78, y + 18, uiFont(14, 500), GAME_UI.secondary, 'left');
    this.drawText(value, x + 78, y + 48, uiFont(20, 900), GAME_UI.text, 'left');
  }

  drawSpeakerIcon(cx, cy, scale = 1, color = GAME_UI.cyan) {
    const ctx = this.ctx;
    ctx.save();
    ctx.fillStyle = color;
    ctx.strokeStyle = color;
    ctx.lineWidth = 3 * scale;
    ctx.lineCap = 'round';
    ctx.lineJoin = 'round';
    ctx.shadowColor = color;
    ctx.shadowBlur = 10;
    ctx.beginPath();
    ctx.moveTo(cx - 27 * scale, cy - 13 * scale);
    ctx.lineTo(cx - 13 * scale, cy - 13 * scale);
    ctx.lineTo(cx + 7 * scale, cy - 29 * scale);
    ctx.lineTo(cx + 7 * scale, cy + 29 * scale);
    ctx.lineTo(cx - 13 * scale, cy + 13 * scale);
    ctx.lineTo(cx - 27 * scale, cy + 13 * scale);
    ctx.closePath();
    ctx.fill();
    ctx.beginPath();
    ctx.arc(cx + 12 * scale, cy, 17 * scale, -0.62, 0.62);
    ctx.stroke();
    ctx.beginPath();
    ctx.arc(cx + 13 * scale, cy, 29 * scale, -0.56, 0.56);
    ctx.globalAlpha = 0.55;
    ctx.stroke();
    ctx.restore();
  }

  drawToggleSwitch(x, y, on, variant = 'cyan') {
    const accent = on ? this.accentColor(variant) : 'rgba(180,175,230,0.36)';
    const fill = on ? this.glassFill(x, y, 80, 36, variant) : 'rgba(24,22,72,0.82)';
    this.drawGlassCard(x, y, 80, 36, 18, fill, accent, {
      lineWidth: 1.4,
      shadowColor: on ? `${accent}55` : 'rgba(0,0,0,0.20)',
      shadowBlur: on ? 10 : 3,
      shadowOffsetY: 0,
      innerAlpha: 0.06,
    });
    const knobX = x + (on ? 58 : 22);
    this.ctx.save();
    this.ctx.shadowColor = on ? `${accent}bb` : 'rgba(255,255,255,0.20)';
    this.ctx.shadowBlur = on ? 9 : 4;
    this.ctx.beginPath();
    this.ctx.arc(knobX, y + 18, 13, 0, Math.PI * 2);
    this.ctx.fillStyle = on ? '#ffffff' : 'rgba(200,205,235,0.78)';
    this.ctx.fill();
    this.ctx.restore();
  }

  drawSettingsToggleRow(x, y, width, height, label, detail, on, action, key, variant = 'cyan') {
    const activeVariant = on ? variant : 'violet';
    this.addButtonHit(x, y, width, height, action, { key });
    this.drawGamePanel(x, y, width, height, activeVariant, {
      radius: 20,
      fill: on ? this.glassFill(x, y, width, height, activeVariant) : 'rgba(28,25,78,0.70)',
      stroke: on ? `${this.accentColor(activeVariant)}aa` : 'rgba(180,170,255,0.28)',
      shadowColor: on ? `${this.accentColor(activeVariant)}33` : 'rgba(0,0,0,0.16)',
      shadowBlur: on ? 12 : 4,
      shadowOffsetY: 0,
    });
    this.drawSpeakerIcon(x + 42, y + height / 2, 0.42, on ? this.accentColor(activeVariant) : GAME_UI.faint);
    this.drawText(label, x + 86, y + 28, uiFont(18, 900), GAME_UI.text, 'left');
    this.drawFitText(detail, x + 86, y + 55, width - 220, uiFont(12, 500), on ? GAME_UI.secondary : GAME_UI.muted, 'left');
    this.drawText(on ? '开启' : '关闭', x + width - 126, y + 30, uiFont(13, 800), on ? GAME_UI.success : GAME_UI.secondary);
    this.drawToggleSwitch(x + width - 98, y + 42, on, activeVariant);
  }

  drawSettingVolumeBlock(x, y, width, label, ratio, variant = 'cyan') {
    const color = this.accentColor(variant);
    const volumeKey = label === '背景音乐音量' ? 'music' : label === '按键音效音量' ? 'sfx' : '';
    if (volumeKey) {
      if (!this.volumeDragAreas) this.volumeDragAreas = {};
      this.volumeDragAreas[volumeKey] = { x, y, width, height: 76, barX: x + 22, barWidth: Math.max(1, width - 44) };
      this.addHitArea(x, y, width, 76, () => {}, { key: `settings-volume-${volumeKey}`, dragType: volumeKey });
    }
    this.drawGamePanel(x, y, width, 76, 'dark', {
      radius: 18,
      fill: 'rgba(21,19,69,0.58)',
      stroke: 'rgba(180,170,255,0.22)',
      shadow: false,
    });
    this.drawText(label, x + 22, y + 25, uiFont(14, 700), GAME_UI.secondary, 'left');
    this.drawText(`${Math.round(clamp(ratio, 0, 1) * 100)}%`, x + width - 22, y + 25, uiFont(14, 900), color, 'right');
    this.drawVolumeBar(x + 22, y + 48, width - 44, ratio, color);
  }

  drawFeatureMenuCard(key, x, y, width, height, title, subtitle, variant, drawIcon, action) {
    const accent = this.accentColor(variant);
    this.addButtonHit(x, y, width, height, action, { key });
    this.drawGamePanel(x, y, width, height, variant, {
      radius: 22,
      stroke: `${accent}aa`,
      shadowColor: `${accent}36`,
      shadowBlur: 12,
      shadowOffsetY: 0,
      hotspot: { x: x + 58, y: y + 38, r: 130, color: `${accent}16` },
    });
    this.drawGamePanel(x + 18, y + 21, 62, 62, variant, {
      radius: 18,
      fill: 'rgba(255,255,255,0.07)',
      stroke: `${accent}88`,
      shadow: false,
      innerAlpha: 0.05,
    });
    if (drawIcon) drawIcon(x + 49, y + 52, accent);
      this.drawFitText(title, x + 96, y + 39, width - 126, uiFont(18, 900), GAME_UI.text, 'left');
      this.drawFitText(subtitle, x + 96, y + 68, width - 126, uiFont(12, 500), GAME_UI.secondary, 'left');
    this.drawText('进入', x + width - 24, y + height - 24, uiFont(12, 900), accent, 'right');
  }

  drawFriendLobby() {
    if (this.friendLobbyView !== 'room' && !this.friendRoomFromInvite && !this.friendRoom) {
      this.drawFriendEntry();
      return;
    }
    this.buttons = [];
    this.pollBackendFriendRoom();
    this.drawGameHeader('好友对战', '‹ 返回', () => this.goHome());
    const room = this.friendRoom || friendMatch.createLocalRoom();
    const roomRules = Object.assign({}, friendMatch.rules(), room.rules || {});
    const localMode = this.friendRoomBackendStatus === 'local' || this.friendLocalFallback;
    const players = Array.isArray(room.players) ? room.players : [];
    const currentPlayerID = this.backendAuth && this.backendAuth.user
      ? String(this.backendAuth.user.id || this.backendAuth.user.user_id || '')
      : 'local-player';
    const opponentPlayer = players.find((player) => String(player && (player.user_id || player.id) || '') !== currentPlayerID);
    const selfReady = this.friendSelfReady === undefined ? true : Boolean(this.friendSelfReady);
    const opponentReady = opponentPlayer ? Boolean(opponentPlayer.ready) : false;
    const canStart = (localMode || this.friendRoomBackendStatus === 'ready')
      && friendMatch.isRoomReady(room)
      && selfReady
      && opponentReady;
    const panelY = this.screenContentTop(92);
    // 房间信息卡在手机上采用更紧凑的纵向比例，避免“已准备”贴到卡片底边或被裁切。
    const compact = this.visibleHeight < 1500;
    const roomPanelHeight = compact ? 344 : 372;
    const titleOffset = compact ? 60 : 72;
    const subtitleOffset = compact ? 101 : 118;
    const codeOffset = compact ? 181 : 210;
    const metaOffset = compact ? 230 : 265;
    const rulesOffset = compact ? 278 : 322;
    const statusOffset = compact ? 320 : 364;
    this.drawGamePanel(58, panelY, this.width - 116, roomPanelHeight, 'magenta', {
      radius: 31,
      stroke: 'rgba(255,80,205,0.86)',
      shadowColor: 'rgba(255,80,205,0.25)',
      shadowBlur: 22,
      shadowOffsetY: 0,
      hotspot: { x: this.width / 2, y: panelY + 70, r: 270, color: 'rgba(255,80,205,0.10)' },
    });
    this.drawSparkle(628, panelY + 42, 4, GAME_UI.magentaLight, 0.92);
    this.drawSparkle(122, panelY + 132, 3, GAME_UI.cyanLight, 0.55);
    this.drawFitText('同题竞速', this.width / 2, panelY + titleOffset, this.width - 180, uiFont(compact ? 31 : 33, 900), GAME_UI.magentaLight);
    this.drawFitText('分享房间给好友，双方使用同一组题目', this.width / 2, panelY + subtitleOffset, this.width - 150, uiFont(compact ? 15 : 16, 500), GAME_UI.secondary);
    this.ctx.save();
    this.ctx.shadowColor = 'rgba(255,80,205,0.72)';
    this.ctx.shadowBlur = 18;
    this.drawText(room.room_code, this.width / 2, panelY + codeOffset, uiFont(compact ? 52 : 55, 900), GAME_UI.text);
    this.ctx.restore();
    this.drawFitText(`${roomRules.question_count || 8} 道题 · ${roomRules.time_limit || 120} 秒 · ${localMode ? '本地演示房间' : '服务端同题房间'}`, this.width / 2, panelY + metaOffset, this.width - 150, uiFont(compact ? 14 : 15, 500), GAME_UI.secondary);
    this.drawFitText('同一套题目 · 禁止提示 · 答错扣 5 秒', this.width / 2, panelY + rulesOffset, this.width - 150, uiFont(compact ? 16 : 18, 900), GAME_UI.text);
    const selfReadyLabel = selfReady ? '\u4f60\uff1a\u5df2\u51c6\u5907' : '\u4f60\uff1a\u5f85\u51c6\u5907';
    const opponentReadyLabel = opponentPlayer
      ? `${String(opponentPlayer.nickname || opponentPlayer.name || '\u5bf9\u624b')}\uff1a${opponentReady ? '\u5df2\u51c6\u5907' : '\u5f85\u51c6\u5907'}`
      : '\u5bf9\u624b\uff1a\u7b49\u5f85\u52a0\u5165';
    this.drawFitText(`${selfReadyLabel}  \u00b7  ${opponentReadyLabel}`, this.width / 2, panelY + statusOffset, this.width - 150, uiFont(compact ? 15 : 16, 800), selfReady && opponentReady ? GAME_UI.success : GAME_UI.secondary);
    const shareY = panelY + roomPanelHeight + (compact ? 68 : 84);
    this.drawNeonButton(58, shareY, this.width - 116, 70, '分享给微信好友', () => this.sharePayload(shareService.createFriendRoomPayload(room)), 'cyan', { fontSize: 22, radius: 30, key: 'friend-share' });
    this.drawNeonButton(58, shareY + 102, this.width - 116, 70, selfReady ? '取消准备' : '准备', () => this.toggleFriendReady(), 'cyan', { fontSize: 22, radius: 30, disabled: this.friendReadyRequestInFlight, key: 'friend-ready' });
    this.drawNeonButton(58, shareY + 204, this.width - 116, 70, `开始 ${roomRules.question_count || 8} 题同题竞速`, () => this.startFriend(), 'magenta', { fontSize: 22, radius: 30, disabled: !canStart, key: 'friend-start' });
    const lobbyHint = this.friendRoomBackendStatus === 'loading'
      ? '正在连接好友房间…'
      : this.friendRoomBackendStatus === 'error'
      ? '好友房间连接失败，请返回后重试'
      : !selfReady
        ? '点击准备，等待对手一起开始'
        : !opponentReady
          ? '等待对手加入并准备'
          : canStart
            ? (localMode ? '对手已准备，可以开始体验' : '双方已准备，将使用同一组题目')
            : '正在准备对战…';
    this.drawFitText(lobbyHint, this.width / 2, Math.min(shareY + 314, this.visibleBottom(26)), this.width - 96, uiFont(14, 500), GAME_UI.muted);
  }

  toggleFriendReady() {
    if (!this.friendRoom || this.friendReadyRequestInFlight) return;
    const nextReady = !Boolean(this.friendSelfReady);
    const roomCode = String(this.friendRoom.room_code || '').trim();
    const localMode = this.friendRoomBackendStatus === 'local' || this.friendLocalFallback;
    this.friendSelfReady = nextReady;
    const players = Array.isArray(this.friendRoom.players) ? this.friendRoom.players : [];
    const currentPlayerID = this.backendAuth && this.backendAuth.user
      ? String(this.backendAuth.user.id || this.backendAuth.user.user_id || '')
      : 'local-player';
    const self = players.find((player) => String(player && (player.user_id || player.id) || '') === currentPlayerID) || players[0];
    if (self) self.ready = nextReady;
    if (localMode || !apiClient.readyFriendRoom || !roomCode) {
      this.triggerFeedback('success', nextReady ? '\u5df2\u51c6\u5907' : '\u5df2\u53d6\u6d88\u51c6\u5907');
      return;
    }
    this.friendReadyRequestInFlight = true;
    apiClient.readyFriendRoom(roomCode, nextReady).then((room) => {
      if (room && room.room_code) {
        this.friendRoom = room;
        this.friendRules = Object.assign({}, friendMatch.rules(), room.rules || {});
      }
      this.triggerFeedback('success', nextReady ? '\u5df2\u51c6\u5907' : '\u5df2\u53d6\u6d88\u51c6\u5907');
    }).catch((error) => {
      this.friendSelfReady = !nextReady;
      if (self) self.ready = !nextReady;
      if (this.isFriendRoomTerminalError(error)) this.markFriendRoomExpired('expired');
      else this.beginFriendReconnect(error, 'ready');
      this.triggerFeedback('error', '\u51c6\u5907\u72b6\u6001\u66f4\u65b0\u5931\u8d25');
      try { if (typeof console !== 'undefined' && console.warn) console.warn('[friend-ready]', error); } catch (logError) { /* ready failure is shown in the UI */ }
    }).then(() => { this.friendReadyRequestInFlight = false; });
  }

  drawFriendEntry() {
    this.buttons = [];
    this.drawGameHeader('\u597d\u53cb\u5bf9\u6218', '\u2039 \u8fd4\u56de', () => this.goHome());
    const panelY = this.screenContentTop(96);
    const width = this.width - 116;
    const roomInput = friendMatch.sanitizeRoomCode(this.friendRoomInput || '');
    this.drawGamePanel(58, panelY, width, 220, 'magenta', {
      radius: 30,
      shadowColor: 'rgba(255,80,205,0.25)',
      shadowBlur: 22,
      shadowOffsetY: 0,
      hotspot: { x: this.width / 2, y: panelY + 48, r: 250, color: 'rgba(255,80,205,0.10)' },
    });
    this.drawFitText('\u540c\u9898\u7ade\u901f', this.width / 2, panelY + 62, width - 90, uiFont(34, 900), GAME_UI.magentaLight);
    this.drawFitText('\u548c\u670b\u53cb\u6216\u540c\u65f6\u5728\u7ebf\u7684\u73a9\u5bb6\u6bd4\u4e00\u5c40', this.width / 2, panelY + 112, width - 84, uiFont(17, 600), GAME_UI.text);
    this.drawFitText('\u623f\u95f4\u5bf9\u6218\u53ef\u5206\u4eab\uff0c\u5feb\u901f\u5339\u914d\u4f1a\u81ea\u52a8\u8fdb\u5165\u5bf9\u5c40', this.width / 2, panelY + 160, width - 70, uiFont(14, 500), GAME_UI.secondary);

    // 将输入框、两个主按钮和规则说明作为一个整体放在剩余可视区域的中部，
    // 避免入口页控件集中在上方、底部留下大块空白；不同手机高度会自动重新计算。
    const actionStackHeight = 418;
    const actionAreaTop = panelY + 220;
    const actionAreaBottom = this.visibleBottom(74);
    const inputY = clamp(
      Math.round((actionAreaTop + actionAreaBottom - actionStackHeight) / 2),
      panelY + 254,
      Math.max(panelY + 254, actionAreaBottom - actionStackHeight),
    );
    this.drawFitText('\u8f93\u5165\u623f\u95f4\u7801', 84, inputY - 18, 180, uiFont(18, 800), GAME_UI.text, 'left');
    this.drawGamePanel(58, inputY, 390, 72, 'dark', {
      radius: 22,
      fill: 'rgba(8,8,48,0.82)',
      stroke: roomInput.length === 6 ? GAME_UI.cyan : 'rgba(255,255,255,0.22)',
      shadowColor: roomInput.length === 6 ? 'rgba(40,233,255,0.22)' : 'rgba(0,0,0,0.18)',
      shadowBlur: 12,
      shadowOffsetY: 0,
    });
    this.drawFitText(roomInput || '\u70b9\u51fb\u8f93\u51656\u4f4d\u6570\u5b57\u623f\u95f4\u7801', 78, inputY + 36, 350, uiFont(roomInput ? 29 : 17, 900), roomInput ? GAME_UI.cyanLight : GAME_UI.muted, 'left');
    this.addHitArea(58, inputY, 390, 72, () => this.focusFriendRoomInput(), { key: 'friend-room-input' });
    this.drawNeonButton(468, inputY, 224, 72, '\u52a0\u5165\u623f\u95f4', () => this.joinFriendRoomEntry(), 'cyan', { fontSize: 20, radius: 22, disabled: roomInput.length !== 6, key: 'friend-room-join' });

    const createY = inputY + 104;
    this.drawNeonButton(58, createY, width, 72, '\u521b\u5efa\u623f\u95f4\u5e76\u5206\u4eab', () => this.createFriendRoomEntry(), 'magenta', { fontSize: 22, radius: 28, key: 'friend-room-create' });
    this.drawNeonButton(58, createY + 96, width, 72, '\u5feb\u901f\u5339\u914d', () => this.startFriendMatchmaking(), 'violet', { fontSize: 22, radius: 28, key: 'friend-matchmaking' });
    this.drawGamePanel(58, createY + 204, width, 110, 'dark', { radius: 22, shadow: false, stroke: 'rgba(255,255,255,0.13)' });
    this.drawFitText('\u5339\u914d\u89c4\u5219', this.width / 2, createY + 236, width - 60, uiFont(17, 900), GAME_UI.gold);
    this.drawFitText('\u7b49\u5f85\u6700\u591a15\u79d2\uff0c\u5339\u914d\u8303\u56f4\u4f1a\u9010\u6b65\u6269\u5927', this.width / 2, createY + 278, width - 52, uiFont(15, 600), GAME_UI.secondary);
  }

  drawFriendMatchmaking() {
    this.buttons = [];
    this.drawGameHeader('\u5feb\u901f\u5339\u914d', '\u2039 \u53d6\u6d88', () => this.cancelFriendMatchmaking());
    const state = this.friendMatchmaking || { status: 'searching', startedAt: Date.now() };
    const panelY = this.screenContentTop(156);
    const width = this.width - 116;
    const elapsed = Math.max(0, Math.floor((Date.now() - Number(state.startedAt || Date.now())) / 1000));
    const remaining = Math.max(0, friendMatch.MATCHMAKING_TIMEOUT - elapsed);
    const isBotReady = state.status === 'bot_ready';
    const isMatched = state.status === 'matched';
    this.drawGamePanel(58, panelY, width, 500, isBotReady ? 'gold' : 'violet', {
      radius: 32,
      shadowColor: isBotReady ? 'rgba(255,205,74,0.28)' : 'rgba(155,78,255,0.26)',
      shadowBlur: 24,
      shadowOffsetY: 0,
      hotspot: { x: this.width / 2, y: panelY + 82, r: 240, color: isBotReady ? 'rgba(255,204,82,0.10)' : 'rgba(154,100,255,0.12)' },
    });
    this.drawFitText(isBotReady ? '\u627e\u5230\u5bf9\u624b' : isMatched ? '\u5339\u914d\u6210\u529f' : '\u6b63\u5728\u5bfb\u627e\u5bf9\u624b', this.width / 2, panelY + 70, width - 70, uiFont(30, 900), isBotReady ? GAME_UI.goldLight : GAME_UI.violetLight);
    this.ctx.save();
    this.ctx.strokeStyle = isBotReady ? GAME_UI.gold : GAME_UI.cyan;
    this.ctx.lineWidth = 8;
    this.ctx.lineCap = 'round';
    this.ctx.beginPath();
    this.ctx.arc(this.width / 2, panelY + 190, 58, -Math.PI / 2, -Math.PI / 2 + Math.PI * 1.65, false);
    this.ctx.stroke();
    this.ctx.restore();
    if (isBotReady) {
      const profile = friendMatch.botProfile(this.friendBotDifficulty);
      this.drawFitText(profile.name, this.width / 2, panelY + 294, width - 90, uiFont(28, 900), GAME_UI.text);
      this.drawFitText('\u5bf9\u624b\u5df2\u51c6\u5907', this.width / 2, panelY + 338, width - 100, uiFont(18, 700), GAME_UI.goldLight);
      this.drawFitText('\u4e0d\u7528\u7b49\u5f85\uff0c\u73b0\u5728\u5c31\u5f00\u59cb\u7b54\u9898', this.width / 2, panelY + 386, width - 90, uiFont(16, 500), GAME_UI.secondary);
    } else if (isMatched) {
      this.drawFitText('\u5bf9\u624b\u5df2\u51c6\u5907\uff0c\u5373\u5c06\u5f00\u59cb', this.width / 2, panelY + 320, width - 80, uiFont(19, 800), GAME_UI.cyanLight);
    } else {
      this.drawFitText(`\u5df2\u7b49\u5f85 ${elapsed} \u79d2`, this.width / 2, panelY + 306, width - 80, uiFont(24, 900), GAME_UI.text);
      this.drawFitText(`\u6b63\u5728\u6269\u5927\u5339\u914d\u8303\u56f4\uff0c\u8fd8\u5269 ${remaining} \u79d2`, this.width / 2, panelY + 354, width - 80, uiFont(16, 600), GAME_UI.secondary);
      this.drawFitText('\u540c\u65f6\u70b9\u51fb\u5feb\u901f\u5339\u914d\u7684\u73a9\u5bb6\u4f1a\u4f18\u5148\u5339\u914d', this.width / 2, panelY + 400, width - 82, uiFont(14, 500), GAME_UI.muted);
    }
    if (isBotReady) {
      this.drawNeonButton(58, panelY + 536, width, 72, '\u5f00\u59cb\u5bf9\u6218', () => this.startQueuedFriendMatch(), 'gold', { fontSize: 22, radius: 28, key: 'friend-bot-start' });
    } else {
      this.drawNeonButton(58, panelY + 536, width, 72, '\u53d6\u6d88\u5339\u914d', () => this.cancelFriendMatchmaking(), 'violet', { fontSize: 22, radius: 28, key: 'friend-matchmaking-cancel' });
    }
  }

  focusFriendRoomInput() {
    this.friendInputKeyboardActive = true;
    const defaultValue = String(this.friendRoomInput || '');
    try {
      if (wx.showKeyboard) {
        wx.showKeyboard({ defaultValue, maxLength: 6, multiple: false, confirmType: 'done' });
        return;
      }
      if (wx.showModal) {
        wx.showModal({ title: '\u8f93\u5165\u623f\u95f4\u7801', content: defaultValue, editable: true, confirmText: '\u52a0\u5165', success: (result) => {
          this.friendInputKeyboardActive = false;
          this.friendRoomInput = friendMatch.sanitizeRoomCode(result && result.content || '');
          if (result && result.confirm && this.friendRoomInput.length === 6) this.joinFriendRoomEntry();
        } });
        return;
      }
    } catch (error) {
      this.friendInputKeyboardActive = false;
      this.triggerFeedback('error', '\u65e0\u6cd5\u6253\u5f00\u8f93\u5165\u6846');
    }
  }

  onFriendKeyboardInput(event) {
    if (!this.friendInputKeyboardActive || this.screen !== 'friend_lobby') return;
    this.friendRoomInput = friendMatch.sanitizeRoomCode(event && event.value || '');
  }

  onFriendKeyboardConfirm(event) {
    if (!this.friendInputKeyboardActive || this.screen !== 'friend_lobby') return;
    this.friendRoomInput = friendMatch.sanitizeRoomCode(event && event.value || this.friendRoomInput);
    this.friendInputKeyboardActive = false;
    if (wx.hideKeyboard) { try { wx.hideKeyboard(); } catch (error) { /* keyboard close is best effort */ } }
  }

  createFriendRoomEntry() {
    this.showFriendRoom('create');
  }

  joinFriendRoomEntry() {
    const code = friendMatch.sanitizeRoomCode(this.friendRoomInput);
    if (code.length !== 6) {
      this.triggerFeedback('error', '\u8bf7\u8f93\u51656\u4f4d\u623f\u95f4\u7801');
      return;
    }
    this.showFriendRoom('join', code);
  }

  showFriendRoom(kind = 'create', roomCode = '') {
    if (this.friendInputKeyboardActive && wx.hideKeyboard) { try { wx.hideKeyboard(); } catch (error) { /* close is best effort */ } }
    this.friendInputKeyboardActive = false;
    this.popup = '';
    this.friendLobbyView = 'room';
    this.friendRoomFromInvite = kind === 'join';
    this.friendLocalFallback = !(apiClient.isConfigured && apiClient.isConfigured());
    this.friendRoomBackendStatus = this.friendLocalFallback ? 'local' : 'idle';
    this.friendRoomBackendLoading = false;
    this.friendRoomLastPollAt = 0;
    this.friendRoom = friendMatch.createLocalRoom(kind === 'join' ? roomCode : '');
    this.friendRules = friendMatch.rules();
    this.friendBotDifficulty = 'standard';
    this.friendBotName = '';
    this.friendSelfReady = false;
    this.friendReadyRequestInFlight = false;
    this.friendStartRequestInFlight = false;
    this.friendServerStartAt = 0;
    this.friendCountdownActive = false;
    this.friendConnectionState = 'connected';
    this.friendReconnectStartedAt = 0;
    this.friendReconnectDeadline = 0;
    this.friendReconnectNextAt = 0;
    this.friendReconnectRequestInFlight = false;
    this.friendRoomRequestInFlight = false;
    this.friendRoomExpired = false;
    this.friendRoomError = '';
    this.friendMatch = null;
    this.friendMatchProgress = null;
    this.screen = 'friend_lobby';
    this.syncFriendRoomWithBackend();
  }

  startFriendMatchmaking() {
    if (this.friendInputKeyboardActive && wx.hideKeyboard) { try { wx.hideKeyboard(); } catch (error) { /* close is best effort */ } }
    this.friendInputKeyboardActive = false;
    const now = Date.now();
    this.friendMatchmakingRunId += 1;
    this.friendMatchmaking = {
      status: 'searching',
      startedAt: now,
      ticketId: `mm-${now}-${Math.floor(Math.random() * 100000)}`,
      ticketReady: false,
      apiStarted: false,
      matchedAt: 0,
      botReadyAt: 0,
    };
    this.friendMatchmakingLastPollAt = 0;
    this.friendMatchmakingRequestInFlight = false;
    this.friendMatchmakingLocal = !(apiClient.isConfigured && apiClient.isConfigured());
    this.friendMatchmakingError = '';
    this.screen = 'friend_matchmaking';
  }

  updateFriendMatchmaking() {
    if (this.screen !== 'friend_matchmaking' || !this.friendMatchmaking) return;
    const state = this.friendMatchmaking;
    const now = Date.now();
    const elapsed = Math.max(0, (now - Number(state.startedAt || now)) / 1000);
    const runId = this.friendMatchmakingRunId;
    if (state.status === 'bot_ready' && now >= Number(state.botReadyAt || 0)) {
      this.startQueuedFriendMatch();
      return;
    }
    if (state.status === 'matched' && now >= Number(state.matchedAt || 0)) {
      this.startQueuedFriendMatch();
      return;
    }
    if (state.status !== 'searching') return;

    if (!this.friendMatchmakingLocal && !state.apiStarted && this.backendAuth && this.backendAuth.status === 'ready' && apiClient.joinMatchmaking) {
      state.apiStarted = true;
      this.friendMatchmakingRequestInFlight = true;
      apiClient.joinMatchmaking({ client_ticket: state.ticketId }).then((payload) => {
        if (runId !== this.friendMatchmakingRunId || !this.friendMatchmaking) return;
        if (payload && payload.ticket_id) {
          state.ticketId = String(payload.ticket_id);
          state.ticketReady = true;
        }
        this.applyFriendMatchmakingPayload(payload);
      }).catch((error) => {
        if (runId !== this.friendMatchmakingRunId || !this.friendMatchmaking) return;
        this.friendMatchmakingLocal = true;
        state.apiStarted = false;
        this.friendMatchmakingError = String(error && error.message || error || '\u7f51\u7edc\u5339\u914d\u5931\u8d25');
      }).then(() => {
        if (runId === this.friendMatchmakingRunId) this.friendMatchmakingRequestInFlight = false;
      });
    }

    if (!this.friendMatchmakingLocal && state.apiStarted && state.ticketReady && !this.friendMatchmakingRequestInFlight && apiClient.getMatchmakingStatus && now - this.friendMatchmakingLastPollAt >= friendMatch.MATCHMAKING_POLL_INTERVAL) {
      this.friendMatchmakingLastPollAt = now;
      apiClient.getMatchmakingStatus(state.ticketId).then((payload) => {
        if (runId === this.friendMatchmakingRunId) this.applyFriendMatchmakingPayload(payload);
      }).catch(() => { /* matchmaking polling is best effort; timeout fallback remains available */ });
    }

    if (elapsed >= friendMatch.MATCHMAKING_TIMEOUT) this.prepareBotFriendMatch();
  }

  updateFriendCountdown() {
    if (!this.friendCountdownActive) return;
    if (this.screen !== 'game') {
      this.friendCountdownActive = false;
      return;
    }
    const remaining = Number(this.friendCountdownUntil || 0) - Date.now();
    if (remaining > 0) return;
    this.friendCountdownActive = false;
    this.friendCountdownUntil = 0;
    this.friendCountdownLastNumber = 0;
    this.gamePaused = false;
    this.lastFrame = Date.now();
    this.triggerFeedback('info', '\u5bf9\u6218\u5f00\u59cb');
  }

  applyFriendMatchmakingPayload(payload) {
    if (!payload || !this.friendMatchmaking) return false;
    const source = payload.data && typeof payload.data === 'object' ? payload.data : payload;
    const status = String(source.status || source.state || '').toLowerCase();
    const match = source.match || source.result || source.room || null;
    if (!match || !['matched', 'ready', 'running', 'opponent_found'].includes(status) && !match.room_seed && !match.room_code) return false;
    const roomSource = source.room || (match.room && typeof match.room === 'object' ? match.room : match);
    const fallback = friendMatch.createLocalRoom(roomSource && roomSource.room_code || '');
    const room = friendMatch.normalizeRoom(roomSource, fallback);
    const opponent = source.opponent || match.opponent;
    if (opponent && (!Array.isArray(room.players) || room.players.length < 2)) {
      room.players = Array.isArray(room.players) ? room.players : [];
      room.players.push({ id: String(opponent.id || opponent.user_id || 'opponent'), name: String(opponent.name || opponent.nickname || '\u5bf9\u624b'), ready: true });
    }
    room.status = 'ready';
    room.local_fallback = false;
    this.friendRoom = room;
    this.friendRules = Object.assign({}, friendMatch.rules(), room.rules || {});
    this.friendLocalFallback = false;
    this.friendRoomBackendStatus = 'ready';
    this.friendRoomFromInvite = false;
    this.friendLobbyView = 'room';
    this.friendSelfReady = true;
    this.friendMatchmaking.status = 'matched';
    this.friendMatchmaking.matchedAt = Date.now() + 900;
    return true;
  }

  prepareBotFriendMatch() {
    if (!this.friendMatchmaking || this.friendMatchmaking.status !== 'searching') return;
    const seed = (Date.now() ^ Math.floor(Math.random() * 0x100000)) >>> 0 || 1;
    const code = String(100000 + (seed % 900000)).padStart(6, '0');
    const room = friendMatch.createLocalRoom(code, '\u6211', '\u4eba\u673a');
    const difficulty = friendMatch.randomBotDifficulty(room.room_seed);
    const profile = friendMatch.botProfile(difficulty);
    room.players[1] = { id: profile.id, name: profile.name, ready: true, bot: true, difficulty };
    room.status = 'ready';
    room.local_fallback = true;
    this.friendRoom = room;
    this.friendRules = friendMatch.rules();
    this.friendLocalFallback = true;
    this.friendRoomBackendStatus = 'local';
    this.friendRoomFromInvite = false;
    this.friendLobbyView = 'room';
    this.friendSelfReady = true;
    this.friendBotDifficulty = difficulty;
    this.friendBotName = profile.name;
    this.friendMatchmaking.status = 'bot_ready';
    this.friendMatchmaking.botReadyAt = Date.now() + 750;
    this.friendMatchmakingRunId += 1;
    if (this.friendMatchmakingRequestInFlight && apiClient.cancelMatchmaking && this.friendMatchmaking.ticketId) {
      apiClient.cancelMatchmaking(this.friendMatchmaking.ticketId).catch(() => {});
    }
  }

  startQueuedFriendMatch() {
    if (!this.friendMatchmaking || !this.friendRoom) return;
    this.friendLobbyView = 'room';
    this.friendRoomFromInvite = false;
    this.startFriend();
  }

  cancelFriendMatchmaking() {
    const state = this.friendMatchmaking;
    if (state && state.status === 'searching' && state.ticketId && apiClient.cancelMatchmaking && this.backendAuth && this.backendAuth.status === 'ready') {
      apiClient.cancelMatchmaking(state.ticketId).catch(() => {});
    }
    this.friendMatchmakingRunId += 1;
    this.friendMatchmaking = null;
    this.friendMatchmakingRequestInFlight = false;
    this.screen = 'friend_lobby';
    this.friendLobbyView = 'entry';
    this.friendRoom = null;
    this.triggerFeedback('info', '\u5df2\u53d6\u6d88\u5339\u914d');
  }

  saveAudioSettings() {
    this.progress.audio = this.audio.settings();
    storage.save(this.progress);
    if (this.backendAuth && this.backendAuth.status === 'ready' && apiClient.updatePreferences) {
      apiClient.updatePreferences(this.progress.audio).catch((error) => {
        try { if (typeof console !== 'undefined' && console.warn) console.warn('[game-backend-preferences]', error); } catch (logError) { /* preferences remain local */ }
      });
    }
  }

  applyServerProgressMutation(result) {
    if (!result) return;
    let remote = result.progress;
    if (typeof remote === 'string') {
      try { remote = JSON.parse(remote); } catch (error) { remote = null; }
    }
    if (remote && typeof remote === 'object') {
      if (Array.isArray(remote.owned_skins)) this.progress.owned_skins = remote.owned_skins.slice();
      if (remote.equipped_skin) this.progress.equipped_skin = String(remote.equipped_skin);
      if (Array.isArray(remote.owned_cosmetics)) this.progress.owned_cosmetics = remote.owned_cosmetics.slice();
      if (remote.equipped_cosmetics && typeof remote.equipped_cosmetics === 'object') this.progress.equipped_cosmetics = { ...remote.equipped_cosmetics };
      if (remote.audio && typeof remote.audio === 'object') this.progress.audio = { ...this.progress.audio, ...remote.audio };
    }
    if (result.coins !== undefined) this.progress.coins = Math.max(0, Math.floor(Number(result.coins) || 0));
    storage.save(this.progress);
    if (this.audio && this.progress.audio) this.audio.applySettings(this.progress.audio);
  }

  restoreStorageBackup() {
    const restored = storage.restoreBackup ? storage.restoreBackup() : null;
    if (!restored) {
      this.status = '没有可恢复的备用存档';
      this.triggerFeedback('info', this.status);
      return;
    }
    this.progress = restored;
    this.storageLoadInfo = storage.getLastLoadInfo ? storage.getLastLoadInfo() : {};
    if (this.audio && this.audio.applySettings) this.audio.applySettings(this.progress.audio || {});
    if (this.ads && this.ads.configure) this.ads.configure(this.progress.ads || {}, storage.todayKey());
    this.menuPage = clamp(Math.floor(safeNumber(this.progress.unlocked_level, 0) / 20), 0, 9);
    this.popup = '';
    this.triggerFeedback('success', '已恢复上一次正常存档');
  }

  diagnosticReport() {
    return diagnostics.report({
      storage,
      audio: this.audio,
      questionService: this.questionService,
      campaignStats: this.campaignPuzzleBankStats,
      backendAuth: this.backendAuth,
      backendConfigured: apiClient.isConfigured && apiClient.isConfigured(),
      viewportWidth: this.viewportWidth,
      viewportHeight: this.viewportHeight,
      dpr: this.dpr,
      lastRuntimeError: this.lastRuntimeError,
    });
  }

  drawDiagnosticsPopup() {
    const width = 620;
    const height = 700;
    const x = (this.width - width) / 2;
    const y = this.modalTop(height);
    const report = this.diagnosticReport();
    const storageState = report.storage;
    const lastError = report.runtime.lastError || storageState.lastError;
    this.drawModalFrame(x, y, width, height, '存档与设备诊断', '仅用于真机测试，不影响正常玩法', 'cyan');
    this.drawGamePanel(x + 24, y + 126, width - 48, 350, 'dark', {
      radius: 22,
      shadow: false,
      stroke: 'rgba(80,227,255,0.24)',
    });
    const rows = [
      ['设备', `${report.device.platform}  ${report.device.screen}  DPR ${report.device.pixelRatio}`],
      ['系统', `${report.device.brand} ${report.device.model}  ${report.device.system}`],
      ['存档', `主档 ${storageState.primaryValid ? '正常' : '缺失'} · 备用 ${storageState.backupValid ? '可用' : '无'} · v${storageState.primaryVersion || 0}`],
      ['题目', `闯关 ${report.questions.campaignVerified}/${report.questions.campaignTotal} 已验证 · 生成器 ${report.questions.generatorReady ? '正常' : '异常'}`],
      ['音频', `背景${report.audio.music ? '开' : '关'} · 音效${report.audio.sfx ? '开' : '关'} · 资源${report.audio.failed ? '异常' : '正常'}`],
      ['联网', `${report.backend.status} · ${report.backend.configured ? '已配置后端' : '本地体验模式'}`],
      ['日志', `${report.runtime.errorCount} 条运行日志`],
    ];
    rows.forEach(([label, value], index) => {
      const rowY = y + 158 + index * 42;
      this.drawText(label, x + 48, rowY, uiFont(15, 900), GAME_UI.cyanLight, 'left');
      this.drawFitText(value, x + 142, rowY, width - 190, uiFont(14, 500), GAME_UI.text, 'left');
    });
    if (lastError) {
      const message = String(lastError.message || lastError).replace(/\s+/g, ' ').slice(0, 72);
      this.drawFitText(`最近错误：${message}`, this.width / 2, y + 508, width - 80, uiFont(13, 500), GAME_UI.danger, 'left');
    } else {
      this.drawFitText('当前没有记录到运行错误', this.width / 2, y + 508, width - 80, uiFont(13, 500), GAME_UI.success);
    }
    this.drawNeonButton(x + 24, y + 570, 276, 54, '恢复上次存档', () => this.restoreStorageBackup(), 'violet', {
      fontSize: 16,
      radius: 18,
      disabled: !storageState.backupValid,
      key: 'diagnostics-restore',
    });
    this.drawNeonButton(x + 320, y + 570, 276, 54, '清除错误日志', () => {
      if (storage.clearErrorLogs) storage.clearErrorLogs();
      this.triggerFeedback('success', '错误日志已清除');
    }, 'magenta', { fontSize: 16, radius: 18, key: 'diagnostics-clear-logs' });
  }

  finishTutorial() {
    this.progress.tutorial_seen = true;
    storage.save(this.progress);
    this.popup = '';
    this.tutorialStep = 0;
  }

  nextTutorial() {
    if (this.tutorialStep >= 2) this.finishTutorial();
    else this.tutorialStep += 1;
  }

  drawPopup() {
    // 模态层先注册全屏拦截，避免弹窗打开时误触到底层首页按钮。
    this.addHitArea(0, 0, this.width, this.height, () => {}, { key: 'popup-overlay' });
    this.ctx.fillStyle = GAME_UI.overlay;
    this.ctx.fillRect(0, 0, this.width, this.height);
    let x = 0;
    let y = 0;
    let width = 0;
    let height = 0;
    if (this.popup === 'chapter_info') {
      width = 590; height = 520;
      x = (this.width - width) / 2;
      y = this.modalTop(height);
      const chapterIndex = clamp(this.menuPage, 0, levelCatalog.CHAPTERS.length - 1);
      const chapter = levelCatalog.CHAPTERS[chapterIndex] || levelCatalog.CHAPTERS[0];
      const chapterStart = chapterIndex * 20;
      const unlocked = clamp(safeNumber(this.progress.unlocked_level) - chapterStart, 0, 20);
      const completed = Object.keys(this.progress.levels || {}).filter((key) => Number(key) >= chapterStart && Number(key) < chapterStart + 20).length;
      this.drawModalFrame(x, y, width, height, `${chapterIndex + 1}. ${chapter[0]}`, '章节详情 · 点击右上角关闭', 'cyan');
      this.drawGamePanel(x + 30, y + 132, width - 60, 112, 'violet', { radius: 22, shadow: false });
      this.drawFitText(chapter[1], this.width / 2, y + 166, width - 100, uiFont(18, 800), GAME_UI.text);
      this.drawFitText(`章节目标：${chapter[4] || '完成本章全部关卡'}`, this.width / 2, y + 207, width - 100, uiFont(15, 500), GAME_UI.secondary);
      this.drawGamePanel(x + 30, y + 268, width - 60, 92, 'gold', { radius: 22, shadow: false });
      this.drawFitText(`已解锁 ${unlocked} / 20    已完成 ${completed} / 20`, this.width / 2, y + 302, width - 100, uiFont(20, 900), GAME_UI.goldLight);
      this.drawFitText('本章题目均由程序生成并验证，逐步提高难度。', this.width / 2, y + 335, width - 100, uiFont(14, 500), GAME_UI.secondary);
      this.drawNeonButton(x + 54, y + 392, width - 108, 58, unlocked > 0 ? `开始第 ${chapterStart + 1} 关` : '等待解锁', () => {
        this.popup = '';
        if (unlocked > 0) this.startCampaign(chapterStart);
      }, 'cyan', { fontSize: 18, radius: 18, disabled: unlocked <= 0, key: 'chapter-start' });
      return;
    }
    if (this.popup === 'tutorial') {
      width = 572; height = 560;
      x = (this.width - width) / 2;
      y = this.modalTop(height);
      this.drawModalFrame(x, y, width, height, '欢迎来到三火算术练习', `新手引导 ${this.tutorialStep + 1} / 3`, 'cyan');
      const steps = [
        ['先点一个数字', '选择你想先参与计算的数字卡片。', '数字会出现青色高光。'],
        ['再点运算符', '选择 +、−、× 或 ÷。', '除法只允许整数结果。'],
        ['最后点第二个数字', '完成一次合成，继续计算直到得到 24。', '每个原始数字只能使用一次。'],
      ];
      const step = steps[this.tutorialStep] || steps[0];
      this.drawGamePanel(x + 48, y + 142, width - 96, 190, 'violet', { radius: 22, shadow: false });
      this.drawFitText(step[0], this.width / 2, y + 194, width - 126, uiFont(28, 900), GAME_UI.gold);
      this.drawFitText(step[1], this.width / 2, y + 244, width - 126, uiFont(17, 500), GAME_UI.text);
      this.drawFitText(step[2], this.width / 2, y + 284, width - 126, uiFont(15, 500), GAME_UI.secondary);
      this.drawGamePanel(x + 148, y + 360, 276, 64, 'cyan', { radius: 22, shadowColor: 'rgba(80,227,255,0.28)', shadowBlur: 12 });
      this.drawFitText(this.tutorialStep === 0 ? '数字卡片' : this.tutorialStep === 1 ? '+   −   ×   ÷' : '合成结果 → 24', this.width / 2, y + 392, 240, uiFont(24, 900), GAME_UI.cyan);
      this.drawNeonButton(x + 48, y + 454, 210, 56, '跳过引导', () => this.finishTutorial(), 'violet', { fontSize: 17, radius: 18, key: 'tutorial-skip' });
      this.drawNeonButton(x + 282, y + 454, 242, 56, this.tutorialStep >= 2 ? '开始练习' : '下一步', () => this.nextTutorial(), 'cyan', { fontSize: 18, radius: 18, key: 'tutorial-next' });
      return;
    }
    if (this.popup === 'settings') {
      width = 446;
      height = 650;
      x = (this.width - width) / 2;
      y = this.modalTop(height);
      this.drawModalFrame(x, y, width, height, '设置', '音效设置会自动保存。', 'cyan');
      const audio = this.audio.settings();
      const musicOn = audio.music_enabled !== false;
      const sfxOn = audio.sfx_enabled !== false;
      this.drawSettingsToggleRow(x + 24, y + 128, width - 48, 84, '背景音乐', `当前曲目 · ${this.audio.getMusicTrackName()}`, musicOn, () => { this.audio.setMusicEnabled(!musicOn); this.saveAudioSettings(); }, 'settings-music', 'cyan');
      this.drawSettingVolumeBlock(x + 24, y + 226, width - 48, '背景音乐音量', safeNumber(audio.music_volume, 0.42), 'cyan');
      this.drawNeonButton(x + 24, y + 318, width - 48, 54, `更换音乐 · ${this.audio.getMusicTrackName()}`, () => { this.audio.setMusicTrack(audio.music_track + 1); this.saveAudioSettings(); }, 'violet', { fontSize: 16, radius: 18, key: 'settings-track', shadowBlur: 8 });
      this.drawSettingsToggleRow(x + 24, y + 392, width - 48, 84, '按键音效', '点击、合成、倒计时反馈', sfxOn, () => { this.audio.setSfxEnabled(!sfxOn); this.saveAudioSettings(); }, 'settings-sfx', 'magenta');
      this.drawSettingVolumeBlock(x + 24, y + 490, width - 48, '按键音效音量', safeNumber(audio.sfx_volume, 0.72), 'magenta');
      this.drawNeonButton(x + 24, y + 580, width - 48, 52, '存档与设备诊断', () => { this.popup = 'diagnostics'; this.diagnosticsEnabled = true; }, 'cyan', { fontSize: 16, radius: 18, key: 'settings-diagnostics' });
      return;
    }
    if (this.popup === 'diagnostics') {
      this.drawDiagnosticsPopup();
      return;
    }
    if (this.popup === 'tasks') {
      width = 580; height = 700;
      x = (this.width - width) / 2;
      y = this.modalTop(height);
      const login = this.progress.login || {};
      const loginHint = `每日零点刷新 · 连续登录 ${safeNumber(login.streak)} 天${this.loginReward > 0 ? ` · 今日 +${this.loginReward} 金币已到账` : ''}`;
      this.drawModalFrame(x, y, width, height, '每日任务', loginHint, 'violet');
      const dailyTasks = taskService.snapshot(this.progress, storage.todayKey());
      Object.entries(dailyTasks).forEach(([taskId, task], index) => {
        const claimed = Boolean(task.claimed);
        const rowY = y + 130 + index * 110;
        this.drawProgressTaskCard(x + 24, rowY, width - 48, 88, task, claimed);
      });
      const weeklyTasks = Object.values(taskService.weeklySnapshot(this.progress, storage.todayKey()));
      this.drawText('本周任务 · 每周一刷新', x + 24, y + 484, uiFont(17, 900), GAME_UI.cyanLight, 'left');
      weeklyTasks.forEach((task, index) => {
        const col = index % 2;
        const row = Math.floor(index / 2);
        const itemX = x + 24 + col * 266;
        const itemY = y + 510 + row * 42;
        const value = task.claimed ? '已完成' : `${task.value}/${task.target}`;
        this.drawFitText(`${task.title.replace('本周', '')}`, itemX, itemY, 230, uiFont(13, 500), GAME_UI.text, 'left');
        this.drawFitText(`${value} · +${task.reward}`, itemX, itemY + 20, 230, uiFont(12, 700), task.claimed ? GAME_UI.success : GAME_UI.gold, 'left');
      });
      this.drawNeonButton(x + 24, y + 630, width - 48, 52, '知道了', () => { this.popup = ''; }, 'violet', { fontSize: 18, radius: 18, key: 'tasks-ok' });
      return;
    }

    width = 520; height = 468;
    x = (this.width - width) / 2;
    y = this.modalTop(height);
    this.drawModalFrame(x, y, width, height, '更多功能', '选择一个功能继续。', 'magenta');
    const cardW = 224;
    const cardH = 116;
    const leftX = x + 28;
    const rightX = x + width - 28 - cardW;
    const topY = y + 136;
    const bottomY = y + 272;
    this.drawFeatureMenuCard('more-shop', leftX, topY, cardW, cardH, '主题商城', '皮肤 · 外观', 'cyan', (cx, cy, accent) => {
      this.drawThemePreview(cx - 20, cy - 20, 40, skinCatalog.getSkin(this.progress.equipped_skin || 'classic'), false);
      this.drawText('24', cx, cy + 4, uiFont(16, 900), GAME_UI.text);
    }, () => { this.popup = ''; this.shopNotice = ''; this.screen = 'shop'; });
    this.drawFeatureMenuCard('more-achievements', rightX, topY, cardW, cardH, '成就徽章', '奖励 · 收集', 'violet', (cx, cy) => {
      this.drawAchievementBadge(cx, cy, false, 0);
    }, () => { this.popup = ''; this.screen = 'achievements'; });
    this.drawFeatureMenuCard('more-leaderboard', leftX, bottomY, cardW, cardH, '排行榜', '总榜 · 好友', 'magenta', (cx, cy, accent) => {
      this.drawBarsIcon(cx, cy + 4, 0.38);
      this.ctx.save();
      this.ctx.globalAlpha = 0.6;
      this.drawText('1', cx - 18, cy - 20, uiFont(13, 900), accent);
      this.ctx.restore();
    }, () => { this.popup = ''; this.screen = 'leaderboard'; });
    this.drawFeatureMenuCard('more-records', rightX, bottomY, cardW, cardH, '挑战记录', '数据 · 统计', 'cyan', (cx, cy, accent) => {
      this.drawTargetIcon(cx, cy, 0.45, accent);
    }, () => { this.popup = ''; this.screen = 'records'; });
    this.drawFitText('所有数据保存在本机，后续可接入微信云同步。', this.width / 2, y + height - 32, width - 80, uiFont(12, 500), GAME_UI.muted);
  }

  shopItemsForTab() {
    if (!this.shopTab) this.shopTab = 'themes';
    if (this.shopTab === 'themes') return skinCatalog.all();
    const category = { cards: 'card', operators: 'operator', effects: 'result' }[this.shopTab] || 'card';
    return skinCatalog.allCosmetics(category);
  }

  drawCosmeticPreview(x, y, size, item, active = false) {
    const category = item && item.category || 'card';
    const preview = item && item.preview || 'classic';
    const variant = preview === 'candy' || preview === 'bubble' ? 'magenta' : preview === 'prism' ? 'cyan' : preview === 'fireworks' ? 'gold' : 'violet';
    const accent = active ? GAME_UI.gold : this.accentColor(variant);
    this.drawGamePanel(x, y, size, size, variant, {
      radius: 22,
      stroke: accent,
      shadowColor: `${accent}66`,
      shadowBlur: active ? 18 : 12,
      shadowOffsetY: 0,
    });
    if (category === 'card') {
      const fill = preview === 'neon' ? 'rgba(117,226,255,0.92)' : preview === 'candy' ? 'rgba(255,174,228,0.92)' : 'rgba(255,255,255,0.92)';
      this.drawGlassCard(x + 21, y + 16, size - 42, size - 32, 15, fill, accent, { shadowColor: `${accent}55`, shadowBlur: 8, shadowOffsetY: 0 });
      this.drawText('24', x + size / 2, y + size / 2 + 2, uiFont(size * 0.34, 900), '#17163e');
    } else if (category === 'operator') {
      ['+', '−', '×', '÷'].forEach((symbol, index) => {
        const col = index % 2;
        const row = Math.floor(index / 2);
        this.drawGlassCard(x + 18 + col * (size - 54) / 1.65, y + 18 + row * 42, 38, 30, 11, 'rgba(255,255,255,0.12)', accent, { shadow: false });
        this.drawText(symbol, x + 37 + col * (size - 54) / 1.65, y + 34 + row * 42, uiFont(18, 900), GAME_UI.text);
      });
    } else {
      this.drawStarIcon(x + size / 2, y + size / 2, 0.62, accent);
      if (preview !== 'classic') {
        for (let index = 0; index < 5; index += 1) {
          const angle = (Math.PI * 2 * index) / 5;
          this.drawSparkle(x + size / 2 + Math.cos(angle) * 34, y + size / 2 + Math.sin(angle) * 24, 3.5, [GAME_UI.cyan, GAME_UI.magenta, GAME_UI.gold][index % 3], 0.82);
        }
      }
    }
  }

  drawShop() {
    this.buttons = [];
    if (!this.shopTab) this.shopTab = 'themes';
    this.drawGameHeader('主题商城', '‹ 返回', () => this.goHome());
    const introY = this.screenContentTop(92);
    const items = this.shopItemsForTab();
    const pageSize = 2;
    const pageCount = Math.max(1, Math.ceil(items.length / pageSize));
    this.shopPage = clamp(safeNumber(this.shopPage, 0), 0, pageCount - 1);
    const pageItems = items.slice(this.shopPage * pageSize, this.shopPage * pageSize + pageSize);
    const tabLabels = [['themes', '主题'], ['cards', '卡片'], ['operators', '运算符'], ['effects', '特效']];
    this.drawGamePanel(32, introY, this.width - 64, 112, 'cyan', { radius: 26, shadowColor: 'rgba(80,227,255,0.24)', shadowBlur: 12, shadowOffsetY: 0 });
    this.drawFitText(this.shopTab === 'themes' ? '用金币换个主题，换个心情继续练习' : '收集小外观，让每次计算都有新鲜感', this.width / 2, introY + 34, this.width - 120, uiFont(20, 900), GAME_UI.text);
    this.drawFitText('外观只改变表现，不影响题目难度与分数', this.width / 2, introY + 62, this.width - 120, uiFont(12, 600), GAME_UI.secondary);
    // 金币图标与文字使用固定的左右排列，避免居中测量时图标盖住“金币余额”的首字。
    this.drawCoinIcon(this.width / 2 - 68, introY + 90, 17);
    this.drawFitText(`金币余额 · ${safeNumber(this.progress.coins)}`, this.width / 2 - 42, introY + 90, 220, uiFont(18, 800), GAME_UI.gold, 'left');
    const tabY = introY + 124;
    const tabWidth = (this.width - 64 - 36) / 4;
    tabLabels.forEach(([tab, label], index) => {
      this.drawNeonButton(32 + index * (tabWidth + 12), tabY, tabWidth, 46, label, () => { this.shopTab = tab; this.shopPage = 0; this.shopNotice = ''; }, this.shopTab === tab ? 'cyan' : 'violet', { fontSize: 15, radius: 16, key: `shop-tab-${tab}` });
    });
    const category = this.shopTab === 'themes' ? '' : { cards: 'card', operators: 'operator', effects: 'result' }[this.shopTab];
    const ownedThemes = Array.isArray(this.progress.owned_skins) ? this.progress.owned_skins : ['classic'];
    const ownedCosmetics = Array.isArray(this.progress.owned_cosmetics) ? this.progress.owned_cosmetics : ['card_classic', 'operator_classic', 'result_classic'];
    pageItems.forEach((item, index) => {
      const y = introY + 184 + index * 252;
      const isTheme = this.shopTab === 'themes';
      const isOwned = (isTheme ? ownedThemes : ownedCosmetics).includes(item.id);
      const equipped = isTheme ? String(this.progress.equipped_skin || 'classic') : String((this.progress.equipped_cosmetics || {})[category] || '');
      const isEquipped = equipped === item.id;
      const isPreviewing = isTheme && this.activeSkinId() === item.id;
      const requirement = isTheme ? skinCatalog.unlockStatus(item.id, this.progress) : skinCatalog.cosmeticUnlockStatus(item.id, this.progress);
      const currentCoins = safeNumber(this.progress.coins);
      const price = safeNumber(item.price);
      const enoughCoins = currentCoins >= price;
      const canBuy = requirement.unlocked && enoughCoins;
      const shopBusy = Boolean(this.shopActionInFlight);
      const variant = isEquipped || isPreviewing ? 'gold' : canBuy || isOwned ? 'cyan' : 'violet';
      this.drawGamePanel(32, y, this.width - 64, 224, variant, { radius: 26, lineWidth: isEquipped || isPreviewing ? 3 : 1.6, shadowColor: isEquipped || isPreviewing ? 'rgba(255,211,77,0.26)' : 'rgba(40,233,255,0.18)', shadowBlur: isEquipped || isPreviewing ? 18 : 12, shadowOffsetY: 0 });
      if (isTheme) this.drawThemePreview(58, y + 28, 132, item, isPreviewing);
      else this.drawCosmeticPreview(58, y + 28, 132, item, isEquipped);
      this.drawFitText(item.name, 224, y + 43, 410, uiFont(23, 900), GAME_UI.text, 'left');
      this.drawFitText(item.description, 224, y + 79, 410, uiFont(14, 500), GAME_UI.secondary, 'left');
      const stateText = isEquipped ? '已装备' : isPreviewing ? '试用中 · 主题会自动恢复' : isOwned ? '已拥有 · 可装备' : !requirement.unlocked ? `暂未解锁 · ${requirement.reason}` : !enoughCoins ? `金币不足 · 还差 ${price - currentCoins} 金币` : `可兑换 · ${price} 金币`;
      const stateColor = isEquipped ? GAME_UI.success : !isOwned && !requirement.unlocked ? GAME_UI.muted : canBuy || isOwned ? GAME_UI.cyan : GAME_UI.magentaLight;
      this.drawFitText(stateText, 224, y + 115, 410, uiFont(15, 800), stateColor, 'left');
      const trialEnabled = isTheme && !isEquipped && !shopBusy;
      const trialLabel = isTheme ? (trialEnabled ? '试用 10 秒' : '当前主题') : '暂无试用';
      this.drawNeonButton(224, y + 154, 194, 50, trialLabel, () => this.startSkinPreview(item), 'magenta', { fontSize: 16, radius: 17, disabled: !trialEnabled, key: `trial-${item.id}` });
      const actionLabel = shopBusy ? '处理中…' : isEquipped ? '已装备' : isOwned ? (isTheme ? '装备主题' : '装备') : !requirement.unlocked ? '暂未解锁' : !enoughCoins ? '金币不足' : `兑换 · ${price}`;
      this.drawNeonButton(430, y + 154, 210, 50, actionLabel, () => this.selectShopItem(item), isEquipped ? 'gold' : canBuy || isOwned ? 'violet' : 'violet', { fontSize: 16, radius: 17, disabled: shopBusy || isEquipped || (!isOwned && !canBuy), key: `shop-item-${item.id}` });
    });
    const navY = Math.min(introY + 184 + pageItems.length * 252 + 10, this.visibleBottom(88) - 50);
    if (pageCount > 1) {
      this.drawNeonButton(32, navY, 132, 50, '‹ 上一页', () => { this.shopPage = Math.max(0, this.shopPage - 1); }, 'violet', { fontSize: 15, radius: 16, disabled: this.shopPage === 0, key: 'shop-prev' });
      this.drawText(`第 ${this.shopPage + 1} / ${pageCount} 页`, this.width / 2, navY + 25, uiFont(15, 500), GAME_UI.secondary);
      this.drawNeonButton(this.width - 164, navY, 132, 50, '下一页 ›', () => { this.shopPage = Math.min(pageCount - 1, this.shopPage + 1); }, 'cyan', { fontSize: 15, radius: 16, disabled: this.shopPage === pageCount - 1, key: 'shop-next' });
    }
    if (this.shopNotice) this.drawFitText(this.shopNotice, this.width / 2, navY + 80, this.width - 86, uiFont(16, 900), GAME_UI.gold);
    this.drawFitText(`主题试用 10 秒 · 当前分类：${this.shopTab === 'themes' ? '主题' : skinCatalog.categoryLabel(category)}`, this.width / 2, Math.min(navY + 130, this.visibleBottom(20)), this.width - 86, uiFont(14, 500), GAME_UI.secondary);
  }

  drawShopCosmeticState(item) {
    const owned = Array.isArray(this.progress.owned_cosmetics) ? this.progress.owned_cosmetics : [];
    return owned.includes(item.id);
  }

  selectShopItem(item) {
    if (!item) return;
    if (this.shopActionInFlight) {
      this.shopNotice = '上一笔商城操作正在处理中，请稍候';
      return;
    }
    if (this.shopTab === 'themes') {
      this.selectShopSkin(item);
      return;
    }
    const owned = Array.isArray(this.progress.owned_cosmetics) ? this.progress.owned_cosmetics : ['card_classic', 'operator_classic', 'result_classic'];
    if (owned.includes(item.id)) {
      if (this.backendAuth && this.backendAuth.status === 'ready' && apiClient.equipCosmetic) {
        this.shopActionInFlight = item.id;
        this.shopNotice = '正在保存装备…';
        Promise.resolve().then(() => apiClient.equipCosmetic(item.id)).then((result) => {
          this.applyServerProgressMutation(result);
          this.shopNotice = `已装备「${item.name}」`;
          this.triggerFeedback('success', `已装备${skinCatalog.categoryLabel(item.category)}`);
        }).catch((error) => {
          this.shopNotice = String(error && error.message || '装备失败');
        }).then(() => { if (this.shopActionInFlight === item.id) this.shopActionInFlight = ''; });
        return;
      }
      this.progress.equipped_cosmetics = this.progress.equipped_cosmetics || {};
      this.progress.equipped_cosmetics[item.category] = item.id;
      storage.save(this.progress);
      this.shopNotice = `已装备「${item.name}」`;
      this.triggerFeedback('success', `已装备${skinCatalog.categoryLabel(item.category)}`);
      return;
    }
    const requirement = skinCatalog.cosmeticUnlockStatus(item.id, this.progress);
    const price = Math.max(0, safeNumber(item.price));
    if (!requirement.unlocked) { this.shopNotice = requirement.reason; return; }
    if (safeNumber(this.progress.coins) < price) {
      this.shopNotice = `金币不足，还需要 ${price - safeNumber(this.progress.coins)} 金币`;
      return;
    }
    if (this.backendAuth && this.backendAuth.status === 'ready' && apiClient.purchaseCosmetic) {
      // 后端购买不做乐观扣币，只有服务端确认后才改变本地存档；失败时不会留下半笔交易。
      this.shopActionInFlight = item.id;
      this.shopNotice = '正在向服务端确认兑换…';
      Promise.resolve().then(() => apiClient.purchaseCosmetic(item.id)).then((result) => {
        const remoteCoins = result && (result.coins !== undefined || result.progress && result.progress.coins !== undefined);
        if (!remoteCoins && !storage.spendCoins(this.progress, price)) throw new Error('金币余额已变化，请刷新后重试');
        this.applyServerProgressMutation(result);
        this.shopNotice = `兑换成功，已装备「${item.name}」`;
        this.triggerFeedback('success', `获得${skinCatalog.categoryLabel(item.category)}`);
      }).catch((error) => {
        this.shopNotice = String(error && error.message || '兑换失败');
      }).then(() => { if (this.shopActionInFlight === item.id) this.shopActionInFlight = ''; });
      return;
    }
    if (!storage.spendCoins(this.progress, price)) {
      this.shopNotice = `金币不足，还需要 ${price - safeNumber(this.progress.coins)} 金币`;
      return;
    }
    // 本地模式在确认余额后才扣币并写入拥有列表。
    this.progress.owned_cosmetics = owned.concat([item.id]);
    this.progress.equipped_cosmetics = this.progress.equipped_cosmetics || {};
    this.progress.equipped_cosmetics[item.category] = item.id;
    storage.save(this.progress);
    this.shopNotice = `兑换成功，已装备「${item.name}」`;
    this.triggerFeedback('success', `获得${skinCatalog.categoryLabel(item.category)}`);
  }

  selectShopSkin(skin) {
    if (!skin || !skin.id || this.shopActionInFlight) return;
    const owned = Array.isArray(this.progress.owned_skins) ? this.progress.owned_skins : ['classic'];
    if (owned.includes(skin.id)) {
      if (this.backendAuth && this.backendAuth.status === 'ready' && apiClient.equipSkin) {
        this.shopActionInFlight = skin.id;
        this.shopNotice = '正在保存装备…';
        Promise.resolve().then(() => apiClient.equipSkin(skin.id)).then((result) => {
          this.applyServerProgressMutation(result);
          this.clearSkinPreview(false);
          this.shopNotice = `已装备「${skin.name}」`;
          this.triggerFeedback('success', `已装备主题「${skin.name}」`);
        }).catch((error) => {
          this.shopNotice = String(error && error.message || '装备失败');
        }).then(() => { if (this.shopActionInFlight === skin.id) this.shopActionInFlight = ''; });
        return;
      }
      this.clearSkinPreview(false);
      this.progress.equipped_skin = skin.id;
      storage.save(this.progress);
      this.shopNotice = `已装备「${skin.name}」`;
      this.triggerFeedback('success', `已装备主题「${skin.name}」`);
      return;
    }
    const requirement = skinCatalog.unlockStatus(skin.id, this.progress);
    if (!requirement.unlocked) { this.shopNotice = requirement.reason; return; }
    const price = Math.max(0, safeNumber(skin.price));
    if (safeNumber(this.progress.coins) < price) { this.shopNotice = `金币不足，还需要 ${price - safeNumber(this.progress.coins)} 金币`; return; }
    if (this.backendAuth && this.backendAuth.status === 'ready' && apiClient.purchaseSkin) {
      this.shopActionInFlight = skin.id;
      this.shopNotice = '正在向服务端确认兑换…';
      Promise.resolve().then(() => apiClient.purchaseSkin(skin.id)).then((result) => {
        const remoteCoins = result && (result.coins !== undefined || result.progress && result.progress.coins !== undefined);
        if (!remoteCoins && !storage.spendCoins(this.progress, price)) throw new Error('金币余额已变化，请刷新后重试');
        this.applyServerProgressMutation(result);
        this.clearSkinPreview(false);
        this.shopNotice = `兑换成功，已装备「${skin.name}」`;
        this.triggerFeedback('success', `获得主题「${skin.name}」`);
      }).catch((error) => {
        this.shopNotice = String(error && error.message || '兑换失败');
      }).then(() => { if (this.shopActionInFlight === skin.id) this.shopActionInFlight = ''; });
      return;
    }
    if (!storage.spendCoins(this.progress, price)) {
      this.shopNotice = `金币不足，还需要 ${price - safeNumber(this.progress.coins)} 金币`;
      return;
    }
    this.progress.owned_skins = owned.concat([skin.id]);
    this.progress.equipped_skin = skin.id;
    this.clearSkinPreview(false);
    storage.save(this.progress);
    this.shopNotice = `兑换成功，已装备「${skin.name}」`;
    this.triggerFeedback('success', `获得主题「${skin.name}」`);
  }

  drawAchievements() {
    this.buttons = [];
    this.drawGameHeader('成就徽章', '‹ 返回', () => this.goHome());
    const top = this.pageTop();
    const summaryY = this.screenContentTop(92);
    const achievements = achievementService.all();
    const count = achievementService.unlockedCount(this.progress);
    const pageSize = 8;
    const pageCount = Math.max(1, Math.ceil(achievements.length / pageSize));
    this.achievementPage = clamp(safeNumber(this.achievementPage, 0), 0, pageCount - 1);
    const pageItems = achievements.slice(this.achievementPage * pageSize, this.achievementPage * pageSize + pageSize);
    this.drawGamePanel(32, summaryY, this.width - 64, 108, 'violet', {
      radius: 24,
      shadowColor: 'rgba(191,156,255,0.24)',
      shadowBlur: 12,
      shadowOffsetY: 0,
    });
    this.drawFitText(`已解锁 ${count} / ${achievements.length}`, this.width / 2, summaryY + 38, this.width - 120, uiFont(26, 900), GAME_UI.violetLight);
    this.drawFitText('完成挑战、连击和主题收集，获得额外金币', this.width / 2, summaryY + 76, this.width - 120, uiFont(15, 500), GAME_UI.secondary);
    pageItems.forEach((item, index) => {
      const col = index % 2;
      const row = Math.floor(index / 2);
      const x = 32 + col * 344;
      const y = summaryY + 138 + row * 132;
      const unlocked = achievementService.isUnlocked(this.progress, item.id);
      this.drawGamePanel(x, y, 326, 112, unlocked ? 'gold' : 'violet', {
        radius: 20,
        fill: unlocked ? this.glassFill(x, y, 326, 112, 'gold') : 'rgba(35,30,92,0.54)',
        stroke: unlocked ? 'rgba(255,211,77,0.72)' : 'rgba(170,135,255,0.38)',
        shadow: false,
      });
      this.drawAchievementBadge(x + 48, y + 52, unlocked, index);
      this.drawFitText(item.title, x + 92, y + 31, 210, uiFont(16, 900), GAME_UI.text, 'left');
      this.drawFitText(item.description, x + 92, y + 59, 210, uiFont(12, 500), GAME_UI.secondary, 'left');
      this.drawFitText(unlocked ? '已获得' : `奖励 +${item.reward}`, x + 92, y + 88, 210, uiFont(13, 800), unlocked ? GAME_UI.success : GAME_UI.gold, 'left');
    });
    const navY = Math.min(summaryY + 682, this.visibleBottom(72) - 50);
    if (pageCount > 1) {
      this.drawNeonButton(32, navY, 132, 50, '‹ 上一页', () => { this.achievementPage = Math.max(0, this.achievementPage - 1); }, 'violet', { fontSize: 15, radius: 16, disabled: this.achievementPage === 0, key: 'achievement-prev' });
      this.drawFitText(`第 ${this.achievementPage + 1} / ${pageCount} 页`, this.width / 2, navY + 25, 220, uiFont(15, 500), GAME_UI.secondary);
      this.drawNeonButton(this.width - 164, navY, 132, 50, '下一页 ›', () => { this.achievementPage = Math.min(pageCount - 1, this.achievementPage + 1); }, 'cyan', { fontSize: 15, radius: 16, disabled: this.achievementPage === pageCount - 1, key: 'achievement-next' });
    } else {
      this.drawFitText('更多成就将在后续版本逐步开放', this.width / 2, navY + 25, this.width - 96, uiFont(14, 500), GAME_UI.secondary);
    }
  }

  drawLeaderboard() {
    this.buttons = [];
    const top = this.pageTop();
    this.drawGameHeader('排行榜', '‹ 返回', () => this.goHome());
    this.drawNeonButton(this.width - 138, top, 110, 52, '刷新', () => this.refreshLeaderboard(), 'cyan', { fontSize: 16, radius: 22, key: 'rank-refresh' });
    const boardY = this.screenContentTop(92);
    const modeY = boardY + 82;
    const listY = modeY + 76;
    this.drawNeonButton(32, boardY, 320, 54, '游戏总榜', () => { this.leaderboardBoard = leaderboardService.BOARD_GLOBAL; }, this.leaderboardBoard === leaderboardService.BOARD_GLOBAL ? 'cyan' : 'violet', { fontSize: 17, radius: 18, key: 'rank-global' });
    this.drawNeonButton(398, boardY, 320, 54, '微信好友榜', () => { this.leaderboardBoard = leaderboardService.BOARD_FRIENDS; }, this.leaderboardBoard === leaderboardService.BOARD_FRIENDS ? 'magenta' : 'violet', { fontSize: 17, radius: 18, key: 'rank-friends' });
    const modes = leaderboardService.modeIds();
    modes.forEach((mode, index) => {
      const x = 32 + index * 172;
      this.drawNeonButton(x, modeY, 156, 50, leaderboardService.modeName(mode), () => { this.leaderboardMode = mode; }, this.leaderboardMode === mode ? 'cyan' : 'violet', { fontSize: 14, radius: 16, key: `rank-mode-${mode}` });
    });
    const leaderboardScope = this.leaderboardBoard === leaderboardService.BOARD_FRIENDS ? 'friends' : 'global';
    const leaderboardKey = `${leaderboardScope}:${this.leaderboardMode}`;
    this.loadRemoteLeaderboard(this.leaderboardMode, leaderboardScope);
    const remoteLeaderboard = this.leaderboardRemote[leaderboardKey];
    const currentUserID = this.backendAuth && this.backendAuth.user ? Number(this.backendAuth.user.id || 0) : 0;
    const entries = leaderboardService.getEntries(this.progress, this.leaderboardBoard, this.leaderboardMode, remoteLeaderboard, currentUserID);
    const personal = leaderboardService.personalSummary(this.progress, this.leaderboardMode, remoteLeaderboard, currentUserID);
    this.drawGamePanel(32, listY, this.width - 64, 690, 'dark', { radius: 24, shadowBlur: 10 });
    this.drawFitText(`${leaderboardService.boardName(this.leaderboardBoard)} · ${leaderboardService.modeName(this.leaderboardMode)}`, this.width / 2, listY + 38, this.width - 120, uiFont(20, 900), GAME_UI.text);
    // 预留一行给“我的成绩”，避免真实榜单有数据时玩家看不到自己的本地记录。
    entries.slice(0, 7).forEach((entry, index) => {
      const y = listY + 78 + index * 70;
      const rankVariant = entry.is_player ? 'gold' : index === 0 ? 'gold' : index === 1 ? 'cyan' : index === 2 ? 'magenta' : 'dark';
      const accent = entry.is_player || index === 0 ? GAME_UI.gold : index === 1 ? GAME_UI.cyan : index === 2 ? GAME_UI.magentaLight : GAME_UI.secondary;
      this.drawGamePanel(56, y, this.width - 112, 56, rankVariant, {
        radius: 16,
        fill: entry.is_player ? this.glassFill(56, y, this.width - 112, 56, 'gold') : 'rgba(255,255,255,0.055)',
        stroke: entry.is_player ? 'rgba(255,211,77,0.70)' : 'rgba(255,255,255,0.10)',
        shadow: false,
      });
      this.drawText(String(entry.rank), 86, y + 29, uiFont(21, 900), accent);
      this.drawFitText(entry.name, 132, y + 21, 350, uiFont(16, 900), GAME_UI.text, 'left');
      this.drawFitText(entry.subtitle, 132, y + 41, 350, uiFont(12, 500), GAME_UI.secondary, 'left');
      this.drawFitText(String(entry.score), 646, y + 29, 120, uiFont(20, 900), entry.is_player ? GAME_UI.gold : GAME_UI.cyan, 'right');
    });
    const isLoading = Boolean(this.leaderboardRemoteLoading[leaderboardKey]);
    const loadFailed = Boolean(this.leaderboardRemoteFailedAt[leaderboardKey]);
    if (!entries.length) {
      this.drawFitText(
        isLoading ? '排行榜加载中…' : loadFailed ? '排行榜暂时无法加载，请检查后端连接' : '暂无排行榜数据',
        this.width / 2,
        listY + 300,
        this.width - 96,
        uiFont(18, 700),
        GAME_UI.secondary,
      );
      this.drawGamePanel(132, listY + 348, this.width - 264, 88, 'gold', {
        radius: 20,
        fill: 'rgba(255,211,77,0.08)',
        stroke: 'rgba(255,211,77,0.34)',
        shadow: false,
      });
      this.drawFitText('我的最好成绩', this.width / 2, listY + 378, this.width - 330, uiFont(14, 700), GAME_UI.secondary);
      this.drawFitText(String(personal.score), this.width / 2, listY + 414, this.width - 330, uiFont(26, 900), GAME_UI.gold);
    } else {
      const personalY = listY + 586;
      this.drawGamePanel(56, personalY, this.width - 112, 76, 'gold', {
        radius: 18,
        fill: this.glassFill(56, personalY, this.width - 112, 76, 'gold'),
        stroke: 'rgba(255,211,77,0.70)',
        shadow: false,
      });
      const rankText = personal.rank > 0 ? `第 ${personal.rank} 名` : '暂未上榜';
      this.drawFitText(personal.name || '我的成绩', 80, personalY + 25, 250, uiFont(15, 900), GAME_UI.text, 'left');
      this.drawFitText(rankText, 80, personalY + 51, 250, uiFont(12, 600), GAME_UI.secondary, 'left');
      this.drawFitText(String(personal.score), 646, personalY + 38, 120, uiFont(23, 900), GAME_UI.gold, 'right');
    }
    const footer = remoteLeaderboard
      ? '服务端实时排行榜 · 数据来自真实玩家成绩'
      : isLoading
        ? '正在从服务端读取排行榜…'
        : loadFailed
          ? '请确认后端服务和数据库迁移已完成'
          : '排行榜暂无数据';
    this.drawFitText(footer, this.width / 2, listY + 738, this.width - 96, uiFont(13, 500), GAME_UI.secondary);
  }

  drawRecords() {
    this.buttons = [];
    this.drawGameHeader('挑战记录', '‹ 返回', () => this.goHome());
    const top = this.pageTop();
    const summaryY = this.screenContentTop(102);
    const stats = playerStats.summary(this.progress);
    const friend = this.progress.friend_matches || {};
    const endless = this.progress.endless || {};
    this.drawGamePanel(32, summaryY, this.width - 64, 202, 'cyan', {
      radius: 26,
      shadowColor: 'rgba(80,227,255,0.24)',
      shadowBlur: 14,
      shadowOffsetY: 0,
      hotspot: { x: 112, y: summaryY + 30, r: 220, color: 'rgba(40,233,255,0.09)' },
    });
    this.drawText('总体数据', 60, summaryY + 42, uiFont(22, 900), GAME_UI.cyan, 'left');
    this.drawDashboardMetric(70, summaryY + 80, (cx, cy, scale, color) => this.drawTargetIcon(cx, cy, scale, color), '答对题数', String(safeNumber(stats.total_solved)), 'cyan');
    this.drawDashboardMetric(370, summaryY + 80, (cx, cy, scale, color) => this.drawLightningIcon(cx, cy, scale, color), '最高连击', String(safeNumber(stats.best_combo)), 'violet');
    this.drawDashboardMetric(70, summaryY + 140, null, '累计得分', String(safeNumber(stats.total_score)), 'gold');
    this.drawText('+', 98, summaryY + 170, uiFont(46, 900), 'rgba(154,100,255,0.28)');
    this.drawDashboardMetric(370, summaryY + 140, (cx, cy, scale, color) => this.drawClockIcon(cx, cy, scale, color), '最快用时', stats.fastest_ms ? `${(stats.fastest_ms / 1000).toFixed(1)} 秒` : '暂无', 'cyan');
    const cards = [
      ['闯关模式', `已解锁 ${this.highestPlayableLevelNumber()} 关`, 'cyan', (cx, cy) => this.drawFlagIcon(cx, cy, 0.44)],
      ['无尽模式', `最高 ${safeNumber(endless.best_questions)} 题`, 'violet', (cx, cy) => this.drawInfinityIcon(cx, cy, 0.38)],
      ['好友对战', `胜利 ${safeNumber(friend.wins)} / ${safeNumber(friend.played)} 场`, 'magenta', (cx, cy) => this.drawVsStar(cx, cy, 0.34)],
      ['主题收藏', `${(this.progress.owned_skins || []).length} / ${skinCatalog.all().length} 套`, 'gold', (cx, cy) => this.drawStarIcon(cx, cy, 0.48)],
    ];
    cards.forEach((card, index) => {
      const col = index % 2;
      const row = Math.floor(index / 2);
      const x = 32 + col * 344;
      const y = summaryY + 252 + row * 168;
      this.drawGamePanel(x, y, 326, 140, card[2], { radius: 24, shadowBlur: 13, shadowOffsetY: 0 });
      card[3](x + 68, y + 70);
      this.drawFitText(card[0], x + 128, y + 56, 180, uiFont(22, 900), GAME_UI.text, 'left');
      this.drawFitText(card[1], x + 128, y + 96, 180, uiFont(16, 500), this.accentColor(card[2]), 'left');
    });
    this.drawFitText('记录保存在本机，换设备后可通过账号系统同步', this.width / 2, summaryY + 630, this.width - 96, uiFont(14, 500), GAME_UI.secondary);
  }

  drawVolumeBar(x, y, width, ratio, color) {
    const safeRatio = clamp(ratio, 0, 1);
    this.drawPanel(x, y, width, 10, 'rgba(255,255,255,0.10)', 'rgba(255,255,255,0.12)', 5, { shadow: false });
    this.drawPanel(x, y, width * safeRatio, 10, color, color, 5, { shadowColor: `${color}80`, shadowBlur: 7 });
    // 加大滑块的可视反馈，方便手机用户发现“这里可以拖动”。
    this.ctx.save();
    this.ctx.shadowColor = `${color}aa`;
    this.ctx.shadowBlur = 8;
    this.ctx.beginPath();
    this.ctx.arc(x + width * safeRatio, y + 5, 10, 0, Math.PI * 2);
    this.ctx.fillStyle = '#ffffff';
    this.ctx.fill();
    this.ctx.strokeStyle = color;
    this.ctx.lineWidth = 2;
    this.ctx.stroke();
    this.ctx.restore();
  }

  updateVolumeFromPoint(type, touchX) {
    const area = this.volumeDragAreas && this.volumeDragAreas[type];
    if (!area || !this.audio) return false;
    const barX = safeNumber(area.barX, area.x);
    const barWidth = Math.max(1, safeNumber(area.barWidth, area.width));
    const ratio = clamp((safeNumber(touchX) - barX) / barWidth, 0, 1);
    if (type === 'music' && this.audio.setMusicVolume) this.audio.setMusicVolume(ratio);
    else if (type === 'sfx' && this.audio.setSfxVolume) this.audio.setSfxVolume(ratio);
    else return false;
    this.progress.audio = this.audio.settings();
    return true;
  }

  roundRect(x, y, width, height, radius, fill, stroke, lineWidth) {
    const ctx = this.ctx;
    const r = Math.min(radius, width / 2, height / 2);
    ctx.beginPath();
    ctx.moveTo(x + r, y);
    ctx.arcTo(x + width, y, x + width, y + height, r);
    ctx.arcTo(x + width, y + height, x, y + height, r);
    ctx.arcTo(x, y + height, x, y, r);
    ctx.arcTo(x, y, x + width, y, r);
    ctx.closePath();
    ctx.fillStyle = fill;
    ctx.fill();
    if (stroke) {
      ctx.strokeStyle = stroke;
      ctx.lineWidth = lineWidth || 1;
      ctx.stroke();
    }
  }

  onTouch(event) {
    const touch = event && event.touches && event.touches[0];
    this.lastTouchHandled = false;
    this.lastHandledTouchPoint = null;
    if (!touch) return;
    const touchX = this.touchCoordinate(touch, 'x');
    const touchY = this.touchCoordinate(touch, 'y');
    // wx.onTouchStart 每次回调就是一次新的手指落点，不能按时间或位置
    // 丢弃第二次点击：玩家可能连续两步点击同一张合成卡片，
    // 例如“运算符 → 第二个数字 → 下一步再点同一列的卡片”。
    // 绘制采用等比缩放并预留安全区，触摸坐标必须使用同一变换反算。
    const pointCandidates = this.touchPointCandidates(touch);
    const primary = pointCandidates[0] || { x: 0, y: 0 };
    const x = primary.x;
    const y = primary.y;
    // 某些真机/开发者工具版本可能不派发 touchend；新的一次落点必须先释放旧滑块状态。
    this.volumeDragType = '';
    if (this.hintPopup) {
      this.hintPopup = null;
      this.touchEffect = { x, y, time: Date.now() / 1000, color: COLORS.magenta || COLORS.pink };
      this.lastTouchHandled = true;
      this.lastHandledTouchPoint = { x, y };
      return;
    }
    if (this.resultHelpPopup) {
      const helpHit = this.buttons.slice().reverse().find((button) => x >= button.x && x <= button.x + button.width && y >= button.y && y <= button.y + button.height);
      if (helpHit && !helpHit.disabled) this.invokeTouchAction(helpHit.action, helpHit.key || 'modal');
      else this.resultHelpPopup = false;
      this.lastTouchHandled = true;
      this.lastHandledTouchPoint = { x, y };
      return;
    }
    // 好友连接恢复层只属于好友大厅/对战页面，不能拦截排行榜、商城、设置等其他页面的点击。
    const friendRecoveryScreen = ['friend_lobby', 'game', 'result'].includes(this.screen);
    if (friendRecoveryScreen && (this.friendConnectionState === 'reconnecting' || this.friendConnectionState === 'reconnect_timeout' || this.friendRoomExpired)) {
      for (const point of pointCandidates) {
        const connectionHit = this.buttons.slice().reverse().find((button) => point.x >= button.x && point.x <= button.x + button.width && point.y >= button.y && point.y <= button.y + button.height && !button.disabled);
        if (connectionHit) {
          try { this.audio.playClick(); } catch (error) { /* network recovery must remain usable without audio */ }
          this.invokeTouchAction(connectionHit.action, connectionHit.key || 'friend-connection');
        }
        this.lastTouchHandled = true;
        this.lastHandledTouchPoint = { x: point.x, y: point.y };
        return;
      }
    }
    if (this.friendCountdownActive) {
      this.touchEffect = { x, y, time: Date.now() / 1000, color: COLORS.magenta || COLORS.pink };
      this.lastTouchHandled = true;
      this.lastHandledTouchPoint = { x, y };
      return;
    }
    // 重开按钮属于“恢复操作”，不能被自动跳题过渡状态拦截。
    // 真机上偶尔会丢失最后一次 touchend，玩家仍然应该可以立即重开本局。
    let restartHit = null;
    let restartPoint = primary;
    for (const point of pointCandidates) {
      const candidate = this.buttons.slice().reverse().find((button) => {
        const isRestart = button.key === 'game-restart' || button.key === 'runtime-restart';
        return isRestart
          && point.x >= button.x && point.x <= button.x + button.width
          && point.y >= button.y && point.y <= button.y + button.height;
      });
      if (candidate) {
        restartHit = candidate;
        restartPoint = point;
        break;
      }
    }
    if (restartHit && !restartHit.disabled) {
      this.touchEffect = { x: restartPoint.x, y: restartPoint.y, time: Date.now() / 1000, color: COLORS.cyan };
      try { this.audio.playClick(); } catch (error) { /* 音效失败不能阻断重开 */ }
      this.invokeTouchAction(restartHit.action, restartHit.key || 'restart');
      this.lastTouchHandled = true;
      this.lastHandledTouchPoint = { x: restartPoint.x, y: restartPoint.y };
      return;
    }

    if (this.transitioning) {
      // 如果最后一次触摸或某个定时器被微信丢失，允许玩家再次点击把“只剩 24”状态自恢复。
      const solved = this.screen === 'game' && Array.isArray(this.cards) && this.cards.length === 1
        && this.cards[0] && Math.abs(Number(this.cards[0].value) - 24) < 0.000001;
      if (solved && !this.settling && this.autoNextAt > 0) {
        this.invokeTouchAction(() => this.nextQuestion(), 'next-question');
        this.lastTouchHandled = true;
      }
      return;
    }

    // 游戏卡片优先走当前帧重新计算的命中测试。
    // 微信真机偶尔会在 Canvas 重绘前派发 touchstart，此时 buttons 可能还是上一帧的列表；
    // 如果先查通用按钮，最后两张卡片的点击可能被旧热区吞掉，表现为“选了运算符后死机”。
    if (this.screen === 'game') {
      // 先处理提示、重置、撤销和运算符等控件。备用坐标候选不能抢在主坐标前，
      // 也不能把按钮点击误判成数字卡片点击。
      // Resolve cards before generic controls across all coordinate candidates.
      // On some phones one fallback candidate can map a card tap into the
      // header area; the card hit must win over the back button in that case.
      for (const point of pointCandidates) {
        const cardIndex = this.findGameCardAtPoint(point.x, point.y);
        if (cardIndex >= 0) {
          this.touchEffect = { x: point.x, y: point.y, time: Date.now() / 1000, color: COLORS.cyan };
          this.invokeTouchAction(() => this.selectCard(cardIndex), `card-${cardIndex}`);
          this.lastTouchHandled = true;
          this.lastHandledTouchPoint = { x: point.x, y: point.y };
          return;
        }
      }

      let controlHit = null;
      let controlPoint = primary;
      for (const point of pointCandidates) {
        const controlButtons = this.buttons.filter((button) => !String(button.key || '').startsWith('game-card-'));
        const candidate = hitTest.findButtonAtPoint(controlButtons, point.x, point.y);
        if (candidate) {
          controlHit = candidate;
          controlPoint = point;
          break;
        }
      }
      if (controlHit) {
        this.touchEffect = { x: controlPoint.x, y: controlPoint.y, time: Date.now() / 1000, color: controlHit.disabled ? COLORS.muted : COLORS.cyan };
        if (!controlHit.disabled) {
          try { this.audio.playClick(); } catch (error) { /* 控件音效失败不能阻断玩法 */ }
          if (controlHit.dragType) {
            this.volumeDragType = controlHit.dragType;
            this.updateVolumeFromPoint(controlHit.dragType, controlPoint.x);
            this.saveAudioSettings();
          } else {
            this.invokeTouchAction(controlHit.action, controlHit.key || 'game-control');
          }
        }
        this.lastTouchHandled = true;
        this.lastHandledTouchPoint = { x: controlPoint.x, y: controlPoint.y };
        return;
      }
      for (const point of pointCandidates) {
        const cardIndex = this.findGameCardAtPoint(point.x, point.y);
        if (cardIndex >= 0) {
          this.touchEffect = { x: point.x, y: point.y, time: Date.now() / 1000, color: COLORS.cyan };
          this.invokeTouchAction(() => this.selectCard(cardIndex), `card-${cardIndex}`);
          this.lastTouchHandled = true;
          this.lastHandledTouchPoint = { x: point.x, y: point.y };
          return;
        }
      }
    }
    let hitPoint = primary;
    let hit = null;
    for (const point of pointCandidates) {
        const candidate = hitTest.findButtonAtPoint(this.buttons, point.x, point.y);
      if (candidate) { hit = candidate; hitPoint = point; break; }
    }
    const hitX = hitPoint.x;
    const hitY = hitPoint.y;
    this.touchEffect = { x: hitX, y: hitY, time: Date.now() / 1000, color: hit && hit.disabled ? COLORS.muted : COLORS.cyan };
    if (hit && !hit.disabled) {
      if (hit.dragType) {
        this.volumeDragType = hit.dragType;
        this.updateVolumeFromPoint(hit.dragType, hitX);
        this.saveAudioSettings();
        this.lastTouchHandled = true;
        this.lastHandledTouchPoint = { x: hitX, y: hitY };
        return;
      }
      if (this.screen === 'home' && hit.key) this.homeMotion = { activeButton: hit.key, activeUntil: Date.now() + 140 };
      try { this.audio.playClick(); } catch (error) { /* 音效失败不能阻断按钮 */ }
      this.invokeTouchAction(hit.action, hit.key || 'button');
      this.lastTouchHandled = true;
        this.lastHandledTouchPoint = { x: hitX, y: hitY };
    }
    else if (this.screen === 'game') this.lastTouchHandled = this.handleGameTouch(hitX, hitY);
  }

  touchPointCandidates(touch) {
    return touchGeometry.touchPointCandidates(this, touch);
  }

  findGameCardAtPoint(x, y) {
    return hitTest.findGameCardAtPoint(this, x, y);
  }

  onTouchMove(event) {
    if (!this.volumeDragType || this.popup !== 'settings') return;
    const touch = event && event.touches && event.touches[0];
    if (!touch) return;
    const touchX = this.touchCoordinate(touch, 'x');
    const touchY = this.touchCoordinate(touch, 'y');
    const x = (safeNumber(touchX) - this.renderOffsetX) / this.renderScale;
    const y = (safeNumber(touchY) - this.renderOffsetY) / this.renderScale;
    const area = this.volumeDragAreas && this.volumeDragAreas[this.volumeDragType];
    if (!area) return;
    // 允许手指稍微移出细条，但仍限制在对应的音量卡片内，手机上更容易拖动。
    if (y < area.y - 24 || y > area.y + area.height + 24) return;
    // 拖动过程中只更新内存和画面，避免每个 touchmove 都同步本地存储造成卡顿；
    // 在 touchend（以及下一次点击开始时）统一保存。
    this.updateVolumeFromPoint(this.volumeDragType, x);
  }

  onTouchEnd(event) {
    // 少数真机版本会在 touchstart 阶段给出空 touches，或在 Canvas 缩放时丢掉
    // touchstart 的坐标。用 touchend 做一次只执行一次的兜底，专门保护最后一个数字。
    if (this.screen === 'game' && !this.transitioning) {
      const touch = event && event.changedTouches && event.changedTouches[0] || event && event.touches && event.touches[0];
      if (touch) {
        const previous = this.lastHandledTouchPoint;
        for (const point of this.touchPointCandidates(touch)) {
          const index = this.findGameCardAtPoint(point.x, point.y);
          const sameTouch = previous && Math.abs(previous.x - point.x) <= 18 && Math.abs(previous.y - point.y) <= 18;
          // touchstart 已成功处理同一个点时不重复合成；touchstart 丢失或命中旧热区时，
          // touchend 仍会把当前卡片补交给状态机。
          if (index >= 0 && !sameTouch) {
            this.invokeTouchAction(() => this.selectCard(index), `card-${index}-end`);
            this.lastTouchHandled = true;
            this.lastHandledTouchPoint = { x: point.x, y: point.y };
            break;
          }
        }
      }
    }
    if (this.volumeDragType) this.saveAudioSettings();
    this.volumeDragType = '';
  }

  touchCoordinate(touch, axis) {
    return touchGeometry.touchCoordinate(touch, axis);
  }

  handleGameTouch(x, y) {
    const layout = this.gameLayout();
    const cardWidth = layout.cardWidth;
    const cardHeight = layout.cardHeight;
    const gapX = layout.gapX;
    const gapY = layout.gapY;
    const contentTop = this.gameContentTop();
    const startX = (this.width - cardWidth * 2 - gapX) / 2;
    const startY = layout.cardStartY;
    for (let index = 0; index < this.cards.length; index += 1) {
      const rect = this.cardRect(index, startX, startY, cardWidth, cardHeight, gapX, gapY);
      if (x >= rect.x && x <= rect.x + rect.width && y >= rect.y && y <= rect.y + rect.height) {
        return this.invokeTouchAction(() => this.selectCard(index), `card-${index}-fallback`);
      }
    }
  }

  showLevels() {
    this.popup = '';
    this.hintPopup = null;
    this.menuPage = clamp(Math.floor(safeNumber(this.progress.unlocked_level, 0) / 20), 0, 9);
    this.screen = 'levels';
  }

  ensureQuestionService() {
    if (this.questionService) return this.questionService;
    this.questionService = new questionServiceModule.QuestionService({
      generator: puzzle,
      levels: Array.isArray(this.levels) ? this.levels : levelCatalog.all(),
      campaignData: campaignPuzzleData,
      campaignBank: this.campaignPuzzleBank,
      campaignSeedBase: 240000,
    });
    return this.questionService;
  }

  ensureCampaignPuzzleBank() {
    if (Array.isArray(this.campaignPuzzleBank) && this.campaignPuzzleBank.length === this.levels.length) return this.campaignPuzzleBank;
    const service = this.ensureQuestionService();
    const bank = service.ensureCampaignBank();
    if (bank) this.campaignPuzzleBank = bank;
    return bank;
  }

  calculateCampaignStars(config = {}) {
    // 闯关模式只看统一的关卡总分，避免不同关卡的目标分越来越高、玩家无法理解星级。
    const score = clamp(Math.floor(safeNumber(this.score, 0)), 0, 100);
    const stars = score >= 100 ? 3 : score >= 80 ? 2 : score >= 60 ? 1 : 0;
    const starDetails = [
      { star: 1, label: '关卡总分达到 60 分', met: score >= 60 },
      { star: 2, label: '关卡总分达到 80 分', met: score >= 80 },
      { star: 3, label: '关卡总分达到 100 分', met: score >= 100 },
    ];
    const failed = starDetails.filter((item) => !item.met);
    const summary = stars >= 3
      ? '三星达成：关卡总分 100 分'
      : `${stars}星 · ${failed.length ? `还可完成：${failed[0].label}` : '继续保持表现'}`;
    return { stars, summary, starDetails };
  }

  calculateGeneralStars() {
    const mistakes = Math.max(0, Math.floor(safeNumber(this.mistakes, 0)));
    const timeReached = safeNumber(this.timeLeft, 0) > safeNumber(this.timerLimit, 1) * 0.45;
    const stars = mistakes === 0 && timeReached ? 3 : mistakes <= 1 ? 2 : 1;
    const starDetails = [
      { star: 3, label: '全程 0 错误', met: mistakes === 0 },
      { star: 3, label: '剩余时间超过 45%', met: timeReached },
      { star: 2, label: '错误次数不超过 1 次', met: mistakes <= 1 },
    ];
    const failed = starDetails.filter((item) => !item.met);
    return {
      stars,
      starDetails,
      summary: stars >= 3 ? '三星达成：0 错误且剩余时间超过 45%' : `${stars}星 · ${failed.length ? `还可完成：${failed[0].label}` : '继续保持表现'}`,
    };
  }

  finish(passed, reason) {
    try {
      return this.finishInternal(passed, reason);
    } catch (error) {
      this.handleFinishError(error, passed, reason);
      return false;
    }
  }

  handleFinishError(error, passed, reason) {
    const errorMessage = String(error && (error.stack || error.message) || error || 'unknown finish error');
    this.lastFinishError = { message: errorMessage, time: Date.now() };
    try { storage.appendErrorLog('finish', error, { mode: this.mode, screen: this.screen }); } catch (logError) { /* 日志失败不能影响结算恢复 */ }
    try { if (typeof console !== 'undefined' && console.error) console.error('[24点挑战][finish]', error); } catch (logError) { /* 静默降级 */ }
    const safePassed = Boolean(passed);
    const isLastQuestion = this.mode === 'campaign' || this.mode === 'daily'
      ? this.currentQuestion >= (Array.isArray(this.puzzles) ? this.puzzles.length - 1 : 0)
      : this.mode === 'friend'
        ? this.currentQuestion >= this.friendQuestionCount() - 1
        : false;
    if (safePassed && !isLastQuestion && this.screen === 'game') {
      // 题目级结算异常不能提前展示“本局结算”。本局结算只允许出现在整关最后一题。
      this.transitioning = true;
      this.autoNextToken += 1;
      this.autoNextAt = Date.now();
      this.autoNextFallbackMs = 0;
      this.hintPopup = null;
      this.status = '答对啦！正在进入下一题';
      return;
    }
    if (safePassed && this.mode === 'daily' && isLastQuestion) {
      try {
        const date = storage.todayKey();
        this.progress.daily = this.progress.daily || { last_date: '', streak: 0, best_score: 0, completed: {}, reward_claimed: {} };
        this.progress.daily.completed = this.progress.daily.completed || {};
        this.progress.daily.completed[date] = true;
        this.progress.daily.last_date = date;
        storage.save(this.progress);
      } catch (dailySaveError) {
        try { if (typeof console !== 'undefined' && console.warn) console.warn('[24点挑战][daily-completion-fallback]', dailySaveError); } catch (logError) { /* 静默降级 */ }
      }
    }
    this.transitioning = false;
    this.autoNextAt = 0;
    this.autoNextFallbackMs = 0;
    const campaignConfig = this.mode === 'campaign' && this.levels && this.levels[this.currentLevel]
      ? this.levels[this.currentLevel]
      : {};
    if (safePassed && this.mode === 'campaign' && isLastQuestion) {
      this.score = this.campaignProgressScore(this.currentQuestion + 1);
    }
    const starResult = safePassed && this.mode === 'campaign' && isLastQuestion
      ? this.calculateCampaignStars(campaignConfig)
      : { stars: safePassed ? 1 : 0, summary: '', starDetails: [] };
    const levelComplete = safePassed && this.mode === 'campaign' && isLastQuestion;
    // 结算的存档、任务、排行榜属于附属功能，即使其中一个异常，也不能把已经完成的题目降级成 1 星。
    if (levelComplete) {
      try {
        if (this.progress && this.progress.levels && storage && storage.saveLevel) {
          storage.saveLevel(this.progress, this.currentLevel, starResult.stars, safeNumber(this.score));
        }
      } catch (saveError) {
        try { if (typeof console !== 'undefined' && console.warn) console.warn('[24点挑战][finish-save-fallback]', saveError); } catch (logError) { /* 静默降级 */ }
      }
    }
    const hasNextLevel = levelComplete
      && this.currentLevel < (Array.isArray(this.levels) ? this.levels.length - 1 : 0)
      && this.isCampaignLevelUnlocked(this.currentLevel + 1);
    this.result = {
      passed: safePassed,
      score: safeNumber(this.score),
      stars: starResult.stars,
      starSummary: starResult.summary,
      starDetails: starResult.starDetails || [],
      combo: safeNumber(this.maxCombo),
      mistakes: safeNumber(this.mistakes),
      reason: reason || (safePassed ? '本题已完成' : '结算发生异常'),
      rewardCoins: 0,
      bonusLabels: [],
      levelComplete,
      next: safePassed && (!isLastQuestion || hasNextLevel),
    };
    this.screen = 'result';
    this.renderRecovery = false;
  }

  finishInternal(passed, reason) {
    if (this.screen !== 'game' || this.transitioning) return;
    const scoreBefore = this.score;
    if (passed) {
      if (this.mode === 'campaign') {
        // 闯关模式固定 100 分制：按已完成题目进度计分，再扣错误和提示分。
        this.score = this.campaignProgressScore(this.currentQuestion + 1);
      } else {
        const gained = Math.max(10, Math.round(this.timeLeft * 6 + this.combo * 30 - this.mistakes * 5));
        this.score += gained;
      }
      this.combo += 1;
      this.maxCombo = Math.max(this.maxCombo, this.combo);
      if (this.mode === 'friend') this.friendPlayerSolved += 1;
      this.audio.playSuccess();
      this.triggerFeedback('success', '答对啦！');
    } else {
      this.combo = 0;
      this.audio.playError();
      this.triggerFeedback('error', reason || '再试一次');
    }
    const isLast = this.mode === 'campaign' ? this.currentQuestion >= this.puzzles.length - 1 : this.mode === 'daily' ? this.currentQuestion >= this.puzzles.length - 1 : this.mode === 'friend' ? this.currentQuestion >= this.friendQuestionCount() - 1 : false;
    if (this.mode === 'friend' && this.currentPuzzle) {
      const timeLimit = this.friendTimeLimit();
      const elapsedMs = Math.round(clamp(timeLimit - Math.max(0, this.timeLeft), 0, timeLimit) * 1000);
      const contract = this.friendMatchContract || {};
      const attempt = matchData.createAttemptRecord(contract, this.currentPuzzle, {
        question_index: this.currentQuestion,
        elapsed_ms: elapsedMs,
        solved: passed,
        mistakes: this.mistakes,
        score: this.score,
        score_delta: Math.max(0, this.score - scoreBefore),
        event_id: `${String(contract.match_id || 'friend')}:${this.currentQuestion}:${Array.isArray(this.friendAttempts) ? this.friendAttempts.length : 0}:${Date.now()}`,
        operations: Array.isArray(this.questionSteps) ? this.questionSteps.map((step) => ({ ...step })) : [],
        solution_steps: this.questionSteps,
      });
      this.friendAttempts = Array.isArray(this.friendAttempts) ? this.friendAttempts : [];
      this.friendAttempts.push(attempt);
      this.sendFriendMatchProgress(!passed || isLast);
    }
    if (this.mode === 'endless' && this.endlessRun && this.currentPuzzle) {
      const elapsedMs = Math.round(clamp(this.timerLimit - Math.max(0, this.timeLeft), 0, this.timerLimit) * 1000);
      this.endlessAttempts = Array.isArray(this.endlessAttempts) ? this.endlessAttempts : [];
      this.endlessAttempts.push({
        puzzle_id: String(this.currentPuzzle.puzzleId || this.currentPuzzle.puzzle_id || `ENDLESS-Q${this.currentQuestion + 1}`),
        question_index: this.currentQuestion,
        elapsed_ms: elapsedMs,
        solved: Boolean(passed),
        mistakes: Math.max(0, Number(this.mistakes) || 0),
        score: Math.max(0, Number(this.score) || 0),
        score_delta: Math.max(0, Number(this.score) - Number(scoreBefore) || 0),
        combo: Math.max(0, Number(this.combo) || 0),
        solution_steps: passed && Array.isArray(this.questionSteps) ? this.questionSteps.map((step) => ({ ...step })) : [],
      });
    }
    if (this.mode === 'campaign' && this.campaignRun && this.currentPuzzle && passed) {
      const elapsedMs = Math.round(clamp(this.timerLimit - Math.max(0, this.timeLeft), 0, this.timerLimit) * 1000);
      this.campaignAttempts = Array.isArray(this.campaignAttempts) ? this.campaignAttempts : [];
      this.campaignAttempts.push({
        puzzle_id: String(this.currentPuzzle.puzzleId || this.currentPuzzle.puzzle_id || `C${String(this.currentLevel + 1).padStart(3, '0')}-Q${String(this.currentQuestion + 1).padStart(2, '0')}`),
        question_index: this.currentQuestion,
        elapsed_ms: elapsedMs,
        solved: true,
        mistakes: Math.max(0, Number(this.mistakes) || 0),
        hints: this.hintUsed ? 1 : 0,
        score: Math.max(0, Math.min(100, Number(this.score) || 0)),
        score_delta: Math.floor(Number(this.score) - Number(scoreBefore) || 0),
        combo: Math.max(0, Number(this.combo) || 0),
        solution_steps: Array.isArray(this.questionSteps) ? this.questionSteps.map((step) => ({ ...step })) : [],
      });
    }
    if (this.mode === 'daily' && this.dailyRun && this.currentPuzzle && passed) {
      const elapsedMs = Math.round(clamp(this.timerLimit - Math.max(0, this.timeLeft), 0, this.timerLimit) * 1000);
      this.dailyAttempts = Array.isArray(this.dailyAttempts) ? this.dailyAttempts : [];
      this.dailyAttempts.push({
        puzzle_id: String(this.currentPuzzle.puzzleId || this.currentPuzzle.puzzle_id || `D${storage.todayKey().replace(/-/g, '')}-Q${String(this.currentQuestion + 1).padStart(2, '0')}`),
        question_index: this.currentQuestion,
        elapsed_ms: elapsedMs,
        solved: true,
        mistakes: Math.max(0, Number(this.mistakes) || 0),
        hints: this.hintUsed ? 1 : 0,
        score: Math.max(0, Number(this.score) || 0),
        score_delta: Math.max(0, Math.floor(Number(this.score) - Number(scoreBefore) || 0)),
        combo: Math.max(1, Number(this.combo) || 1),
        solution_steps: Array.isArray(this.questionSteps) ? this.questionSteps.map((step) => ({ ...step })) : [],
      });
    }
    let stars = 0;
    let starSummary = '';
    let starDetails = [];
    let levelComplete = false;
    let rewardCoins = 0;
    let bonusLabels = [];
    const serverAuthoritative = Boolean(this.backendAuth && this.backendAuth.status === 'ready' && (
      (this.mode === 'campaign' && this.campaignRun) ||
      (this.mode === 'daily' && this.dailyRun) ||
      (this.mode === 'endless' && this.endlessRun) ||
      (this.mode === 'friend' && !this.friendLocalFallback)
    ));
    if (passed && this.mode === 'campaign' && isLast) {
      const config = this.levels[this.currentLevel] || {};
      const starResult = this.calculateCampaignStars(config);
      stars = starResult.stars;
      starSummary = starResult.summary;
      starDetails = starResult.starDetails || [];
      const oldRecord = this.progress.levels[String(this.currentLevel)] || {};
      const newLevelClear = safeNumber(oldRecord.best_score, 0) <= 0;
      if (!serverAuthoritative) {
        rewardCoins = storage.claimLevelReward(this.progress, this.currentLevel, stars);
        const bonus = storage.claimCampaignBonus(
          this.progress,
          this.currentLevel,
          stars,
          safeNumber(config.chapterIndex, Math.floor(this.currentLevel / 20)),
          this.currentLevel % 20 === 19,
          newLevelClear,
        );
        rewardCoins += bonus.coins;
        bonusLabels = bonus.labels;
      }
      storage.saveLevel(this.progress, this.currentLevel, stars, this.score);
      levelComplete = true;
      this.submitCampaignLevelCompletion(this.currentLevel, this.score, stars);
    } else if (passed) {
      const starResult = this.calculateGeneralStars();
      stars = starResult.stars;
      starSummary = starResult.summary;
      starDetails = starResult.starDetails || [];
    }
    if (passed && this.mode === 'daily' && isLast) {
      const date = storage.todayKey();
      if (!serverAuthoritative) {
      const alreadyCompleted = storage.isDailyCompleted(this.progress, date);
      const previousDate = previousDateKey(date);
      this.progress.daily.completed[date] = true;
      this.progress.daily.best_score = Math.max(safeNumber(this.progress.daily.best_score), this.score);
      if (!alreadyCompleted) {
        this.progress.daily.streak = previousDate && storage.isDailyCompleted(this.progress, previousDate)
          ? safeNumber(this.progress.daily.streak) + 1 : 1;
      }
      this.progress.daily.last_date = date;
      this.progress.daily.reward_claimed = this.progress.daily.reward_claimed || {};
      if (!alreadyCompleted && !this.progress.daily.reward_claimed[date]) {
        this.progress.daily.reward_claimed[date] = true;
        rewardCoins += 15;
        storage.addCoins(this.progress, 15);
        bonusLabels.push('每日挑战完成 +15');
      }
      const dailyStreak = safeNumber(this.progress.daily.streak);
      if (dailyStreak >= 3) bonusLabels.push('连续挑战 3 天');
      if (dailyStreak >= 7) bonusLabels.push('连续挑战 7 天');
      storage.save(this.progress);
      }
      this.submitDailyChallengeCompletion(this.score);
    }
    if (this.mode === 'endless') {
      this.progress.endless.best_score = Math.max(safeNumber(this.progress.endless.best_score), this.score);
      const questionsReached = this.currentQuestion + (passed ? 1 : 0);
      this.progress.endless.best_questions = Math.max(safeNumber(this.progress.endless.best_questions), questionsReached);
      this.progress.endless.best_combo = Math.max(safeNumber(this.progress.endless.best_combo), this.maxCombo);
      this.progress.endless.best_stage = Math.max(safeNumber(this.progress.endless.best_stage), Math.floor(Math.max(0, questionsReached - 1) / 3) + 1);
      this.progress.endless.last_score = this.score;
      if (!serverAuthoritative) {
      const endlessReward = storage.claimEndlessReward(this.progress, this.endlessRunId, questionsReached, storage.todayKey());
      if (endlessReward > 0) {
        rewardCoins += endlessReward;
        bonusLabels.push(`无尽奖励 +${endlessReward}`);
      }
      storage.save(this.progress);
      }
      this.submitEndlessLeaderboard(this.score, questionsReached, Math.max(0, (this.timerLimit - this.timeLeft) * 1000));
    }
    let matchResult = null;
    let matchReward = 0;
    if (this.mode === 'friend' && (!passed || isLast)) {
      const timeLimit = this.friendTimeLimit();
      const elapsed = clamp(timeLimit - Math.max(0, this.timeLeft), 0, timeLimit);
      const opponent = this.friendOpponentState();
      matchResult = this.friendLocalFallback
        ? friendMatch.calculateResult(this.friendPlayerSolved, this.score, this.mistakes, elapsed, opponent)
        : {
          outcome: 'pending',
          player_solved: this.friendPlayerSolved,
          player_score: this.score,
          player_mistakes: this.mistakes,
          player_elapsed: elapsed,
          opponent_solved: opponent.solved,
          opponent_score: opponent.score,
          opponent_elapsed: opponent.elapsed,
        };
      const submission = matchData.createResultSubmission(this.friendMatchContract || {
        match_id: this.friendMatch && this.friendMatch.match_id,
        room_id: this.friendRoom && this.friendRoom.room_id,
        room_seed: this.friendSeed,
        question_count: this.friendQuestionCount(),
        question_hash: '',
        puzzle_ids: this.puzzles.map((record, index) => String(record.puzzleId || record.puzzle_id || `Q${index + 1}`)),
      }, this.friendAttempts, {
        player_solved: matchResult.player_solved,
        player_score: matchResult.player_score,
        player_mistakes: matchResult.player_mistakes,
        player_elapsed: matchResult.player_elapsed,
        outcome: matchResult.outcome,
      });
      submission.idempotency_key = `friend_${String(this.friendMatch && this.friendMatch.match_id || this.friendRoom && this.friendRoom.room_id || 'friend')}_${Number(this.friendStartedAt || Date.now())}`;
      if (this.friendLocalFallback) {
        matchReward = this.applyFriendMatchResult(matchResult, elapsed, bonusLabels);
      } else {
        this.submitFriendLeaderboard(matchResult, this.friendRoom, submission);
      }
      storage.save(this.progress);
    }
    const dateKey = storage.todayKey();
    if (passed && !serverAuthoritative) {
      const modeId = this.mode === 'campaign' ? 'campaign' : this.mode === 'daily' ? 'daily' : this.mode === 'endless' ? 'endless' : 'friend';
      playerStats.recordSolve(this.progress, modeId, Math.max(0, (this.timerLimit - this.timeLeft) * 1000), Math.max(0, this.score - scoreBefore), this.combo, this.questionOperators, this.mode === 'campaign' ? this.currentLevel : -1);
      const taskRewards = [];
      if (this.mode === 'campaign' && levelComplete) taskRewards.push(taskService.record(this.progress, 'campaign_clear', 1, dateKey));
      if (this.mode === 'endless') taskRewards.push(taskService.record(this.progress, 'endless_questions', 1, dateKey));
      taskRewards.push(taskService.recordMax(this.progress, 'combo', this.maxCombo, dateKey));
      if (this.mode === 'campaign' && levelComplete) taskRewards.push(taskService.recordWeekly(this.progress, 'weekly_campaign', 1, dateKey));
      if (this.mode === 'daily' && isLast) taskRewards.push(taskService.recordWeekly(this.progress, 'weekly_daily', 1, dateKey));
      if (this.mode === 'endless') taskRewards.push(taskService.recordWeekly(this.progress, 'weekly_endless', 1, dateKey));
      if (this.mode === 'friend' && matchResult && matchResult.outcome === 'win') taskRewards.push(taskService.recordWeekly(this.progress, 'weekly_friend', 1, dateKey));
      taskRewards.filter((item) => item && item.reward > 0).forEach((item) => bonusLabels.push(`${item.title} +${item.reward}`));
      const unlockIds = [];
      if (this.mode === 'campaign' && levelComplete && this.currentLevel === 0) unlockIds.push('first_clear');
      if (stars >= 3) unlockIds.push('three_star');
      if (this.mistakes === 0 && this.mode === 'campaign' && levelComplete) unlockIds.push('perfect_clear');
      if (this.maxCombo >= 5) unlockIds.push('combo_5');
      if (this.maxCombo >= 10) unlockIds.push('combo_10');
      if (this.mode === 'endless' && this.currentQuestion >= 4) unlockIds.push('endless_5');
      if (this.mode === 'endless' && this.currentQuestion >= 9) unlockIds.push('endless_10');
      if (this.mode === 'endless' && this.currentQuestion >= 29) unlockIds.push('endless_30');
      if (this.mode === 'daily' && safeNumber(this.progress.daily.streak) >= 3) unlockIds.push('daily_3');
      if (this.mode === 'daily' && safeNumber(this.progress.daily.streak) >= 7) unlockIds.push('daily_7');
      const unlocks = achievementService.unlockMany(this.progress, unlockIds);
      if (unlocks.length) bonusLabels = bonusLabels.concat(unlocks.map((item) => `新成就 ${item.title} +${item.reward}`));
      const leaderboardMode = this.mode === 'campaign' ? leaderboardService.MODE_CAMPAIGN : this.mode === 'daily' ? leaderboardService.MODE_DAILY : this.mode === 'endless' ? leaderboardService.MODE_ENDLESS : this.mode === 'friend' ? leaderboardService.MODE_FRIEND : '';
      if (leaderboardMode) {
        const submitted = leaderboardService.submitScore(this.progress, leaderboardMode, this.score, { level: this.currentLevel, questions: this.currentQuestion + 1, mistakes: this.mistakes });
        if (submitted.new_record) bonusLabels.push('刷新个人最高分');
        if (!this.backendAuth || this.backendAuth.status !== 'ready') {
          platform.submitLeaderboard(leaderboardMode, this.score, { questions: this.currentQuestion + 1, mistakes: this.mistakes, clientAuthoritative: false });
        }
      }
      storage.save(this.progress);
    }
    const shouldAutoNext = Boolean(passed && !isLast);
    if (shouldAutoNext) {
      const transitionToken = ++this.autoNextToken;
      this.transitioning = true;
      this.autoNextAt = Date.now() + 260;
      this.status = '答对啦！正在进入下一题';
      this.hintPopup = null;
      setTimeout(() => {
        if (this.screen !== 'game' || !this.transitioning || transitionToken !== this.autoNextToken) return;
        this.nextQuestion();
      }, 260);
      // 某些微信运行环境可能暂停 setTimeout；loop() 会用 autoNextAt 再兜底一次。
      return;
    }
    const nextAvailable = passed && (this.mode === 'endless' || !isLast || (this.mode === 'campaign' && this.isCampaignLevelUnlocked(this.currentLevel + 1)));
    if (levelComplete && this.currentLevel === 99 && !this.isCampaignLevelUnlocked(100)) {
      bonusLabels.push(`下一阶段需要前 100 关累计 ${this.campaignBlockGateScore()} 分`);
    }
    this.result = {
      passed, score: this.score, stars, starSummary, starDetails, combo: this.maxCombo, mistakes: this.mistakes, reason,
      rewardCoins: rewardCoins + matchReward, bonusLabels, levelComplete, matchResult,
      next: nextAvailable,
    };
    this.screen = 'result';
  }

  nextQuestion() {
    if (this.screen !== 'game' && this.screen !== 'result') return;
    if (this.screen === 'game' && !this.transitioning) return;
    // 先锁住当前过渡，避免 setTimeout、主循环和重复点击同时推进两道题。
    this.transitioning = false;
    this.result = null;
    this.autoNextAt = 0;
    this.autoNextFallbackMs = 0;
    this.currentQuestion += 1;
    if (this.mode === 'endless') { this.beginEndlessQuestion(); return; }
    const config = this.levels[this.currentLevel] || {};
    const dailyLimit = this.dailyChallenge ? safeNumber(this.dailyChallenge.time_limit, 90) : 90;
    this.beginSession(this.mode === 'daily' ? dailyLimit : this.mode === 'friend' ? this.friendTimeLimit() : safeNumber(config.timeLimit || config.time_limit, 60));
  }

  restartMode() {
    // 统一清理所有可能拦截输入或让旧计时器继续运行的状态。
    // 这一步必须在分模式重启前执行，避免“撤销后卡住”或旧的自动跳题回调串入新题。
    this.renderRecovery = false;
    this.settling = false;
    this.settleToken += 1;
    this.transitioning = false;
    this.autoNextAt = 0;
    this.autoNextFallbackMs = 0;
    this.autoNextToken = safeNumber(this.autoNextToken) + 1;
    this.gameRequestToken += 1;
    this.result = null;
    this.hintPopup = null;
    this.resultHelpPopup = false;
    this.popup = '';
    this.selectedIndex = -1;
    this.selectedOperator = '';
    this.undoStack = [];
    this.questionOperators = [];
    this.status = '';
    this.gamePaused = false;

    if (this.mode === 'campaign') this.startCampaign(this.currentLevel, { forceRestart: true });
    else if (this.mode === 'daily') this.startDaily();
    else if (this.mode === 'endless') this.startEndless();
    else this.startFriend();
  }

  backFromGame() {
    this.gameRequestToken += 1;
    this.settling = false;
    this.settleToken += 1;
    this.transitioning = false;
    this.autoNextAt = 0;
    this.autoNextToken += 1;
    this.friendCountdownActive = false;
    this.friendCountdownUntil = 0;
    if (this.mode === 'campaign') this.showLevels();
    else this.goHome();
  }

  goHome() {
    this.gameRequestToken += 1;
    this.renderRecovery = false;
    this.settling = false;
    this.settleToken += 1;
    this.transitioning = false;
    this.autoNextAt = 0;
    this.autoNextToken += 1;
    this.popup = '';
    this.hintPopup = null;
    this.resultHelpPopup = false;
    this.progress = storage.load();
    this.ads.configure(this.progress.ads || {}, storage.todayKey());
    this.dateKey = storage.todayKey();
    this.gamePaused = false;
    this.friendCountdownActive = false;
    this.friendCountdownUntil = 0;
    this.friendServerStartAt = 0;
    this.screen = 'home';
  }
}

require('../modes/mode_controller.js').install(GameApp);
require('../modes/campaign_progress.js').install(GameApp);
require('../ui/page_layout.js').install(GameApp);
require('../ui/screen_renderer.js').install(GameApp);

function createGame() {
  if (typeof wx === 'undefined' || !wx.createCanvas) return null;
  return new GameApp();
}

module.exports = { GameApp, createGame };
