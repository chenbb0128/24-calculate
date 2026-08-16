/*
 * Button and interaction audit for the native WeChat Canvas game.
 * Run with: node tools/button_audit.js
 * This audit never creates a real wx instance and never writes player storage.
 */
const assert = require('assert');
const { GameApp } = require('../src/app.js');
const storage = require('../src/services/storage.js');
const levelCatalog = require('../src/core/level_catalog.js');
const dailyChallenge = require('../src/core/daily_challenge.js');
const friendMatch = require('../src/core/friend_match_service.js');
const leaderboardService = require('../src/services/leaderboard_service.js');
const hitTest = require('../src/input/hit_test.js');

function fakeGradient() { return { addColorStop() {} }; }

function fakeContext() {
  const state = {};
  return new Proxy(state, {
    get(target, prop) {
      if (prop === 'createLinearGradient' || prop === 'createRadialGradient') return fakeGradient;
      if (prop === 'measureText') return (value) => ({ width: String(value || '').length * 8 });
      if (prop === 'canvas') return { width: 750, height: 1334 };
      if (prop in target) return target[prop];
      return () => {};
    },
    set(target, prop, value) { target[prop] = value; return true; },
  });
}

function appFor(width = 375, height = 667) {
  const app = Object.create(GameApp.prototype);
  app.width = 750;
  app.viewportWidth = width;
  app.viewportHeight = height;
  app.renderScale = width / app.width;
  app.visibleHeight = height / app.renderScale;
  app.height = Math.max(1334, Math.round(app.visibleHeight));
  app.homeYScale = Math.min(1, app.height / 1584);
  app.safeTop = 24;
  app.safeBottom = 24;
  app.renderOffsetX = 0;
  app.renderOffsetY = 0;
  app.dpr = 1;
  app.ctx = fakeContext();
  app.menuButton = { top: 42, bottom: 82 };
  app.levels = levelCatalog.all();
  app.progress = storage.normalize({
    tutorial_seen: true,
    coins: 999999,
    unlocked_level: 58,
    last_level: 58,
    owned_skins: ['classic', 'ocean'],
    equipped_skin: 'classic',
    player_stats: { total_solved: 1234, total_score: 98765, best_combo: 12, fastest_ms: 1234 },
    endless: { best_questions: 18 },
    friend_matches: { wins: 2, played: 4 },
  });
  app.buttons = [];
  app.stars = [];
  app.floatNumbers = [];
  app.homeMotion = { activeButton: null, activeUntil: 0 };
  app.popup = '';
  app.hintPopup = null;
  app.resultHelpPopup = false;
  app.tutorialStep = 0;
  app.screen = 'home';
  app.mode = 'campaign';
  app.currentLevel = 0;
  app.currentQuestion = 0;
  app.puzzles = [{ rules: {} }, { rules: {} }, { rules: {} }];
  app.currentPuzzle = app.puzzles[0];
  app.cards = [{ value: 1 }, { value: 1 }, { value: 2 }, { value: 6 }];
  app.originalCards = app.cards.slice();
  app.selectedIndex = -1;
  app.selectedOperator = '';
  app.undoStack = [];
  app.freeUndo = true;
  app.freeHint = true;
  app.hintUsed = false;
  app.timeLeft = 65;
  app.timerLimit = 90;
  app.score = 80;
  app.combo = 3;
  app.mistakes = 1;
  app.maxCombo = 3;
  app.status = '';
  app.shopPage = 0;
  app.shopTab = 'themes';
  app.shopNotice = '';
  app.shopActionInFlight = '';
  app.achievementPage = 0;
  app.leaderboardBoard = leaderboardService.BOARD_GLOBAL;
  app.leaderboardMode = leaderboardService.MODE_CAMPAIGN;
  app.leaderboardRemote = {};
  app.leaderboardRemoteLoading = {};
  app.leaderboardRemoteFailedAt = {};
  app.friendRoom = friendMatch.createRoom(20260814);
  app.friendRoomBackendStatus = 'local';
  app.friendRoomFromInvite = false;
  app.friendLocalFallback = true;
  app.friendLobbyView = 'entry';
  app.friendRoomInput = '123456';
  app.friendSelfReady = true;
  app.friendReadyRequestInFlight = false;
  app.friendStartRequestInFlight = false;
  app.friendConnectionState = 'connected';
  app.friendRoomExpired = false;
  app.friendBotDifficulty = 'standard';
  app.friendMatchmaking = { status: 'searching', startedAt: Date.now() - 3000 };
  app.friendSeed = app.friendRoom.room_seed;
  app.friendStartedAt = Date.now() - 30000;
  app.friendPlayerSolved = 3;
  app.friendMatch = null;
  app.friendMatchContract = null;
  app.friendAttempts = [];
  app.friendRules = friendMatch.rules();
  app.friendCountdownActive = false;
  app.friendCountdownUntil = 0;
  app.dailyChallenge = dailyChallenge.build(require('../src/core/puzzle_generator.js'), '2026-08-14', 20260814);
  app.backendAuth = { status: 'offline', user: null, error: null };
  app.audio = {
    settings: () => ({ music_enabled: true, sfx_enabled: true, music_track: 0, music_volume: 0.42, sfx_volume: 0.72 }),
    getMusicTrackName: () => 'test-track',
    setMusicEnabled() {}, setSfxEnabled() {}, setMusicTrack() {},
    setMusicVolume() {}, setSfxVolume() {},
    playClick() {},
  };
  app.ads = { configure() {} };
  app.volumeDragAreas = {};
  app.diagnosticReport = () => ({
    device: { platform: 'test', screen: '375x667', pixelRatio: 2, brand: 'test', model: 'test', system: 'test' },
    storage: { primaryValid: true, backupValid: true, primaryVersion: 1, lastError: null },
    questions: { campaignVerified: 200, campaignTotal: 200, generatorReady: true },
    audio: { music: true, sfx: true, failed: false },
    backend: { status: 'offline', configured: false },
    runtime: { errorCount: 0, lastError: null },
  });
  app.pollBackendFriendRoom = () => {};
  app.pollFriendMatchProgress = () => {};
  app.pollBackendFriendRoom = () => {};
  app.refreshLeaderboard = () => {};
  app.loadRemoteLeaderboard = () => {};
  app.sharePayload = () => {};
  app.triggerFeedback = () => {};
  return app;
}

