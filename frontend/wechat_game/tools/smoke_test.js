/*
 * 微信端离线烟雾测试。
 * 运行：node tools/smoke_test.js
 * 这里不依赖 wx，专门验证题目、存档、奖励和模式数据契约。
 */
const assert = require('assert');
const fs = require('fs');
const path = require('path');
const puzzle = require('../src/core/puzzle_generator.js');
const levelCatalog = require('../src/core/level_catalog.js');
const campaignPuzzleData = require('../src/core/campaign_puzzle_data.js');
const dailyPuzzleData = require('../src/core/daily_puzzle_data.js');
const daily = require('../src/core/daily_challenge.js');
const { QuestionService } = require('../src/core/question_service.js');
const endless = require('../src/core/endless_mode.js');
const friend = require('../src/core/friend_match_service.js');
const matchData = require('../src/core/match_data.js');
const storage = require('../src/services/storage.js');
const tasks = require('../src/services/task_service.js');
const achievements = require('../src/services/achievement_service.js');
const skinCatalog = require('../src/core/skin_catalog.js');
const { AudioService } = require('../src/services/audio_service.js');
const { GameApp } = require('../src/app.js');
const apiClient = require('../src/services/api_client.js');

function check(condition, message) {
  assert.ok(condition, message);
}

function testLevels() {
  const levels = levelCatalog.all();
  check(levels.length === 200, '应生成 200 个闯关关卡');
  check(levels.every((level) => level.questionCount >= 1 && level.timeLimit > 0), '关卡配置必须有题目数和时间限制');
  const first = puzzle.generatePuzzleSet(levels[0], 0, levels[0].questionCount, 240000);
  const hard = puzzle.generatePuzzleSet(levels[199], 199, levels[199].questionCount, 240000 + 199 * 9973);
  check(first.length === levels[0].questionCount, '第 1 关题目生成失败');
  check(hard.length === levels[199].questionCount, '第 200 关题目生成失败');
  [...first, ...hard].forEach((record) => {
    check(record.target === 24, '题目目标必须是 24');
    check(record.numbers.length === 4, '每道题必须有 4 个数字');
    check(record.solution && record.firstStep, '题目必须带至少一种解法和第一步提示');
  });
}

function testCampaignPuzzleBank() {
  const levels = levelCatalog.all();
  const bank = puzzle.loadCampaignPuzzleBankFromData(campaignPuzzleData, levels, 240000);
  check(bank, '静态闯关题库读取失败');
  check(bank.total === bank.expected && bank.expected === 680, '闯关题库数量不完整');
  check(bank.missingCount === 0, '闯关题库存在缺题');
  check(bank.duplicateCount === 0 && bank.uniqueKeys === bank.total, '闯关题目存在数字组合重复');
  check(bank.verifiedCount === bank.total, '闯关题库存在未验证题目');
  const keys = new Set();
  bank.bank.forEach((records, levelIndex) => {
    check(records.length === levels[levelIndex].questionCount, `第 ${levelIndex + 1} 关题目数量不完整`);
    records.forEach((record) => {
      const key = record.numbers.join(',');
      check(!keys.has(key), `第 ${levelIndex + 1} 关存在重复数字组合`);
      keys.add(key);
      check(record.difficultyTier && puzzle.isVerifiedRecord(record), `第 ${levelIndex + 1} 关存在无难度标签或未验证题目`);
    });
  });
  const phaseAverage = (phase) => bank.phaseStats[String(phase)] && bank.phaseStats[String(phase)].average;
  check(phaseAverage(0) < phaseAverage(1), '进阶阶段难度没有高于简单阶段');
  check(phaseAverage(1) < phaseAverage(2), '困难阶段难度没有高于进阶阶段');
  check(phaseAverage(2) < phaseAverage(3), '大师阶段难度没有高于困难阶段');
}

function testPuzzleRotationAndVariety() {
  check(dailyPuzzleData && dailyPuzzleData.cycle_days === 365, 'daily puzzle cycle must contain 365 days');
  check(Array.isArray(dailyPuzzleData.days) && dailyPuzzleData.days.length === 365, 'daily puzzle data is incomplete');
  const dailyKeys = new Set();
  for (let offset = 0; offset < 365; offset += 1) {
    const date = new Date(Date.UTC(2026, 0, 1 + offset));
    const dateKey = date.toISOString().slice(0, 10);
    const seed = 20260000 + offset;
    const result = daily.build(puzzle, dateKey, seed);
    check(result && result.puzzles.length === daily.DAILY_QUESTION_COUNT, `daily schedule ${dateKey} is incomplete`);
    result.puzzles.forEach((record) => {
      const key = puzzle.numberKey(record.numbers);
      check(!dailyKeys.has(key), `daily schedule repeats puzzle ${key}`);
      dailyKeys.add(key);
      check(puzzle.isVerifiedRecord(record), `daily schedule contains an unsolved puzzle ${key}`);
      check(record.solutionSteps.length === 3, `daily schedule solution is incomplete ${key}`);
    });
  }
  check(dailyKeys.size === 365 * daily.DAILY_QUESTION_COUNT, 'daily schedule uniqueness check failed');

  const levels = levelCatalog.all();
  const bank = puzzle.loadCampaignPuzzleBankFromData(campaignPuzzleData, levels, 240000);
  const firstTwenty = bank.bank.slice(0, 20).flat();
  const operatorKinds = (record) => new Set(record.solutionSteps.map((step) => step.operator)).size;
  check(firstTwenty.every((record) => operatorKinds(record) >= 2), 'first 20 levels contain single-operator puzzles');
  check(firstTwenty.every((record) => !(record.solutionSteps.every((step) => step.operator === '+'))), 'first 20 levels contain pure addition puzzles');
  check(bank.phaseStats['0'].average < bank.phaseStats['1'].average, 'campaign phase 1 is not harder than phase 0');
  check(bank.phaseStats['1'].average < bank.phaseStats['2'].average, 'campaign phase 2 is not harder than phase 1');
  check(bank.phaseStats['2'].average < bank.phaseStats['3'].average, 'campaign phase 3 is not harder than phase 2');
}

function testQuestionService() {
  const levels = levelCatalog.all();
  const service = new QuestionService({ levels, campaignData: campaignPuzzleData });
  const campaignFirst = service.getCampaignQuestion(0, 0);
  const campaignSecond = service.getCampaignQuestion(0, 1);
  const campaignAgain = service.getCampaignQuestion(0, 0);
  check(service.isVerified(campaignFirst) && service.isVerified(campaignSecond), 'question service campaign result is invalid');
  check(puzzle.numberKey(campaignFirst.numbers) !== puzzle.numberKey(campaignSecond.numbers), 'question service campaign questions repeat');
  check(JSON.stringify(campaignFirst.numbers) === JSON.stringify(campaignAgain.numbers), 'question service campaign seed is not stable');

  const dailyResult = service.getDailyChallenge('2026-08-15', 20260815);
  const dailyAgain = service.getDailyChallenge('2026-08-15', 20260815);
  check(dailyResult && dailyResult.puzzles.length === 3, 'question service daily result is incomplete');
  check(JSON.stringify(dailyResult.puzzles.map((record) => record.numbers)) === JSON.stringify(dailyAgain.puzzles.map((record) => record.numbers)), 'question service daily seed is not stable');
  check(dailyResult.puzzles.every((record) => service.isVerified(record)), 'question service daily result is invalid');

  const endlessFirst = service.getEndlessQuestion(0, 987654);
  const endlessSecond = service.getEndlessQuestion(1, 987654);
  check(service.isVerified(endlessFirst) && service.isVerified(endlessSecond), 'question service endless result is invalid');
  check(puzzle.numberKey(endlessFirst.numbers) !== puzzle.numberKey(endlessSecond.numbers), 'question service endless questions repeat');

  const friendQuestions = service.getFriendQuestions(123456, { count: 8 });
  check(friendQuestions.length === 8 && friendQuestions.every((record) => service.isVerified(record)), 'question service friend result is invalid');
  check(new Set(friendQuestions.map((record) => puzzle.numberKey(record.numbers))).size === 8, 'question service friend questions repeat');

  const fallbackService = new QuestionService({ levels: [levels[0]], campaignData: { levels: [] } });
  const generated = fallbackService.getCampaignQuestion(0, 0, { config: levels[0] });
  check(generated && fallbackService.isVerified(generated), 'question service generated fallback is invalid');
  check(generated.generation && generated.generation.validated === true, 'question service result is missing validation metadata');
  check(service.getQuestion('campaign', { levelIndex: 0, questionIndex: 0 }), 'question service generic dispatcher failed');
}

