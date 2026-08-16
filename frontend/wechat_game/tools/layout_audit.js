/*
 * 微信端手机布局审计。
 * 运行：node tools/layout_audit.js
 * 只检查逻辑坐标，不需要启动微信开发者工具。
 */
const assert = require('assert');
const { GameApp } = require('../src/app.js');
const storage = require('../src/services/storage.js');
const levelCatalog = require('../src/core/level_catalog.js');
const dailyChallenge = require('../src/core/daily_challenge.js');
const friendMatch = require('../src/core/friend_match_service.js');
const leaderboardService = require('../src/services/leaderboard_service.js');

function fakeGradient() {
  return { addColorStop() {} };
}

function fakeContext() {
  const state = {};
  return new Proxy(state, {
    get(target, prop) {
      if (prop === 'createLinearGradient' || prop === 'createRadialGradient') return fakeGradient;
      if (prop === 'measureText') {
        return (value) => {
          const text = String(value || '');
          const wide = (text.match(/[\u4e00-\u9fff]/g) || []).length;
          return { width: text.length * 8 + wide * 8 };
        };
      }
      if (prop === 'canvas') return { width: 750, height: 1334 };
      if (prop in target) return target[prop];
      return () => {};
    },
    set(target, prop, value) {
      target[prop] = value;
      return true;
    },
  });
}