function isOverlay(app, button) {
  return button.key === 'popup-overlay' || button.key === 'result-help-overlay'
    || (button.x <= 0 && button.y <= 0 && button.width >= app.width && button.height >= app.height);
}

function overlaps(a, b) {
  return !(a.x + a.width <= b.x || b.x + b.width <= a.x
    || a.y + a.height <= b.y || b.y + b.height <= a.y);
}

function ignoreOverlap(a, b) {
  const keys = [String(a.key || ''), String(b.key || '')];
  return keys.some((key) => key.startsWith('modal-close'))
    || keys.some((key) => key.includes('overlay'));
}

function activeButtons(app) {
  const lastOverlay = app.buttons.reduce(
    (last, button, index) => (isOverlay(app, button) ? index : last), -1,
  );
  return lastOverlay >= 0 ? app.buttons.slice(lastOverlay + 1) : app.buttons;
}

function checkButtons(app, label) {
  assert.ok(app.buttons.length > 0, `${label}: 没有注册任何按钮`);
  const buttons = activeButtons(app);
  const keys = new Set();
  buttons.forEach((button) => {
    assert.ok(typeof button.action === 'function', `${label}: ${button.key || '?'} 没有事件函数`);
    assert.ok(Number.isFinite(button.x) && Number.isFinite(button.y), `${label}: ${button.key || '?'} 坐标非法`);
    assert.ok(button.width > 0 && button.height > 0, `${label}: ${button.key || '?'} 尺寸非法`);
    assert.ok(button.x >= -2 && button.x + button.width <= app.width + 2, `${label}: ${button.key || '?'} 横向越界`);
    assert.ok(button.y >= -2 && button.y + button.height <= app.visibleBottom(0) + 2, `${label}: ${button.key || '?'} 纵向越界`);
    assert.ok(typeof button.disabled === 'boolean', `${label}: ${button.key || '?'} disabled 状态非法`);
    assert.ok(button.key, `${label}: 存在没有唯一 key 的按钮`);
    assert.ok(!keys.has(button.key), `${label}: key 重复 ${button.key}`);
    keys.add(button.key);
    assert.ok(hitTest.isButtonHit(button, button.x + button.width / 2, button.y + button.height / 2, 0), `${label}: ${button.key} 中心点不可命中`);
    if (button.dragType) assert.ok(['music', 'sfx'].includes(button.dragType), `${label}: ${button.key} 滑块类型非法`);
  });
  for (let i = 0; i < buttons.length; i += 1) {
    for (let j = i + 1; j < buttons.length; j += 1) {
      if (!ignoreOverlap(buttons[i], buttons[j])) assert.ok(!overlaps(buttons[i], buttons[j]), `${label}: 热区重叠 ${buttons[i].key}/${buttons[j].key}`);
    }
  }
  return buttons;
}