function testCampaignStaticFlow() {
  const app = Object.create(GameApp.prototype);
  Object.assign(app, {
    levels: levelCatalog.all(),
    progress: storage.normalize({ unlocked_level: 0 }),
    campaignPuzzleBank: null,
    campaignPuzzleBankStats: null,
    mode: 'campaign', currentLevel: 0, hintPopup: null, resultHelpPopup: false,
    dailyChallenge: null, popup: '', status: '', screen: 'levels', puzzles: [],
  });
  app.isCampaignLevelUnlocked = () => true;
  app.beginSession = function beginSessionForTest() { this.screen = 'game'; };
  app.showLevels = function showLevelsForTest() { this.screen = 'levels'; };
  app.startCampaign(0);
  check(app.screen === 'game', '静态题库没有让闯关点击直接进入游戏');
  check(app.puzzles.length === app.levels[0].questionCount, '静态题库关卡题目数量错误');
  check(app.questionService instanceof QuestionService, '闯关没有接入统一题目服务');
  check(app.puzzles.every((record) => app.questionService.isVerified(record)), '统一题目服务返回了未验证闯关题目');
  check(!app.campaignPuzzleBuilder, '运行时不应启动全量题库构建器');

  const fallback = Object.create(GameApp.prototype);
  Object.assign(fallback, {
    levels: levelCatalog.all(),
    progress: storage.normalize({ unlocked_level: 0 }),
    campaignPuzzleBank: null,
    questionService: new QuestionService({ levels: levelCatalog.all(), campaignData: { levels: [] } }),
    mode: 'campaign', currentLevel: 0, hintPopup: null, resultHelpPopup: false,
    dailyChallenge: null, popup: '', status: '', screen: 'levels', puzzles: [],
  });
  fallback.isCampaignLevelUnlocked = () => true;
  fallback.beginSession = function beginSessionForFallbackTest() { this.screen = 'game'; };
  fallback.startCampaign(0);
  check(fallback.screen === 'game' && fallback.puzzles.length === fallback.levels[0].questionCount, '统一题目服务生成回退题目失败');
  check(fallback.puzzles.every((record) => fallback.questionService.isVerified(record)), '统一题目服务回退题目未通过验证');
}

function testDaily() {
  const result = daily.build(puzzle, '2026-08-14', 20260814);
  check(result && result.puzzles.length === daily.DAILY_QUESTION_COUNT, '每日挑战题目数量错误');
  check(result.puzzles.every((record) => record.solution && record.firstStep && record.solutionSteps.length === 3 && record.rules.allowNegativeIntermediate === false && puzzle.isVerifiedRecord(record)), '每日挑战存在未验证或不够合理的题目');
  check(result.puzzles.every((record) => puzzle.executeSteps(record.numbers, record.solutionSteps, record.rules)), '每日挑战存在无法执行的解法');
  for (let offset = 0; offset < daily.RULE_COUNT; offset += 1) {
    const seed = 20260808 + offset;
    const sample = daily.build(puzzle, String(seed), seed);
    check(sample.puzzles.length === daily.DAILY_QUESTION_COUNT, `每日规则 ${sample.rule_id || offset} 没有生成完整题目`);
    sample.puzzles.forEach((record) => {
      check(record.rules.allowNegativeIntermediate === false, `每日规则 ${sample.rule_id} 未关闭负数中间结果`);
      check(record.solutionSteps.length === 3 && puzzle.isVerifiedRecord(record), `每日规则 ${sample.rule_id} 存在不可执行题目`);
    });
  }
  const titleFormatter = Object.create(GameApp.prototype);
  check(titleFormatter.formatDailyRuleTitle('今日规则：快速出手') === '快速出手', '每日规则标题不应重复显示前缀');
  check(titleFormatter.formatDailyRuleTitle('快速出手') === '快速出手', '无前缀的每日规则标题不应被改写');
}

function testExecutableSolutions() {
  const duplicate = puzzle.makeVerifiedRecord([1, 1, 2, 6], 0, 0);
  check(duplicate.firstStep.firstIndex !== duplicate.firstStep.secondIndex, '重复数字提示没有保存卡片索引');
  check(puzzle.isVerifiedRecord(duplicate), '重复数字题目解法验证失败');
  const records = puzzle.generatePuzzleSet({ min_digit: 1, max_digit: 13, min_solutions: 1, max_solutions: 40 }, 8, 12, 20260814);
  check(records.length === 12 && records.every((record) => puzzle.isVerifiedRecord(record)), '批量题目存在不可执行解法');
}

function testGameInteraction() {
  const record = puzzle.makeVerifiedRecord([1, 2, 4, 5], 0, 0);
  const nextRecord = puzzle.makeVerifiedRecord([1, 1, 2, 6], 0, 1);
  const app = Object.create(GameApp.prototype);
  Object.assign(app, {
    screen: 'game', transitioning: false, autoNextAt: 0, autoNextToken: 0,
    mode: 'campaign', currentQuestion: 0, currentLevel: 0, puzzles: [record, nextRecord],
    levels: levelCatalog.all(), progress: storage.normalize({ tutorial_seen: true }),
    score: 0, combo: 0, mistakes: 0, maxCombo: 0, timeLeft: 30, timerLimit: 60,
    hintUsed: false, questionOperators: [], freeUndo: true, freeHint: true,
    undoStack: [], selectedIndex: -1, selectedOperator: '', status: '', hintPopup: null,
    audio: { playSuccess() {}, playError() {}, playMerge() {}, playCard() {}, playOperator() {}, playClick() {} },
    triggerFeedback() {},
  });
  app.setupPuzzle(record);
  for (const step of record.solutionSteps) {
    const locate = (indices) => app.cards.findIndex((card) => {
      const source = (card.sourceIndices || [card.id]).slice().sort((a, b) => a - b);
      const target = indices.slice().sort((a, b) => a - b);
      return source.length === target.length && source.every((value, index) => value === target[index]);
    });
    const firstIndex = locate(step.firstIndices);
    check(firstIndex >= 0, '真实点击流程找不到第一张卡片');
    app.selectCard(firstIndex);
    app.selectOperator(step.operator);
    const secondIndex = locate(step.secondIndices);
    check(secondIndex >= 0, '真实点击流程找不到第二张卡片');
    app.selectCard(secondIndex);
  }
  check(app.cards.length === 1 && app.cards[0].value === 24, '真实点击流程没有合成 24');
  check(app.transitioning === true && app.currentQuestion === 0, '第一题合成 24 后没有进入自动跳题状态');
  app.nextQuestion();
  check(app.screen === 'game' && app.currentQuestion === 1 && app.cards.length === 4, '第一题完成后没有进入第二题');
  app.setupPuzzle(nextRecord);
  for (const step of nextRecord.solutionSteps) {
    const locate = (indices) => app.cards.findIndex((card) => {
      const source = (card.sourceIndices || [card.id]).slice().sort((a, b) => a - b);
      const target = indices.slice().sort((a, b) => a - b);
      return source.length === target.length && source.every((value, index) => value === target[index]);
    });
    app.selectCard(locate(step.firstIndices));
    app.selectOperator(step.operator);
    app.selectCard(locate(step.secondIndices));
  }
  check(app.cards.length === 1 && app.cards[0].value === 24, '最后一题真实点击没有合成 24');
  check(app.screen === 'result', '最后一题合成 24 后没有进入结算页');
}