function appFor(width, height) {
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
  app.renderOffsetY = 0;
  app.renderOffsetX = 0;
  app.dpr = 1;
  app.ctx = fakeContext();
  // 以真机常见的微信右上角胶囊区域作为标题栏安全区输入。
  app.menuButton = { top: 42, bottom: 82 };
  app.cards = [{ value: 1 }, { value: 1 }, { value: 2 }, { value: 6 }];
  app.levels = levelCatalog.all();
  app.progress = storage.normalize({
    tutorial_seen: true,
    coins: 999999,
    unlocked_level: 58,
    last_level: 58,
    owned_skins: ['classic', 'ocean'],
    equipped_skin: 'classic',
    player_stats: { total_solved: 123456, total_score: 9876543, best_combo: 99, fastest_ms: 1234 },
    endless: { best_questions: 88 },
    friend_matches: { wins: 12, played: 34 },
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
  app.originalCards = app.cards.slice();
  app.selectedIndex = -1;
  app.selectedOperator = '';
  app.freeUndo = true;
  app.freeHint = true;
  app.timeLeft = 65;
  app.timerLimit = 90;
  app.score = 12345;
  app.combo = 12;
  app.mistakes = 1;
  app.maxCombo = 12;
  app.status = '';
  app.shopPage = 0;
  app.shopNotice = '兑换成功，已装备「星空玻璃」';
  app.achievementPage = 0;
  app.leaderboardBoard = leaderboardService.BOARD_GLOBAL;
  app.leaderboardMode = leaderboardService.MODE_CAMPAIGN;
  app.leaderboardRemote = {};
  app.leaderboardRemoteLoading = {};
  app.leaderboardRemoteFailedAt = {};
  app.friendRoom = friendMatch.createRoom(20260814);
  app.friendSeed = app.friendRoom.room_seed;
  app.friendStartedAt = Date.now() - 30000;
  app.friendPlayerSolved = 3;
  app.dailyChallenge = dailyChallenge.build(require('../src/core/puzzle_generator.js'), '2026-08-14', 20260814);
  app.audio = {
    settings: () => ({ music_enabled: true, sfx_enabled: true, music_track: 0, music_volume: 0.42, sfx_volume: 0.72 }),
    getMusicTrackName: () => '星空节拍',
    setMusicEnabled() {},
    setSfxEnabled() {},
    setMusicTrack() {},
  };
  return app;
}

function checkGameLayout(app, mode, cardCount) {
  app.mode = mode;
  app.cards = Array.from({ length: cardCount }, (_, index) => index + 1);
  const layout = app.gameLayout();
  const headerBottom = layout.headerY + 58;
  assert.ok(layout.statsY >= headerBottom, `${mode}: stats 与标题栏重叠`);
  assert.ok(layout.infoY >= layout.statsY + 84, `${mode}: 信息条与统计卡重叠`);
  assert.ok(layout.cardStartY >= layout.contentY + 226, `${mode}: 数字卡片压到题目面板`);
  assert.ok(layout.opTitleY >= layout.cardStartY + layout.cardRows * layout.cardHeight + layout.gapY, `${mode}: 运算区压到数字卡片`);
  assert.ok(layout.actionY >= layout.opTitleY + layout.operatorHeight + 18, `${mode}: 操作按钮压到运算符`);
  assert.ok(layout.bottomY >= layout.actionY + 22 + layout.actionHeight, `${mode}: 底部按钮压到操作按钮`);
  assert.ok(layout.footerY + layout.footerHeight <= app.visibleBottom(18), `${mode}: 底部信息条超出安全区`);
}

function checkPageTop(app) {
  const headerBottom = app.pageTop() + 58;
  ['levels', 'result', 'friend_lobby', 'shop', 'achievements', 'leaderboard', 'records'].forEach((page) => {
    const gap = page === 'records' ? 102 : page === 'result' ? 86 : 92;
    assert.ok(app.screenContentTop(gap) >= headerBottom, `${page}: 内容压到标题栏`);
  });
  [468, 560, 576, 700].forEach((height) => {
    const top = app.modalTop(height);
    assert.ok(top >= headerBottom, `弹窗 ${height}: 压到标题栏`);
    assert.ok(top + height <= app.visibleBottom(18), `弹窗 ${height}: 超出底部安全区`);
  });
}

function checkHomeButtonsRespectVisibleArea(app) {
  app.buttons = [];
  app.screen = 'home';
  app.popup = '';
  app.drawHome(1);
  const bottom = app.visibleBottom(0);
  app.buttons.forEach((button) => {
    assert.ok(button.y + button.height <= bottom + 2, `home: ${button.key || 'button'} 超出底部安全区`);
  });
}

function checkAutoNextState() {
  const app = Object.create(GameApp.prototype);
  Object.assign(app, {
    screen: 'game', transitioning: false, autoNextAt: 0, autoNextToken: 0,
    mode: 'campaign', currentQuestion: 0, currentLevel: 0, puzzles: [{}, {}],
    levels: levelCatalog.all(), score: 0, combo: 0, mistakes: 0, maxCombo: 0,
    timeLeft: 30, timerLimit: 60, hintUsed: false, questionOperators: [],
    progress: storage.normalize({}), audio: { playSuccess() {}, playError() {} },
    triggerFeedback() {},
  });
  app.finish(true, '测试答对');
  assert.ok(app.transitioning, '答对后没有进入下一题过渡状态');
  assert.ok(app.autoNextAt > Date.now(), '答对后没有设置自动跳题时间');
  // 取消测试定时器，避免异步回调影响其它审计。
  app.transitioning = false;
  app.autoNextToken += 1;
  app.autoNextAt = 0;
}

function intersects(a, b) {
  return !(a.x + a.width <= b.x || b.x + b.width <= a.x || a.y + a.height <= b.y || b.y + b.height <= a.y);
}

function shouldIgnoreOverlap(a, b) {
  const keys = [a.key || '', b.key || ''];
  if (keys.some((key) => key.includes('overlay'))) return true;
  if (keys.some((key) => key.startsWith('modal-close'))) return true;
  return false;
}

function isFullScreenIntercept(app, button) {
  return button.x <= 0 && button.y <= 0 && button.width >= app.width && button.height >= app.height;
}

function checkButtonRects(app, label) {
  const bottom = app.visibleBottom(0);
  const lastInterceptIndex = app.buttons.reduce((found, button, index) => (isFullScreenIntercept(app, button) ? index : found), -1);
  const activeButtons = lastInterceptIndex >= 0 ? app.buttons.slice(lastInterceptIndex + 1) : app.buttons;
  activeButtons.forEach((button) => {
    if (isFullScreenIntercept(app, button)) return;
    assert.ok(Number.isFinite(button.x) && Number.isFinite(button.y), `${label}: 按钮坐标非法 ${button.key}`);
    assert.ok(button.width > 0 && button.height > 0, `${label}: 按钮尺寸非法 ${button.key}`);
    assert.ok(button.x >= -2 && button.x + button.width <= app.width + 2, `${label}: 按钮横向越界 ${button.key}`);
    assert.ok(button.y >= -2 && button.y + button.height <= bottom + 2, `${label}: 按钮纵向越界 ${button.key}`);
  });
  for (let i = 0; i < activeButtons.length; i += 1) {
    for (let j = i + 1; j < activeButtons.length; j += 1) {
      const a = activeButtons[i];
      const b = activeButtons[j];
      if (isFullScreenIntercept(app, a) || isFullScreenIntercept(app, b)) continue;
      if (shouldIgnoreOverlap(a, b)) continue;
      assert.ok(!intersects(a, b), `${label}: 按钮热区重叠 ${a.key || '?'} / ${b.key || '?'}`);
    }
  }
}

function drawAndCheck(app, label, setup, draw) {
  app.buttons = [];
  setup(app);
  draw.call(app);
  checkButtonRects(app, label);
}

function checkDrawablePages(app) {
  drawAndCheck(app, 'home', (target) => { target.screen = 'home'; target.popup = ''; }, function run() { this.drawHome(1); });
  ['chapter_info', 'tutorial', 'settings', 'diagnostics', 'tasks', 'more'].forEach((popup) => {
    drawAndCheck(app, `popup:${popup}`, (target) => { target.screen = 'home'; target.popup = popup; target.tutorialStep = 1; }, function run() { this.drawHome(1); this.drawPopup(); });
  });
  drawAndCheck(app, 'levels', (target) => { target.screen = 'levels'; target.popup = ''; target.menuPage = 2; }, function run() { this.drawLevels(); });
  ['campaign', 'daily', 'endless', 'friend'].forEach((mode) => {
    drawAndCheck(app, `game:${mode}`, (target) => {
      target.screen = 'game';
      target.popup = '';
      target.mode = mode;
      target.cards = [{ value: 1 }, { value: 1 }, { value: 2 }, { value: 6 }];
      target.currentLevel = 0;
      target.currentQuestion = 0;
      target.puzzles = mode === 'friend' ? Array.from({ length: friendMatch.QUESTION_COUNT }, () => ({ rules: {} })) : [{ rules: {} }, { rules: {} }, { rules: {} }];
      target.currentPuzzle = target.puzzles[0];
    }, function run() { this.drawGame(); });
  });
  drawAndCheck(app, 'result:campaign', (target) => {
    target.screen = 'result';
    target.mode = 'campaign';
    target.result = { passed: true, score: 123456789, stars: 3, combo: 12, mistakes: 0, reason: '完成本关', rewardCoins: 9999, bonusLabels: ['新成就 完美解题 +80', '完成 1 个闯关关卡 +15', '刷新个人最高分'], levelComplete: true, next: true };
  }, function run() { this.drawResult(); });
  drawAndCheck(app, 'result:friend', (target) => {
    target.screen = 'result';
    target.mode = 'friend';
    target.result = {
      passed: false,
      score: 88888,
      stars: 1,
      combo: 4,
      mistakes: 3,
      reason: '答错扣时后时间用尽',
      rewardCoins: 0,
      bonusLabels: ['今日对战奖励已达上限'],
      next: false,
      matchResult: { outcome: 'lose', player_solved: 3, player_score: 88888, player_elapsed: 119.4, opponent_solved: 5, opponent_score: 99999, opponent_elapsed: 108.2 },
    };
  }, function run() { this.drawResult(); });
  drawAndCheck(app, 'result-help', (target) => { target.screen = 'result'; target.resultHelpPopup = true; }, function run() { this.drawResultHelpPopup(); this.resultHelpPopup = false; });
  drawAndCheck(app, 'friend_lobby', (target) => { target.screen = 'friend_lobby'; }, function run() { this.drawFriendLobby(); });
  drawAndCheck(app, 'shop', (target) => { target.screen = 'shop'; target.shopPage = 0; }, function run() { this.drawShop(); });
  ['themes', 'cards', 'operators', 'effects'].forEach((tab) => {
    drawAndCheck(app, `shop:${tab}`, (target) => { target.screen = 'shop'; target.shopTab = tab; target.shopPage = 0; }, function run() { this.drawShop(); });
  });
  drawAndCheck(app, 'achievements', (target) => { target.screen = 'achievements'; target.achievementPage = 0; }, function run() { this.drawAchievements(); });
  drawAndCheck(app, 'leaderboard', (target) => { target.screen = 'leaderboard'; }, function run() { this.drawLeaderboard(); });
  drawAndCheck(app, 'records', (target) => { target.screen = 'records'; }, function run() { this.drawRecords(); });
  drawAndCheck(app, 'hint-popup', (target) => { target.screen = 'game'; target.hintPopup = { first: 1, second: 6, firstIndex: 0, secondIndex: 3 }; }, function run() { this.drawHintPopup(); this.hintPopup = null; });
}

for (const [width, height] of [[375, 667], [393, 852], [414, 896], [750, 1334]]) {
  const app = appFor(width, height);
  checkPageTop(app);
  checkHomeButtonsRespectVisibleArea(app);
  ['campaign', 'daily', 'endless', 'friend'].forEach((mode) => {
    checkGameLayout(app, mode, 4);
    checkGameLayout(app, mode, 1);
  });
  checkDrawablePages(app);
  console.log(`LAYOUT_OK ${width}x${height}`);
}
checkAutoNextState();
console.log('AUTO_NEXT_OK');