function drawCase(app, label, setup, draw) {
  setup(app);
  app.buttons = [];
  draw.call(app);
  checkButtons(app, label);
}

function drawAllPages(width, height) {
  const app = appFor(width, height);
  drawCase(app, 'home', (a) => { a.screen = 'home'; a.popup = ''; }, function run() { this.drawHome(1); });
  ['chapter_info', 'tutorial', 'settings', 'diagnostics', 'tasks', 'more'].forEach((popup) => {
    drawCase(app, `popup:${popup}`, (a) => { a.screen = 'home'; a.popup = popup; a.tutorialStep = 1; }, function run() { this.drawHome(1); this.drawPopup(); });
  });
  drawCase(app, 'levels:first-page', (a) => { a.screen = 'levels'; a.popup = ''; a.menuPage = 0; }, function run() { this.drawLevels(); });
  drawCase(app, 'levels:last-page', (a) => { a.screen = 'levels'; a.popup = ''; a.menuPage = 9; }, function run() { this.drawLevels(); });
  ['campaign', 'daily', 'endless', 'friend'].forEach((mode) => {
    drawCase(app, `game:${mode}`, (a) => {
      a.screen = 'game'; a.popup = ''; a.mode = mode; a.cards = [{ value: 1 }, { value: 1 }, { value: 2 }, { value: 6 }];
      a.currentQuestion = 0; a.puzzles = mode === 'friend' ? Array.from({ length: 8 }, () => ({ rules: {} })) : [{ rules: {} }, { rules: {} }];
      a.currentPuzzle = a.puzzles[0];
    }, function run() { this.drawGame(); });
  });
  drawCase(app, 'result:campaign', (a) => {
    a.screen = 'result'; a.mode = 'campaign'; a.result = { passed: true, score: 100, stars: 3, combo: 5, mistakes: 0, reason: 'clear', rewardCoins: 15, bonusLabels: [], levelComplete: true, next: true };
  }, function run() { this.drawResult(); });
  drawCase(app, 'result:friend', (a) => {
    a.screen = 'result'; a.mode = 'friend'; a.result = { passed: false, score: 80, stars: 1, combo: 2, mistakes: 1, reason: 'timeout', rewardCoins: 0, bonusLabels: [], next: false, matchResult: { outcome: 'lose', player_solved: 3, player_score: 80, player_elapsed: 30, opponent_solved: 4, opponent_score: 100, opponent_elapsed: 28 } };
  }, function run() { this.drawResult(); });
  drawCase(app, 'result-help', (a) => { a.screen = 'result'; a.resultHelpPopup = true; }, function run() { this.drawResultHelpPopup(); });
  drawCase(app, 'hint-popup', (a) => { a.screen = 'game'; a.hintPopup = { first: 1, second: 6, firstIndex: 0, secondIndex: 3 }; }, function run() { this.drawHintPopup(); });
  drawCase(app, 'friend-entry', (a) => { a.screen = 'friend_lobby'; a.friendLobbyView = 'entry'; a.friendRoom = null; }, function run() { this.drawFriendLobby(); });
  drawCase(app, 'friend-room', (a) => { a.screen = 'friend_lobby'; a.friendLobbyView = 'room'; a.friendRoom = friendMatch.createRoom(20260814); }, function run() { this.drawFriendLobby(); });
  ['searching', 'bot_ready'].forEach((status) => {
    drawCase(app, `friend-matchmaking:${status}`, (a) => { a.screen = 'friend_matchmaking'; a.friendMatchmaking = { status, startedAt: Date.now() - 3000 }; }, function run() { this.drawFriendMatchmaking(); });
  });
  drawCase(app, 'shop', (a) => { a.screen = 'shop'; a.shopTab = 'themes'; a.shopPage = 0; }, function run() { this.drawShop(); });
  ['themes', 'cards', 'operators', 'effects'].forEach((tab) => {
    drawCase(app, `shop:${tab}:last-page`, (a) => { a.screen = 'shop'; a.shopTab = tab; a.shopPage = 999; }, function run() { this.drawShop(); });
  });
  drawCase(app, 'achievements', (a) => { a.screen = 'achievements'; a.achievementPage = 999; }, function run() { this.drawAchievements(); });
  ['global', 'friends'].forEach((board) => {
    drawCase(app, `leaderboard:${board}`, (a) => { a.screen = 'leaderboard'; a.leaderboardBoard = board; }, function run() { this.drawLeaderboard(); });
  });
  drawCase(app, 'records', (a) => { a.screen = 'records'; }, function run() { this.drawRecords(); });
}