function makeGameHarness(mode, record, puzzles = [record, record]) {
  const app = Object.create(GameApp.prototype);
  Object.assign(app, {
    screen: 'game', transitioning: false, autoNextAt: 0, autoNextToken: 0,
    mode, currentQuestion: 0, currentLevel: 0, puzzles,
    levels: levelCatalog.all(), progress: storage.normalize({ tutorial_seen: true }),
    score: 0, combo: 0, mistakes: 0, maxCombo: 0, timeLeft: 60, timerLimit: 60,
    hintUsed: false, questionOperators: [], freeUndo: true, freeHint: true,
    undoStack: [], selectedIndex: -1, selectedOperator: '', status: '', hintPopup: null,
    resultHelpPopup: false, dailyChallenge: mode === 'daily' ? daily.build(puzzle, '2026-08-14', 20260814) : null,
    friendRoom: friend.createRoom(24681357), friendMatch: null, friendSeed: 24681357,
    friendStartedAt: Date.now(), friendPlayerSolved: 0,
     endlessRunId: 'smoke-run',
    audio: { playSuccess() {}, playError() {}, playMerge() {}, playCard() {}, playOperator() {}, playClick() {} },
    triggerFeedback() {},
  });
  app.currentPuzzle = record;
  app.setupPuzzle(record);
  return app;
}

function playSolution(app, record) {
  const locate = (indices) => app.cards.findIndex((card) => {
    const source = (card.sourceIndices || [card.id]).slice().sort((a, b) => a - b);
    const target = indices.slice().sort((a, b) => a - b);
    return source.length === target.length && source.every((value, index) => value === target[index]);
  });
  for (const step of record.solutionSteps) {
    app.selectCard(locate(step.firstIndices));
    app.selectOperator(step.operator);
    app.selectCard(locate(step.secondIndices));
  }
}

function testAllModeCompletion() {
  const record = puzzle.makeVerifiedRecord([1, 2, 4, 5], 0, 0);
  ['campaign', 'daily', 'endless', 'friend'].forEach((mode) => {
    const app = makeGameHarness(mode, record);
    playSolution(app, record);
    check(app.cards.length === 1 && app.cards[0].value === 24, `${mode} 没有合成 24`);
    check(app.transitioning === true, `${mode} 合成 24 后没有进入跳题状态`);
    check(app.screen === 'game', `${mode} 合成 24 后异常离开游戏页`);
  });
}

function testDailyQuestionServiceFlow() {
  const date = storage.todayKey();
  const app = Object.create(GameApp.prototype);
  Object.assign(app, {
    progress: storage.normalize({ tutorial_seen: true }),
    screen: 'home', dailyChallenge: null, status: '', mode: 'campaign',
    questionService: new QuestionService({ levels: levelCatalog.all(), campaignData: campaignPuzzleData }),
  });
  app.beginSession = function beginSessionForDailyTest() { this.screen = 'game'; };
  app.startDaily();
  check(app.screen === 'game', '每日挑战没有通过统一题目服务进入游戏');
  check(app.dailyChallenge && app.dailyChallenge.date_key === date, '每日挑战日期种子不稳定');
  check(app.puzzles.length === daily.DAILY_QUESTION_COUNT, '统一题目服务每日题目数量错误');
  check(app.puzzles.every((record) => app.questionService.isVerified(record)), '统一题目服务每日题目未通过验证');
  check(app.puzzles.every((record) => record.generation && record.generation.validated === true), '每日题目缺少统一验证标记');
  const second = app.questionService.getDailyChallenge(date, storage.todaySeed());
  check(JSON.stringify(app.puzzles.map((record) => record.numbers)) === JSON.stringify(second.puzzles.map((record) => record.numbers)), '同一天每日题目发生变化');
}

function testDailyCompletionLock() {
  const date = storage.todayKey();
  const app = Object.create(GameApp.prototype);
  Object.assign(app, {
    progress: storage.normalize({ daily: { completed: { [date]: true } } }),
    screen: 'home', dailyChallenge: null, status: '', mode: 'campaign',
  });
  app.startDaily();
  check(app.screen === 'home' && app.dailyChallenge === null, '每日挑战完成后仍然可以进入题目');
  check(String(app.status).includes('今日挑战已完成'), '每日挑战完成后没有显示已完成提示');

  const record = puzzle.makeVerifiedRecord([1, 2, 4, 5], 0, 0);
  const completed = makeGameHarness('daily', record, [record]);
  playSolution(completed, record);
  check(completed.screen === 'result' && storage.isDailyCompleted(completed.progress, date), '每日挑战完成后没有写入当天完成记录');
}

function testFastTouchSequence() {
  const record = puzzle.makeVerifiedRecord([3, 8, 1, 1], 0, 0);
  const app = makeGameHarness('campaign', record);
  app.width = 750;
  app.renderScale = 0.5;
  app.renderOffsetX = 0;
  app.renderOffsetY = 0;
  app.visibleHeight = 1334;
  app.height = 1334;
  app.safeTop = 24;
  app.safeBottom = 24;
  app.menuButton = { top: 42, bottom: 82 };
  app.buttons = [];
  app.lastTouch = 0;
  let selected = [];
  app.selectCard = (index) => { selected.push(index); };
  const layout = app.gameLayout();
  const startX = (app.width - layout.cardWidth * 2 - layout.gapX) / 2;
  const first = app.cardRect(0, startX, layout.cardStartY, layout.cardWidth, layout.cardHeight, layout.gapX, layout.gapY);
  const second = app.cardRect(1, startX, layout.cardStartY, layout.cardWidth, layout.cardHeight, layout.gapX, layout.gapY);
  const originalNow = Date.now;
  let clock = 1000;
  Date.now = () => clock;
  try {
    app.onTouch({ touches: [{ clientX: (first.x + first.width / 2) * 0.5, clientY: (first.y + first.height / 2) * 0.5 }] });
    clock += 20;
    app.onTouch({ touches: [{ clientX: (second.x + second.width / 2) * 0.5, clientY: (second.y + second.height / 2) * 0.5 }] });
  } finally {
    Date.now = originalNow;
  }
  check(selected.length === 2 && selected[0] === 0 && selected[1] === 1, '快速点击不同位置时第二次点击被误过滤');
}

function testRealTouchSolveChain() {
  const record = puzzle.makeVerifiedRecord([1, 2, 4, 5], 0, 0);
  const app = makeGameHarness('campaign', record);
  Object.assign(app, {
    width: 750,
    renderScale: 0.5,
    renderOffsetX: 0,
    renderOffsetY: 0,
    visibleHeight: 1334,
    height: 1334,
    safeTop: 24,
    safeBottom: 24,
    menuButton: { top: 42, bottom: 82 },
    lastTouch: 0,
    lastTouchPoint: null,
  });
  app.ctx = new Proxy({}, {
    get(target, property) {
      if (property === 'createLinearGradient' || property === 'createRadialGradient') return () => ({ addColorStop() {} });
      if (property === 'measureText') return (value) => ({ width: String(value || '').length * 8 });
      if (property in target) return target[property];
      return () => {};
    },
    set(target, property, value) { target[property] = value; return true; },
  });
  const originalNow = Date.now;
  let clock = 1000;
  Date.now = () => clock;
  const tap = (x, y) => {
    app.onTouch({ touches: [{ clientX: x * app.renderScale, clientY: y * app.renderScale }] });
    clock += 20;
    // 真机每次触摸后都会继续绘制，按钮热区也随之更新。
    if (app.screen === 'game' && !app.transitioning) app.drawGame();
  };
  const locate = (indices) => app.cards.findIndex((card) => {
    const source = (card.sourceIndices || [card.id]).slice().sort((a, b) => a - b);
    const target = indices.slice().sort((a, b) => a - b);
    return source.length === target.length && source.every((value, index) => value === target[index]);
  });
  try {
    app.drawGame();
    for (const step of record.solutionSteps) {
      const layout = app.gameLayout();
      const startX = (app.width - layout.cardWidth * 2 - layout.gapX) / 2;
      const firstIndex = locate(step.firstIndices);
      const firstRect = app.cardRect(firstIndex, startX, layout.cardStartY, layout.cardWidth, layout.cardHeight, layout.gapX, layout.gapY);
      tap(firstRect.x + firstRect.width / 2, firstRect.y + firstRect.height / 2);
      app.drawGame();
      const operatorButton = app.buttons.find((button) => button.key === `operator-${step.operator === '-' ? '−' : step.operator}`);
      check(operatorButton, `真实触摸找不到运算符 ${step.operator}`);
      tap(operatorButton.x + operatorButton.width / 2, operatorButton.y + operatorButton.height / 2);
      const secondIndex = locate(step.secondIndices);
      const secondRect = app.cardRect(secondIndex, startX, layout.cardStartY, layout.cardWidth, layout.cardHeight, layout.gapX, layout.gapY);
      tap(secondRect.x + secondRect.width / 2, secondRect.y + secondRect.height / 2);
      if (app.transitioning) break;
    }
  } finally {
    Date.now = originalNow;
  }
  check(app.cards.length === 1 && app.cards[0].value === 24, '真实手机坐标点击没有合成 24');
  check(app.transitioning === true, '真实手机坐标合成 24 后没有进入自动跳题');
}

function testGameCardWinsOverBackButtonForFallbackCoordinates() {
  const app = Object.create(GameApp.prototype);
  let backPressed = false;
  let selectedCard = -1;
  Object.assign(app, {
    screen: 'game', cards: [{ value: 1 }, { value: 2 }], buttons: [
      { x: 20, y: 40, width: 120, height: 60, key: 'header-back', action: () => { backPressed = true; } },
    ],
    hintPopup: null, resultHelpPopup: false, friendConnectionState: 'connected', friendRoomExpired: false,
    friendCountdownActive: false, transitioning: false, popup: '', volumeDragType: '',
    invokeTouchAction(action) { action(); return true; },
    touchPointCandidates() { return [{ x: 60, y: 60 }, { x: 300, y: 620 }]; },
    findGameCardAtPoint(x, y) { return x === 300 && y === 620 ? 0 : -1; },
    selectCard(index) { selectedCard = index; },
  });
  app.onTouch({ touches: [{ clientX: 60, clientY: 60 }] });
  check(selectedCard === 0, '手机备用坐标命中数字卡片时没有优先选中卡片');
  check(backPressed === false && app.screen === 'game', '数字卡片误触发了顶部返回按钮');
}

function testGameControlTouchPadding() {
  const app = Object.create(GameApp.prototype);
  let undoPressed = false;
  Object.assign(app, {
    screen: 'game', cards: [], buttons: [
      { x: 100, y: 200, width: 120, height: 60, key: 'game-undo', action: () => { undoPressed = true; } },
    ],
    hintPopup: null, resultHelpPopup: false, friendConnectionState: 'connected', friendRoomExpired: false,
    friendCountdownActive: false, transitioning: false, popup: '', volumeDragType: '',
    audio: { playClick() {} },
    invokeTouchAction(action) { action(); return true; },
    touchPointCandidates() { return [{ x: 160, y: 275 }]; },
    findGameCardAtPoint() { return -1; },
  });
  app.onTouch({ touches: [{ clientX: 160, clientY: 275 }] });
  check(undoPressed === true, '撤销按钮下沿的手机点击没有命中');
}

function testCardMergeAndInvalidTouchRecovery() {
  const record = puzzle.makeVerifiedRecord([3, 8, 1, 1], 0, 0);
  const app = makeGameHarness('campaign', record);
  const first = app.cards.findIndex((card) => card.value === 3);
  const second = app.cards.findIndex((card) => card.value === 8);
  check(first >= 0 && second >= 0, '截图回归题目的 3/8 卡片不存在');
  app.selectCard(first);
  app.selectOperator('×');
  app.selectCard(second);
  check(app.cards.length === 3 && app.cards.some((card) => card.value === 24), '3 × 8 没有合成出 24');
  check(app.selectedIndex === -1 && app.selectedOperator === '', '合成后仍残留选中状态');

  // 模拟真机点击到了过期卡片索引：不能抛异常，也不能把界面锁死。
  app.selectCard(0);
  app.selectOperator('+');
  app.applyOperation(99);
  check(app.selectedIndex === -1 && app.selectedOperator === '', '无效卡片点击后没有清理选中状态');
  check(app.screen === 'game' && app.transitioning === false, '无效卡片点击后错误离开游戏页');
}

function testRealTouchThreeTimesEight() {
  const record = puzzle.makeVerifiedRecord([3, 8, 1, 1], 0, 0);
  const app = makeGameHarness('campaign', record);
  Object.assign(app, {
    width: 750, renderScale: 0.5, renderOffsetX: 0, renderOffsetY: 0,
    visibleHeight: 1334, height: 1334, safeTop: 24, safeBottom: 24,
    menuButton: { top: 42, bottom: 82 }, lastTouch: 0, lastTouchPoint: null,
  });
  app.ctx = new Proxy({}, {
    get(target, property) {
      if (property === 'createLinearGradient' || property === 'createRadialGradient') return () => ({ addColorStop() {} });
      if (property === 'measureText') return (value) => ({ width: String(value || '').length * 8 });
      if (property in target) return target[property];
      return () => {};
    },
    set(target, property, value) { target[property] = value; return true; },
  });
  const originalNow = Date.now;
  Date.now = () => 1000;
  const tapButton = (button) => {
    check(button, '真实触摸回归找不到按钮');
    app.onTouch({ touches: [{ clientX: (button.x + button.width / 2) * app.renderScale, clientY: (button.y + button.height / 2) * app.renderScale }] });
    app.drawGame();
  };
  try {
    app.drawGame();
    const first = app.buttons.find((button) => button.key === 'game-card-0');
    tapButton(first);
    const operator = app.buttons.find((button) => button.key === 'operator-×');
    tapButton(operator);
    const second = app.buttons.find((button) => button.key === 'game-card-1');
    tapButton(second);
  } finally {
    Date.now = originalNow;
  }
  check(app.cards.length === 3 && app.cards.some((card) => card.value === 24), '真实触摸 3 × 8 没有完成合成');
  check(app.selectedIndex === -1 && app.selectedOperator === '', '真实触摸合成后状态没有清空');
}