function findKey(app, key) {
  const button = app.buttons.find((item) => item.key === key);
  assert.ok(button, `关键按钮不存在: ${key}`);
  assert.ok(typeof button.action === 'function', `关键按钮无事件: ${key}`);
  return button;
}

function runCriticalActions() {
  const app = appFor();
  app.showLevels = () => { app.screen = 'levels'; };
  app.showFriendLobby = () => { app.screen = 'friend_lobby'; };
  app.startEndless = () => { app.screen = 'game'; app.mode = 'endless'; };
  app.startDaily = () => { app.screen = 'game'; app.mode = 'daily'; };
  app.drawHome(1);
  findKey(app, 'campaign').action(); assert.strictEqual(app.screen, 'levels', '首页闯关按钮没有进入关卡页');
  app.screen = 'home'; app.drawHome(1); findKey(app, 'friend').action(); assert.strictEqual(app.screen, 'friend_lobby', '好友对战按钮没有进入好友页');
  app.screen = 'home'; app.drawHome(1); findKey(app, 'endless').action(); assert.strictEqual(app.mode, 'endless', '无尽模式按钮没有触发');
  app.screen = 'home'; app.drawHome(1); findKey(app, 'daily').action(); assert.strictEqual(app.mode, 'daily', '每日挑战按钮没有触发');

  let tapped = '';
  app.screen = 'game'; app.cards = []; app.popup = '';
  app.audio.playClick = () => {};
  app.buttons = [
    { key: 'game-undo', x: 44, y: 900, width: 212, height: 62, action: () => { tapped = 'undo'; }, disabled: false },
    { key: 'game-hint', x: 268, y: 900, width: 212, height: 62, action: () => { tapped = 'hint'; }, disabled: false },
  ];
  app.onTouch({ touches: [{ clientX: (44 + 106) * app.renderScale, clientY: (900 - 10) * app.renderScale }] });
  assert.strictEqual(tapped, 'undo', '撤销按钮上方的手机触摸缓冲没有生效');

  app.screen = 'home'; app.popup = 'settings'; app.drawHome(1); app.drawPopup();
  const musicSlider = findKey(app, 'settings-volume-music');
  assert.strictEqual(musicSlider.dragType, 'music', '背景音乐滑块未注册拖动类型');
  const sfxSlider = findKey(app, 'settings-volume-sfx');
  assert.strictEqual(sfxSlider.dragType, 'sfx', '按键音效滑块未注册拖动类型');
  console.log('CRITICAL_BUTTON_ACTIONS_OK');
}

for (const [width, height] of [[375, 667], [393, 852], [414, 896], [750, 1334]]) {
  drawAllPages(width, height);
  console.log(`BUTTONS_OK ${width}x${height}`);
}
runCriticalActions();
console.log('BUTTON_AUDIT_OK');