function testVolumeSliders() {
  const values = { music: 0.42, sfx: 0.72 };
  const app = Object.create(GameApp.prototype);
  Object.assign(app, {
    screen: 'home', popup: 'settings', hintPopup: null, resultHelpPopup: false,
    transitioning: false, renderScale: 1, renderOffsetX: 0, renderOffsetY: 0,
    volumeDragType: '', volumeDragAreas: {
      music: { x: 100, y: 200, width: 300, height: 76, barX: 122, barWidth: 256 },
      sfx: { x: 100, y: 400, width: 300, height: 76, barX: 122, barWidth: 256 },
    },
    buttons: [
      { x: 100, y: 200, width: 300, height: 76, action() {}, dragType: 'music', key: 'settings-volume-music' },
      { x: 100, y: 400, width: 300, height: 76, action() {}, dragType: 'sfx', key: 'settings-volume-sfx' },
    ],
    progress: storage.normalize({ tutorial_seen: true }),
    audio: {
      settings: () => ({ music_enabled: true, sfx_enabled: true, music_track: 0, music_volume: values.music, sfx_volume: values.sfx }),
      setMusicVolume(value) { values.music = value; },
      setSfxVolume(value) { values.sfx = value; },
    },
    touchEffect: null,
  });
  app.onTouch({ touches: [{ clientX: 186, clientY: 238 }] });
  check(Math.abs(values.music - 0.25) < 0.001, '背景音乐音量点击没有更新');
  check(app.volumeDragType === 'music', '背景音乐音量没有进入拖动状态');
  app.onTouchMove({ touches: [{ clientX: 352.4, clientY: 236 }] });
  check(Math.abs(values.music - 0.9) < 0.001, '背景音乐音量拖动没有更新');
  app.onTouchEnd({});
  check(app.volumeDragType === '', '音量拖动结束后状态没有释放');

  app.onTouch({ touches: [{ clientX: 250, clientY: 438 }] });
  check(Math.abs(values.sfx - 0.5) < 0.001, '按键音效音量点击没有更新');
}

function testRestartModeRecovery() {
  const record = puzzle.makeVerifiedRecord([1, 2, 4, 5], 0, 0);
  const modeTargets = {
    campaign: 'startCampaign',
    daily: 'startDaily',
    endless: 'startEndless',
    friend: 'startFriend',
  };

  Object.keys(modeTargets).forEach((mode) => {
    const app = makeGameHarness(mode, record, [record, record]);
    const target = modeTargets[mode];
    let dispatched = 0;
    app[target] = () => { dispatched += 1; };
    Object.assign(app, {
      screen: 'game',
      transitioning: true,
      autoNextAt: Date.now() + 10000,
      autoNextToken: 12,
      renderScale: 1,
      renderOffsetX: 0,
      renderOffsetY: 0,
      hintPopup: null,
      resultHelpPopup: false,
      popup: '',
      result: { passed: true },
      selectedIndex: 1,
      selectedOperator: '+',
      undoStack: [[{ value: 1 }]],
      questionOperators: ['+'],
      audio: { playClick() {} },
      buttons: [{
        x: 100, y: 500, width: 240, height: 72,
        key: 'game-restart', disabled: false,
        action: () => app.restartMode(),
      }],
    });
    app.onTouch({ touches: [{ clientX: 220, clientY: 536 }] });
    check(dispatched === 1, `${mode} 过渡状态下点击重新开始没有分发到当前模式`);
    check(app.transitioning === false && app.autoNextAt === 0, `${mode} 重开后过渡状态没有清理`);
    check(app.selectedIndex === -1 && app.selectedOperator === '' && app.undoStack.length === 0, `${mode} 重开后操作状态没有清理`);
    check(app.result === null && app.hintPopup === null && app.resultHelpPopup === false, `${mode} 重开后弹窗或结算状态没有清理`);
  });

  // 闯关模式即使存档只记录到当前关，也允许重开已经进入的关卡。
  const campaign = makeGameHarness('campaign', record, [record]);
  campaign.currentLevel = 0;
  campaign.progress = storage.normalize({ unlocked_level: 0, tutorial_seen: true });
  campaign.screen = 'game';
  campaign.cards = [{ value: 99 }];
  campaign.transitioning = true;
  campaign.restartMode();
  check(campaign.screen === 'game' && campaign.cards.length === 4, '闯关重开没有恢复四张初始数字卡');
  check(campaign.selectedIndex === -1 && campaign.selectedOperator === '' && campaign.undoStack.length === 0, '闯关重开后仍残留撤销或选中状态');
}

function testSolvedStateWatchdog() {
  // 模拟真机丢失“最后一次点击数字”的情况：画面已经只剩 24，
  // 只能依靠主循环的最终状态检查完成通关。
  const record = puzzle.makeVerifiedRecord([1, 2, 4, 5], 0, 0);
  const nextRecord = puzzle.makeVerifiedRecord([1, 1, 2, 6], 0, 1);
  const app = Object.create(GameApp.prototype);
  Object.assign(app, {
    screen: 'game', transitioning: false, autoNextAt: 0, autoNextToken: 0,
    mode: 'campaign', currentQuestion: 0, currentLevel: 0, puzzles: [record, nextRecord],
    levels: levelCatalog.all(), progress: storage.normalize({ tutorial_seen: true }),
    score: 0, combo: 0, mistakes: 0, maxCombo: 0, timeLeft: 30, timerLimit: 60,
    hintUsed: false, questionOperators: ['+', '×', '-'], freeUndo: true, freeHint: true,
    undoStack: [], selectedIndex: 0, selectedOperator: '', status: '', hintPopup: null,
    cards: [{ value: 24, sourceIndices: [0, 1, 2, 3] }], currentPuzzle: record,
    audio: { playSuccess() {}, playError() {}, playMerge() {} },
    triggerFeedback() {},
  });
  check(app.checkSolvedState() === true, '只剩 24 时最终状态检查没有触发通关');
  check(app.transitioning === true && app.autoNextAt > Date.now(), '状态检查通关后没有设置自动跳题');
  app.nextQuestion();
  check(app.screen === 'game' && app.currentQuestion === 1 && app.cards.length === 4, '状态检查通关后没有进入下一题');
}

function testCompletionRecovery() {
  const record = puzzle.makeVerifiedRecord([1, 2, 4, 5], 0, 0);
  const app = makeGameHarness('campaign', record);
  app.transitioning = true;
  app.autoNextAt = Date.now() + 10000;
  app.cards = [{ value: 24, sourceIndices: [0, 1, 2, 3] }];
  app.renderScale = 1;
  app.renderOffsetX = 0;
  app.renderOffsetY = 0;
  app.buttons = [];
  app.onTouch({ touches: [{ clientX: 20, clientY: 20 }] });
  check(app.currentQuestion === 1 && app.screen === 'game' && app.cards.length === 4, '只剩 24 时再次点击没有恢复到下一题');

  const broken = makeGameHarness('campaign', record, [record, record]);
  broken.audio.playSuccess = () => { throw new Error('simulated success audio failure'); };
  const originalConsoleError = console.error;
  console.error = () => {};
  try { broken.finish(true, '完成本题'); } finally { console.error = originalConsoleError; }
  check(broken.screen === 'game' && broken.transitioning === true && broken.currentQuestion === 0, '非最后一题异常时错误进入了本局结算页');
  broken.nextQuestion();
  check(broken.screen === 'game' && broken.currentQuestion === 1 && broken.cards.length === 4, '非最后一题结算异常后没有进入下一题');

  const last = makeGameHarness('campaign', record, [record]);
  last.audio.playSuccess = () => { throw new Error('simulated final success audio failure'); };
  last.score = 711;
  last.maxCombo = 3;
  const secondConsoleError = console.error;
  console.error = () => {};
  try { last.finish(true, '完成本题'); } finally { console.error = secondConsoleError; }
  check(last.screen === 'result' && last.transitioning === false && last.result, '最后一题异常时没有进入整关结算页');
  check(last.result.stars === 3 && last.result.levelComplete === true && last.result.next === true, '结算异常兜底不应把满足条件的关卡降为 1 星或丢失下一关');
}

function testCompletionRender() {
  const record = puzzle.makeVerifiedRecord([1, 2, 4, 5], 0, 0);
  const makeContext = () => new Proxy({}, {
    get(target, property) {
      if (property === 'createLinearGradient' || property === 'createRadialGradient') return () => ({ addColorStop() {} });
      if (property === 'measureText') return (value) => ({ width: String(value || '').length * 8 });
      if (property in target) return target[property];
      return () => {};
    },
    set(target, property, value) { target[property] = value; return true; },
  });
  ['campaign', 'daily', 'endless', 'friend'].forEach((mode) => {
    const app = makeGameHarness(mode, record, [record]);
    Object.assign(app, {
      width: 750, height: 1334, visibleHeight: 1334, renderScale: 0.5,
      renderOffsetX: 0, renderOffsetY: 0, dpr: 2, safeTop: 24, safeBottom: 24,
      menuButton: { top: 42, bottom: 82 }, stars: [], floatNumbers: [], ctx: makeContext(),
      canvas: { width: 750, height: 1334 }, screen: 'result', renderRecovery: false,
      result: { passed: true, score: 100, stars: 3, combo: 1, mistakes: 0, rewardCoins: 0, bonusLabels: [], levelComplete: false, next: false, reason: '完成本题' },
      buttons: [],
    });
    app.draw(1);
    check(app.renderRecovery === false, `${mode} 结算页绘制发生异常`);
  });
}

function testStarsAndEndlessRun() {
  const record = puzzle.makeVerifiedRecord([1, 2, 4, 5], 0, 0);
  const app = makeGameHarness('campaign', record, [record]);
  const config = { targetScore: 100, targetCombo: 3 };
  Object.assign(app, { score: 59, mistakes: 0, maxCombo: 3, hintUsed: false });
  check(app.calculateCampaignStars(config).stars === 0, '低于 60 分时不应获得星级');
  Object.assign(app, { score: 60, mistakes: 2, maxCombo: 1, hintUsed: true });
  check(app.calculateCampaignStars(config).stars === 1, '达到 60 分时应获得 1 星');
  Object.assign(app, { score: 80, mistakes: 1, maxCombo: 1, hintUsed: true });
  check(app.calculateCampaignStars(config).stars === 2, '达到 80 分时应获得 2 星');
  Object.assign(app, { score: 100, mistakes: 0, maxCombo: 3, hintUsed: false });
  check(app.calculateCampaignStars(config).stars === 3, '达到 100 分时应获得 3 星');
  const gateProgress = { unlocked_level: 100, levels: {} };
  for (let index = 0; index < 100; index += 1) gateProgress.levels[String(index)] = { stars: 1, best_score: 60 };
  app.progress = storage.normalize(gateProgress);
  check(app.campaignBlockScore(0) === 6000, '前 100 关累计分计算错误');
  check(app.isCampaignLevelUnlocked(100), '达到 6000 分后没有解锁第 101 关');
  app.progress.levels['0'].best_score = 59;
  check(!app.isCampaignLevelUnlocked(100), '未达到 6000 分时错误解锁了第 101 关');

  const endlessApp = Object.create(GameApp.prototype);
  Object.assign(endlessApp, {
    renderRecovery: false, transitioning: false, autoNextAt: 0, autoNextToken: 0,
    currentQuestion: 0, endlessSeed: 123456, endlessUsedKeys: {},
    screen: 'home', score: 0, combo: 0, mistakes: 0, maxCombo: 0,
    freeUndo: true, freeHint: true, status: '',
  });
  const keys = new Set();
  for (let index = 0; index < 12; index += 1) {
    endlessApp.currentQuestion = index;
    endlessApp.beginEndlessQuestion();
    check(endlessApp.screen === 'game' && endlessApp.cards.length === 4, `无尽模式第 ${index + 1} 题没有正常生成`);
    const key = endlessApp.cards.map((card) => card.value).sort((a, b) => a - b).join(',');
    check(!keys.has(key), `无尽模式同一局出现重复题目：${key}`);
    keys.add(key);
  }
  check(endlessApp.questionService instanceof QuestionService, '无尽模式没有接入统一题目服务');
  check(endlessApp.currentPuzzle.generation && endlessApp.currentPuzzle.generation.validated === true, '无尽模式题目缺少统一验证标记');

  const otherRun = Object.create(GameApp.prototype);
  Object.assign(otherRun, { ...endlessApp, currentQuestion: 0, endlessSeed: 987654, endlessUsedKeys: {}, screen: 'home' });
  otherRun.beginEndlessQuestion();
  const otherKey = otherRun.cards.map((card) => card.value).sort((a, b) => a - b).join(',');
  check(otherKey !== [...keys][0], '无尽模式重新进入后首题仍然完全相同');
}

function testHintShowsOperator() {
  const record = puzzle.makeVerifiedRecord([1, 2, 4, 5], 0, 0);
  const app = makeGameHarness('campaign', record, [record]);
  app.hint();
  check(app.hintPopup && app.hintPopup.operator, '提示数据没有保存第一步运算符');
  const drawn = [];
  Object.assign(app, {
    width: 750, height: 1334, hintPopup: app.hintPopup,
    ctx: { save() {}, restore() {}, fillRect() {} },
    modalTop() { return 200; },
    drawModalFrame() {}, drawGamePanel() {},
    drawFitText(text) { drawn.push(String(text)); },
  });
  app.drawHintPopup();
  check(drawn.some((text) => text.includes(app.hintPopup.operator)), '提示弹窗没有显示第一步运算符');
}

function testTouchModalIsolation() {
  const app = Object.create(GameApp.prototype);
  Object.assign(app, {
    lastTouch: 0,
    renderScale: 1,
    renderOffsetX: 0,
    renderOffsetY: 0,
    hintPopup: { first: 1, second: 6 },
    resultHelpPopup: false,
    buttons: [],
    screen: 'game',
    transitioning: false,
    audio: { playClick() {} },
  });
  app.onTouch({ touches: [{ clientX: 10, clientY: 10 }] });
  check(app.hintPopup === null, '提示弹窗点击后没有关闭');

  let underlyingClicked = false;
  app.resultHelpPopup = true;
  app.buttons = [
    { x: 0, y: 0, width: 750, height: 80, action: () => { underlyingClicked = true; }, key: 'header-home' },
    { x: 0, y: 0, width: 750, height: 1334, action: () => { app.resultHelpPopup = false; }, key: 'result-help-overlay' },
    { x: 300, y: 500, width: 150, height: 60, action: () => { app.resultHelpPopup = false; }, key: 'result-help-ok' },
  ];
  app.lastTouch = Date.now() - 1000;
  app.onTouch({ touches: [{ clientX: 20, clientY: 20 }] });
  check(app.resultHelpPopup === false, '结算说明点击弹窗外没有关闭');
  check(underlyingClicked === false, '结算说明关闭时误触发了底层按钮');
}

function testLevelPageResume() {
  const app = Object.create(GameApp.prototype);
  Object.assign(app, {
    progress: storage.normalize({ unlocked_level: 21 }),
    popup: 'more',
    hintPopup: { first: 1 },
    menuPage: 0,
    screen: 'home',
  });
  app.showLevels();
  check(app.screen === 'levels' && app.menuPage === 1, '解锁到第 21 关后没有自动定位到第二页');
  check(app.popup === '' && app.hintPopup === null, '进入关卡页时没有清理旧弹窗状态');
}

function testSequentialCampaignUnlocks() {
  const app = Object.create(GameApp.prototype);
  app.progress = storage.normalize({
    unlocked_level: 4,
    levels: { '0': { completed: true, stars: 3, best_score: 100 } },
  });
  check(app.isCampaignLevelUnlocked(0), '首关不应被锁定');
  check(app.isCampaignLevelUnlocked(1), '完成第 1 关后第 2 关应解锁');
  check(!app.isCampaignLevelUnlocked(2), '未完成第 2 关却解锁了第 3 关');
  check(!app.isCampaignLevelUnlocked(3), '未完成前置关卡却解锁了第 4 关');
  check(app.highestPlayableLevelNumber() === 2, '连续解锁进度计算错误');

  app.progress.levels['1'] = { completed: true, stars: 0, best_score: 0 };
  check(app.isCampaignLevelUnlocked(2), '完成第 2 关后第 3 关没有解锁');
  check(!app.isCampaignLevelUnlocked(3), '未完成第 3 关却解锁了第 4 关');

  const legacy = storage.normalize({ unlocked_level: 4, levels: { '0': { stars: 2, best_score: 80 } } });
  check(legacy.levels['0'].completed === true, '旧版星级记录没有迁移为完成记录');
  const fresh = storage.normalize({ unlocked_level: 0 });
  storage.saveLevel(fresh, 0, 0, 0);
  check(fresh.levels['0'].completed === true, '低分通关没有记录 completed 标记');
}

function testLevelExpandContract() {
  const app = Object.create(GameApp.prototype);
  Object.assign(app, {
    width: 750, height: 1334, visibleHeight: 1334, renderScale: 1, safeBottom: 24,
    menuPage: 0, buttons: [], progress: storage.normalize({ tutorial_seen: true }),
    levels: levelCatalog.all(),
    ctx: { createLinearGradient() { return { addColorStop() {} }; } },
    screenContentTop: () => 100, pageTop: () => 42, visibleBottom: () => 1250,
    drawGameHeader() {}, drawGamePanel() {}, drawText() {}, drawFitText() {}, drawStarIcon() {},
    drawLockIcon() {}, drawNeonButton() {}, addHitArea(x, y, width, height, action, options = {}) { this.buttons.push({ x, y, width, height, action, key: options.key }); },
  });
  app.drawLevels();
  const expand = app.buttons.find((button) => button.key === 'chapter-expand');
  check(expand && typeof expand.action === 'function', '章节展开按钮没有绑定点击事件');
  expand.action();
  check(app.popup === 'chapter_info', '章节展开没有打开详情弹窗');
}

function testEndless() {
  const used = {};
  for (let index = 0; index < 18; index += 1) {
    const config = endless.configForQuestion(index);
    const record = endless.generateQuestion(puzzle, index, 20260814, used);
    check(record && record.numbers && record.solution, `无尽模式第 ${index + 1} 题生成失败`);
    check(record.endless_stage === config.stage, '无尽阶段编号不一致');
  }
  check(endless.nextMilestoneForQuestions(0) === 5, '无尽首个里程碑错误');
  check(endless.nextMilestoneForQuestions(5) === 10, '无尽下一里程碑错误');
  check(endless.statusForQuestion(4).question_in_stage === 2, '无尽阶段题目位置错误');
}

function testFriendQuestionServiceFlow() {
  const room = friend.createRoom(24681357);
  room.players.push({ id: 'friend-player', name: '测试好友', ready: true });
  room.status = 'ready';
  const app = Object.create(GameApp.prototype);
  Object.assign(app, {
    friendRoom: room,
    friendMatch: null,
    friendRoomBackendStatus: 'ready',
    backendAuth: { status: 'ready', user: { id: 0 } },
    friendStartedAt: 0,
    friendPlayerSolved: 0,
    progress: storage.normalize({ tutorial_seen: true }),
    screen: 'friend_lobby', mode: 'campaign', currentQuestion: 0,
    questionService: new QuestionService({ levels: levelCatalog.all(), campaignData: campaignPuzzleData }),
    status: '', dailyChallenge: null, hintPopup: null, resultHelpPopup: false,
  });
  app.sendFriendMatchProgress = function sendFriendProgressForTest() {};
  app.beginSession = function beginSessionForFriendTest() { this.screen = 'game'; };
  app.startFriend();
  check(app.screen === 'game', '好友对战没有通过统一题目服务进入游戏');
  check(app.puzzles.length === friend.QUESTION_COUNT, '统一题目服务好友题目数量错误');
  check(matchData.validateMatchContract(app.friendMatchContract), '好友对战没有生成有效的联机对局合同');
  check(app.puzzles.every((record) => app.questionService.isVerified(record)), '统一题目服务好友题目未通过验证');
  check(app.puzzles.every((record) => record.generation && record.generation.validated === true), '好友题目缺少统一验证标记');
  const sameRoom = app.questionService.getFriendQuestions(room.room_seed, { count: friend.QUESTION_COUNT });
  check(JSON.stringify(app.puzzles.map((record) => record.numbers)) === JSON.stringify(sameRoom.map((record) => record.numbers)), '同一房间种子生成了不同题目');
  check(new Set(app.puzzles.map((record) => puzzle.numberKey(record.numbers))).size === friend.QUESTION_COUNT, '好友对战题目存在重复');
}

function testFriendProtocol() {
  const room = friend.createRoom(13579246);
  const service = new QuestionService({ generator: puzzle });
  const puzzles = service.getFriendQuestions(room.room_seed, { count: friend.QUESTION_COUNT });
  const roomContract = matchData.createRoomContract(room);
  const matchContract = matchData.createMatchContract(room, puzzles);
  check(matchData.validateRoomContract(roomContract), '好友房间合同校验失败');
  check(matchData.validateMatchContract(matchContract), '好友对局合同校验失败');
  const attempts = puzzles.map((record, index) => matchData.createAttemptRecord(matchContract, record, {
    question_index: index,
    elapsed_ms: (index + 1) * 1000,
    solved: true,
    mistakes: 0,
    score: (index + 1) * 100,
    score_delta: 100,
  }));
  const submission = matchData.createResultSubmission(matchContract, attempts, { player_solved: 8, player_score: 800 });
  check(matchData.validateResultSubmission(submission), '好友成绩提交合同校验失败');
  const tampered = JSON.parse(JSON.stringify(submission));
  tampered.attempts[0].room_seed += 1;
  check(!matchData.validateResultSubmission(tampered), '篡改房间种子后仍通过成绩校验');
  const duplicate = JSON.parse(JSON.stringify(submission));
  duplicate.attempts.push(duplicate.attempts[0]);
  check(!matchData.validateResultSubmission(duplicate), '重复提交同一道题后仍通过成绩校验');
  const request = matchData.buildCloudRequest(matchData.CLOUD_FUNCTIONS.submitMatch, submission);
  check(request.protocol_version === matchData.PROTOCOL_VERSION && request.client_authoritative === false, '云函数请求合同字段错误');
}

function testFriend() {
  const room = friend.createRoom(24681357);
  const joined = friend.joinRoom(room);
  const puzzles = friend.generatePuzzles(puzzle, room.room_seed);
  const match = friend.createMatch(joined, puzzles);
  check(puzzles.length === friend.QUESTION_COUNT, '好友对战题目数量错误');
  check(match.rules.use_same_seed, '好友对战必须使用同题规则');
  const opponent = { solved: 3, score: 500, elapsed: 30, finished: true };
  const result = friend.calculateResult(2, 500, 0, 30, opponent);
  check(['win', 'lose', 'draw'].includes(result.outcome), '好友对战结果非法');
}

function testRewardsAndShop() {
  const progress = storage.normalize({ coins: 999998 });
  storage.addCoins(progress, 100);
  check(progress.coins === storage.COIN_CAP, '金币上限未生效');
  check(storage.spendCoins(progress, storage.COIN_CAP + 1) === false, '余额不足时不应扣款');
  check(storage.spendCoins(progress, 25) === true && progress.coins === storage.COIN_CAP - 25, '金币扣款错误');
  const endlessProgress = storage.normalize({ coins: 0 });
  const firstReward = storage.claimEndlessReward(endlessProgress, 'run-a', 9, '2026-08-14');
  const repeatReward = storage.claimEndlessReward(endlessProgress, 'run-a', 9, '2026-08-14');
  check(firstReward > 9 && repeatReward === 0, '无尽奖励应逐题结算且不可重复领取');
  const task = tasks.record(progress, 'campaign_clear', 1, '2026-08-14');
  check(task.reward === 15, '每日任务奖励错误');
  const achievement = achievements.unlock(progress, 'first_clear');
  check(achievement.reward === 30, '成就奖励错误');
  const skin = skinCatalog.all().find((item) => item.id !== 'classic' && item.price > 0);
  if (skin) {
    progress.coins = skin.price - 1;
    check(storage.spendCoins(progress, skin.price) === false, '皮肤余额不足校验错误');
  }
  check(skinCatalog.all().length >= 8, '商城主题数量不足');
  const firstShopSkinProgress = storage.normalize({ coins: 360, unlocked_level: 4 });
  check(skinCatalog.unlockStatus('ocean', firstShopSkinProgress).unlocked, '首套商城主题不应被额外关卡条件锁定');
  const loginProgress = storage.normalize({ coins: 0 });
  const loginReward = storage.claimDailyLoginReward(loginProgress, '2026-08-14');
  check(loginReward >= 5 && loginProgress.coins === loginReward, '每日登录奖励未正确发放');
  check(storage.claimDailyLoginReward(loginProgress, '2026-08-14') === 0, '每日登录奖励不应重复领取');
}

function testSkinCardTextContrast() {
  const skins = skinCatalog.all();
  check(skins.length >= 8, '主题数量不足');
  skins.forEach((skin) => {
    const cardText = String(skin.theme && skin.theme.card_text || '').toLowerCase();
    check(cardText !== '#fff' && cardText !== '#ffffff' && cardText !== 'white', `主题 ${skin.id} 的数字卡片文字过浅`);
  });
  check(skinCatalog.getSkin('classic').theme.card_text === '#17163e', '默认主题数字颜色不是深色');
}

function testCosmeticsPreviewAndStars() {
  const cosmetics = skinCatalog.allCosmetics();
  check(cosmetics.length >= 9, '外观商品数量不足');
  check(cosmetics.every((item) => item.id && item.category && item.preview), '外观商品数据不完整');
  const progress = storage.normalize({ coins: 1000, unlocked_level: 20 });
  check(skinCatalog.cosmeticUnlockStatus('card_neon', progress).unlocked, '霓虹卡片不应被错误锁定');
  const app = Object.create(GameApp.prototype);
  Object.assign(app, {
    progress, previewSkinId: '', previewSkinUntil: 0, previewSkinPrevious: '', shopNotice: '',
    triggerFeedback() {},
  });
  app.startSkinPreview(skinCatalog.getSkin('ocean'));
  check(app.activeSkinId() === 'ocean', '主题试用没有立即生效');
  app.clearSkinPreview(false);
  check(app.activeSkinId() === 'classic', '主题试用结束后没有恢复原主题');
  Object.assign(app, { score: 59, maxCombo: 2, mistakes: 2, hintUsed: true, timerLimit: 60, timeLeft: 20 });
  const starResult = app.calculateCampaignStars({ targetScore: 100, targetCombo: 5 });
  check(starResult.stars === 0 && starResult.starDetails.some((item) => !item.met), '星级失败原因没有被记录');
  const audio = new AudioService(storage.normalize({}).audio);
  audio.playClick(); audio.playSuccess(); audio.updateCountdown(2);
  check(audio.settings().music_enabled === true, '无资源音频没有安全初始化');
}

function testAudioResources() {
  const audio = require('../src/services/audio_service.js');
  const required = [
    ...audio.TRACK_SOURCES,
    ...Object.values(audio.SFX_SOURCES),
  ];
  required.forEach((source) => {
    check(source && fs.existsSync(path.resolve(__dirname, '..', source)), `audio resource missing: ${source}`);
  });
  check(audio.TRACK_SOURCES.length === 2, 'background music track count is incorrect');
  check(audio.SFX_SOURCES.countdownTick && audio.SFX_SOURCES.countdownUrgent, 'countdown audio phases are incomplete');
}

function testFrontendHardening() {
  check(typeof apiClient.isConfigured === 'function', 'backend configuration switch is missing');
  const corruptedObject = storage.normalize('{not-json');
  check(corruptedObject && corruptedObject.coins === 0 && corruptedObject.unlocked_level === 0, 'corrupted save data did not fall back safely');
  const stringSave = storage.normalize(JSON.stringify({ coins: 18, unlocked_level: 3 }));
  check(stringSave.coins === 18 && stringSave.unlocked_level === 3, 'string-form save data was not migrated safely');

  const app = Object.create(GameApp.prototype);
  Object.assign(app, { feedback: null, status: '', lastRuntimeError: null });
  const previousConsoleError = console.error;
  console.error = () => {};
  let invoked;
  try {
    invoked = app.invokeTouchAction(() => { throw new Error('touch-test'); }, 'hardening-test');
  } finally {
    console.error = previousConsoleError;
  }
  check(invoked === false && app.feedback && app.feedback.type === 'error', 'touch exception did not become a visible feedback state');

  const lifecycle = Object.create(GameApp.prototype);
  Object.assign(lifecycle, {
    gamePaused: false,
    backgroundPausedAt: 0,
    progress: storage.normalize({ coins: 2 }),
    audio: { pause() {}, resume() {} },
    screen: 'home',
    lastFrame: 0,
    dateKey: storage.todayKey(),
  });
  lifecycle.pauseForBackground();
  check(lifecycle.gamePaused === true && lifecycle.backgroundPausedAt > 0, 'background pause state was not recorded');
  lifecycle.resumeFromBackground();
  check(lifecycle.gamePaused === false && lifecycle.backgroundPausedAt === 0, 'foreground resume state was not restored');
}

function main() {
  testLevels();
  testCampaignPuzzleBank();
  testPuzzleRotationAndVariety();
  testQuestionService();
  testCampaignStaticFlow();
  testDaily();
  testExecutableSolutions();
  testGameInteraction();
  testAllModeCompletion();
  testDailyQuestionServiceFlow();
  testDailyCompletionLock();
  testFastTouchSequence();
  testRealTouchSolveChain();
  testGameCardWinsOverBackButtonForFallbackCoordinates();
  testGameControlTouchPadding();
  testCardMergeAndInvalidTouchRecovery();
  testRealTouchThreeTimesEight();
  testVolumeSliders();
  testRestartModeRecovery();
  testSolvedStateWatchdog();
  testCompletionRecovery();
  testCompletionRender();
  testStarsAndEndlessRun();
  testHintShowsOperator();
  testTouchModalIsolation();
  testLevelPageResume();
  testSequentialCampaignUnlocks();
  testLevelExpandContract();
  testEndless();
  testFriendQuestionServiceFlow();
  testFriendProtocol();
  testFriend();
  testRewardsAndShop();
  testSkinCardTextContrast();
  testCosmeticsPreviewAndStars();
  testAudioResources();
  testFrontendHardening();
  console.log('SMOKE_TEST_OK');
}

main();
