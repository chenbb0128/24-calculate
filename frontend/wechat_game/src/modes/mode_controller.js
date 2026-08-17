/* Mode orchestration methods extracted from the legacy controller. */

const puzzle = require('../core/puzzle_generator.js');
const dailyChallenge = require('../core/daily_challenge.js');
const friendMatch = require('../core/friend_match_service.js');
const matchData = require('../core/match_data.js');
const storage = require('../services/storage.js');
const rankService = require('../services/rank_service.js');
const apiClient = require('../services/api_client.js');
const endlessMode = require('../core/endless_mode.js');
const { safeNumber } = require('../app/app_utils.js');

function makeEndlessSeed() {
  const now = Date.now() >>> 0;
  const random = Math.floor(Math.random() * 0x100000000) >>> 0;
  const day = storage.todaySeed() >>> 0;
  return (now ^ random ^ ((day * 2654435761) >>> 0)) >>> 0 || 1;
}

// These are only bootstrap candidates. They are already known to have an
// integer solution, so entering endless mode never has to wait for the full
// random solver before the first frame can be shown.
const FAST_ENDLESS_NUMBERS = [
  [1, 2, 4, 5],
  [3, 3, 4, 6],
  [2, 3, 4, 6],
  [2, 4, 7, 8],
  [4, 4, 6, 8],
];

function endlessNumberKey(numbers) {
  if (puzzle && typeof puzzle.numberKey === 'function') return puzzle.numberKey(numbers);
  return numbers.slice().sort((left, right) => left - right).join(',');
}

function makeFastEndlessRecord(questionIndex, runSeed, usedKeys) {
  const config = puzzle.endlessConfig(questionIndex);
  const start = Math.abs(Number(runSeed) || 1) % FAST_ENDLESS_NUMBERS.length;
  for (let offset = 0; offset < FAST_ENDLESS_NUMBERS.length; offset += 1) {
    const numbers = FAST_ENDLESS_NUMBERS[(start + offset) % FAST_ENDLESS_NUMBERS.length];
    const key = endlessNumberKey(numbers);
    if (usedKeys && usedKeys[key]) continue;
    const record = puzzle.makeVerifiedRecord(
      numbers,
      2000 + Number(questionIndex),
      Number(questionIndex),
      config.minSolutions,
      config.maxSolutions,
    );
    if (!record || !Array.isArray(record.numbers) || !Array.isArray(record.solutionSteps)) continue;
    record.puzzleId = `ENDLESS-Q${String(Number(questionIndex) + 1).padStart(4, '0')}`;
    record.puzzle_id = record.puzzleId;
    record.endless_stage = config.stage;
    record.generation = {
      source: 'verified_endless_bootstrap',
      validated: true,
      validator: 'PuzzleGenerator.solve',
      candidate_attempts: offset + 1,
      number_key: key,
    };
    if (usedKeys) usedKeys[key] = true;
    return record;
  }
  return null;
}

function samePuzzleNumbers(left, right) {
  if (!Array.isArray(left) || !Array.isArray(right) || left.length !== 4 || right.length !== 4) return false;
  return endlessNumberKey(left) === endlessNumberKey(right);
}

class ModeController {
  isBackendRequired() {
    // GameApp always creates backendAuth. Keeping the guard makes the
    // controller safe for offline tools and unit-test harnesses that only
    // exercise the local game rules.
    return Boolean(this.backendAuth && apiClient.isConfigured && apiClient.isConfigured());
  }

  ensureBackendReady(mode, fallbackScreen) {
    if (!this.isBackendRequired()) return true;
    if (this.backendAuth.status === 'ready') return true;
    const status = String(this.backendAuth.status || '').toLowerCase();
    this.status = status === 'pending' || status === 'syncing'
      ? '正在连接服务器，请稍后重试'
      : '服务器暂时不可用，请点击重试';
    if (fallbackScreen) this.screen = fallbackScreen;
    if (mode === 'friend' && this.friendMatchmaking) {
      this.friendMatchmaking.status = 'error';
      this.friendMatchmakingError = this.status;
    }
    return false;
  }

  markBackendModeUnavailable(message, fallbackScreen) {
    this.status = message || '服务器暂时不可用，请点击重试';
    if (fallbackScreen) this.screen = fallbackScreen;
    this.campaignRun = null;
    this.campaignRunLoading = false;
    this.dailyRun = null;
    this.dailyRunLoading = false;
    this.endlessRun = null;
    this.endlessRunLoading = false;
    if (this.friendMatchmaking) {
      this.friendMatchmaking.status = 'error';
      this.friendMatchmakingError = this.status;
    }
  }

  startCampaign(index, options = {}) {
    const forceRestart = Boolean(options && options.forceRestart);
    if (!forceRestart && this.campaignRunLoading) {
      this.status = '题目正在准备，请稍候';
      return;
    }
    if (!forceRestart && !this.isCampaignLevelUnlocked(index)) {
      if (Number(index) >= 100 && !this.isCampaignBlockUnlocked(Math.floor(Number(index) / 100))) {
        this.status = `前 100 关累计达到 ${this.campaignBlockGateScore()} 分后解锁下一阶段`;
      }
      return;
    }
    this.mode = 'campaign';
    this.hintPopup = null;
    this.resultHelpPopup = false;
    this.dailyChallenge = null;
    this.currentLevel = index;
    const config = this.levels[index];
    const questionCount = safeNumber(config.questionCount || config.question_count, 3);
    const requestToken = ++this.gameRequestToken;

    this.campaignRun = null;
    this.campaignAttempts = [];
    this.campaignRunLoading = false;
    // 闯关题目必须由本地固定题库决定。服务端可以返回同一关的校验运行记录，
    // 但不能用随机题目覆盖本地题库，否则玩家重复进入同一关会看到不同题目。
    // 闯关题目来自本地固定题库，不能因为登录、网络或服务端记录接口
    // 尚未就绪而阻塞“点击关卡立即开始”。服务器记录在后台尽力同步。
    const backendReady = Boolean(this.backendAuth && this.backendAuth.status === 'ready');
    this.startCampaignLocal(index, config, questionCount, true);
    if (!this.puzzles || this.puzzles.length !== questionCount) return;
    if (backendReady && apiClient.createCampaignRun) {
      this.campaignRunLoading = true;
      this.status = '正在同步闯关记录…';
      apiClient.createCampaignRun(index).then((run) => {
        if (requestToken !== this.gameRequestToken || this.mode !== 'campaign' || this.currentLevel !== index) return;
        if (!run || !run.run_id || !Array.isArray(run.puzzles) || run.puzzles.length !== questionCount) {
          throw new Error('服务端闯关运行记录无效');
        }
        const fixed = this.puzzles;
        const sameNumbers = run.puzzles.every((record, questionIndex) => {
          const expected = fixed[questionIndex];
          return expected && Array.isArray(record.numbers)
            && record.numbers.length === expected.numbers.length
            && record.numbers.every((value, numberIndex) => Number(value) === Number(expected.numbers[numberIndex]))
            && record.numbers.every((value) => Number.isInteger(Number(value)) && Number(value) >= 1 && Number(value) <= 10);
        });
        if (!sameNumbers) throw new Error('服务端闯关题目与本地固定题库不一致');
        this.campaignRun = run;
        this.campaignRunLoading = false;
      }).catch((error) => {
        if (requestToken !== this.gameRequestToken || this.mode !== 'campaign' || this.currentLevel !== index) return;
        this.campaignRun = null;
        this.campaignRunLoading = false;
        this.status = '服务器记录暂不可用，本局继续按本地闯关';
        try { if (typeof console !== 'undefined' && console.warn) console.warn('[game-backend-campaign-start]', error); } catch (logError) { /* start failure is shown in the UI */ }
      });
    }
  }

  startCampaignLocal(index, config = this.levels[index] || {}, questionCount = 3, startImmediately = true) {
    this.mode = 'campaign';
    this.currentLevel = index;
    this.campaignRun = null;
    this.campaignAttempts = [];
    this.campaignRunLoading = false;
    const service = this.ensureQuestionService();
    this.puzzles = service.getCampaignLevel(index, { config, count: questionCount });
    if (this.puzzles.length < questionCount) {
      this.status = service.lastError || '题目生成失败，请重新进入';
      this.screen = 'levels';
      return;
    }
    this.currentQuestion = 0;
    if (startImmediately) this.beginSession(safeNumber(config.timeLimit || config.time_limit, 60));
  }

  startDaily() {
    const requestToken = ++this.gameRequestToken;
    if (!this.ensureBackendReady('daily', 'home')) return;
    const today = storage.todayKey();
    const currentProgress = this.progress;
    const latestProgress = storage.load();
    const currentCompleted = storage.isDailyCompleted(currentProgress, today);
    const savedCompleted = storage.isDailyCompleted(latestProgress, today);
    this.progress = currentCompleted ? currentProgress : latestProgress;
    if (currentCompleted || savedCompleted) {
      this.dailyChallenge = null;
      this.status = '今日挑战已完成，明天零点更新';
      this.screen = 'home';
      return;
    }
    this.mode = 'daily';
    this.hintPopup = null;
    this.resultHelpPopup = false;
    this.currentLevel = 0;
    this.dailyRun = null;
    this.dailyAttempts = [];
    this.dailyRunLoading = false;
    if (this.backendAuth && this.backendAuth.status === 'ready' && apiClient.createDailyRun) {
      this.dailyRunLoading = true;
      this.status = '正在向服务端领取今日挑战…';
      apiClient.createDailyRun().then((run) => {
        if (requestToken !== this.gameRequestToken || this.mode !== 'daily') return;
        if (run && (run.completed === true || run.done === true || String(run.status || '').toLowerCase() === 'completed')) {
          const completedDate = String(run.date_key || run.dateKey || today);
          this.progress.daily = this.progress.daily || { last_date: '', streak: 0, best_score: 0, completed: {}, reward_claimed: {} };
          this.progress.daily.completed = this.progress.daily.completed || {};
          this.progress.daily.completed[completedDate] = true;
          this.progress.daily.last_date = completedDate;
          storage.save(this.progress);
          this.dailyRun = null;
          this.dailyRunLoading = false;
          this.dailyChallenge = null;
          this.status = '今日挑战已完成，明天零点更新';
          this.screen = 'home';
          return;
        }
        if (!run || !run.run_id || !Array.isArray(run.puzzles) || run.puzzles.length !== dailyChallenge.DAILY_QUESTION_COUNT) {
          throw new Error('服务端每日挑战题目合同无效');
        }
        this.dailyRun = run;
        this.dailyRunLoading = false;
        this.dailyChallenge = {
          ...run,
          date_key: run.date_key || run.dateKey || storage.todayKey(),
          rule_id: run.rule_id || run.ruleId || '',
          rule_title: run.rule_title || run.ruleTitle || '',
          rule_text: run.rule_text || run.ruleText || '',
          time_limit: Number(run.time_limit || run.timeLimitSeconds || 150),
          time_limit_ms: Number(run.time_limit_ms || run.timeLimitMS || 150000),
          hint_count: Number(run.hint_count || run.hintCount || 1),
          allow_hint: run.allow_hint !== undefined ? Boolean(run.allow_hint) : true,
          required_operator: run.required_operator || run.requiredOperator || '',
          forbidden_operator: run.forbidden_operator || run.forbiddenOperator || '',
          puzzles: run.puzzles.map((record) => ({
            ...record,
            puzzleId: record.puzzleId || record.puzzle_id,
            puzzle_id: record.puzzle_id || record.puzzleId,
            target: 24,
            rules: Object.assign({ integerIntermediateResults: true, integer_intermediate_results: true, allowNegativeIntermediate: false, allow_negative_intermediate: false }, record.rules || {}),
          })),
        };
        this.puzzles = this.dailyChallenge.puzzles;
        this.currentQuestion = 0;
        this.beginSession(Math.max(1, Number(this.dailyChallenge.time_limit_ms || this.dailyChallenge.time_limit * 1000 || 150000) / 1000));
      }).catch((error) => {
        if (requestToken !== this.gameRequestToken || this.mode !== 'daily') return;
        this.markBackendModeUnavailable('服务器每日挑战暂不可用，请重试', 'home');
        try { if (typeof console !== 'undefined' && console.warn) console.warn('[game-backend-daily-start]', error); } catch (logError) { /* start failure is shown in the UI */ }
      });
      return;
    }
    this.startDailyLocal();
  }

  startDailyLocal() {
    if (this.isBackendRequired()) {
      this.markBackendModeUnavailable('服务器每日挑战暂不可用，请重试', 'home');
      return;
    }
    this.dailyRun = null;
    this.dailyAttempts = [];
    this.dailyRunLoading = false;
    const service = this.ensureQuestionService();
    this.dailyChallenge = service.getDailyChallenge(storage.todayKey(), storage.todaySeed());
    if (!this.dailyChallenge || !Array.isArray(this.dailyChallenge.puzzles) || this.dailyChallenge.puzzles.length < dailyChallenge.DAILY_QUESTION_COUNT) {
      this.status = service.lastError || '今日挑战题目生成失败，请稍后重试';
      this.screen = 'home';
      return;
    }
    this.puzzles = this.dailyChallenge.puzzles;
    this.currentQuestion = 0;
    this.beginSession(safeNumber(this.dailyChallenge.time_limit, 90));
  }

  startEndless() {
    const requestToken = ++this.gameRequestToken;
    this.mode = 'endless';
    this.hintPopup = null;
    this.resultHelpPopup = false;
    this.dailyChallenge = null;
    this.currentQuestion = 0;
    this.score = 0;
    this.combo = 0;
    this.mistakes = 0;
    this.maxCombo = 0;
    this.freeUndo = true;
    this.freeHint = true;
    this.endlessSeed = makeEndlessSeed();
    this.endlessUsedKeys = {};
    this.endlessRunId = `endless-${this.endlessSeed}-${Date.now()}`;
    this.endlessRun = null;
    this.endlessAttempts = [];
    this.endlessServerResult = null;
    this.endlessRunLoading = false;
    // 先使用本地已验证首题进入游戏，服务端合同在后台同步。
    this.endlessLocalFallback = true;
    this.puzzles = [];

    // 不让登录或网络请求阻塞首帧；首题生成只使用很小的已验证候选池。
    this.beginEndlessQuestion({ fastStart: true });

    if (this.backendAuth && this.backendAuth.status === 'ready' && apiClient.createEndlessRun) {
      this.endlessRunLoading = true;
      apiClient.createEndlessRun().then((run) => {
        if (requestToken !== this.gameRequestToken) return;
        this.endlessRunLoading = false;
        if (this.mode !== 'endless' || this.screen !== 'game' || this.transitioning) return;
        if (!run || !run.run_id || !Array.isArray(run.puzzles) || !run.puzzles.length) throw new Error('服务端无尽题目合同为空');
        const serverPuzzle = run.puzzles[0];
        const localPuzzle = this.currentPuzzle;
        // 题目不一致时绝不替换玩家正在玩的题，继续安全的本地验证流程。
        if (!serverPuzzle || !samePuzzleNumbers(localPuzzle && localPuzzle.numbers, serverPuzzle.numbers)) {
          this.endlessLocalFallback = true;
          return;
        }
        this.endlessRun = run;
        this.endlessRunId = String(run.run_id);
        this.endlessSeed = Number(run.run_seed || this.endlessSeed);
        if (localPuzzle) {
          localPuzzle.puzzleId = serverPuzzle.puzzleId || serverPuzzle.puzzle_id || localPuzzle.puzzleId;
          localPuzzle.puzzle_id = serverPuzzle.puzzle_id || serverPuzzle.puzzleId || localPuzzle.puzzle_id;
        }
        this.endlessLocalFallback = false;
      }).catch((error) => {
        if (requestToken !== this.gameRequestToken || this.mode !== 'endless') return;
        this.endlessRunLoading = false;
        this.endlessLocalFallback = true;
        try { if (typeof console !== 'undefined' && console.warn) console.warn('[game-backend-endless-start]', error); } catch (logError) { /* start failure is shown in the UI */ }
      });
    }
  }

  startFriend() {
    if (this.friendRoomExpired || this.friendConnectionState === 'expired') {
      this.triggerFeedback('error', '房间已过期，请重新创建或加入房间');
      return;
    }
    const localMode = this.friendRoomBackendStatus === 'local' || this.friendLocalFallback;
    const players = this.friendRoom && Array.isArray(this.friendRoom.players) ? this.friendRoom.players : [];
    const currentPlayerID = this.backendAuth && this.backendAuth.user
      ? String(this.backendAuth.user.id || this.backendAuth.user.user_id || '')
      : 'local-player';
    const opponent = players.find((player) => String(player && (player.user_id || player.id) || '') !== currentPlayerID);
    const selfReady = this.friendSelfReady === undefined ? true : Boolean(this.friendSelfReady);
    const opponentReady = opponent ? Boolean(opponent.ready) : false;
    if ((!localMode && this.friendRoomBackendStatus !== 'ready') || !this.friendRoom || !friendMatch.isRoomReady(this.friendRoom) || !selfReady || !opponentReady) {
      this.triggerFeedback('info', !selfReady ? '\u8bf7\u5148\u70b9\u51c6\u5907' : !opponentReady ? '\u8bf7\u7b49\u5f85\u5bf9\u624b\u51c6\u5907' : '\u8bf7\u7b49\u5f85\u5bf9\u624b\u52a0\u5165\u623f\u95f4');
      return;
    }
    if (!localMode && this.backendAuth && this.backendAuth.status === 'ready' && apiClient.startFriendRoom && this.friendStartRequestInFlight !== undefined && !this.friendStartRequestInFlight && !this.friendServerStartAt) {
      const roomCode = String(this.friendRoom.room_code || '').trim();
      if (roomCode) {
        this.friendStartRequestInFlight = true;
        apiClient.startFriendRoom(roomCode).then((payload) => {
          const source = payload && payload.data && typeof payload.data === 'object' ? payload.data : (payload || {});
          const remoteRoom = source.room || source.match || source;
          if (this.isFriendRoomTerminal(remoteRoom)) {
            this.friendStartRequestInFlight = false;
            this.markFriendRoomExpired(String(remoteRoom.status || remoteRoom.state || 'expired'));
            return;
          }
          if (remoteRoom && (remoteRoom.room_seed || remoteRoom.room_code || remoteRoom.match_id)) {
            this.friendRoom = Object.assign({}, this.friendRoom, remoteRoom, { status: 'ready' });
            this.friendRules = Object.assign({}, friendMatch.rules(), this.friendRoom.rules || {});
          }
          this.friendServerStartAt = Number(source.start_at || source.startAt || remoteRoom.start_at || remoteRoom.startAt || 0) || 0;
          this.friendStartRequestInFlight = false;
          this.startFriend();
        }).catch((error) => {
          this.friendStartRequestInFlight = false;
          if (this.isFriendRoomTerminalError(error)) this.markFriendRoomExpired('expired');
          else this.beginFriendReconnect(error, 'start');
          this.triggerFeedback('error', '\u5bf9\u6218\u5f00\u59cb\u5931\u8d25\uff0c\u8bf7\u91cd\u8bd5');
          try { if (typeof console !== 'undefined' && console.warn) console.warn('[friend-start]', error); } catch (logError) { /* start failure is shown in the UI */ }
        });
        return;
      }
    }
    this.mode = 'friend';
    this.markFriendConnectionRecovered();
    this.hintPopup = null;
    this.resultHelpPopup = false;
    this.dailyChallenge = null;
    this.currentQuestion = 0;
    const roomSeed = Number(this.friendRoom.room_seed || 0);
    if (!Number.isFinite(roomSeed) || roomSeed <= 0) {
      this.status = '好友房间缺少服务端题目种子，请重新创建房间';
      this.screen = 'friend_lobby';
      return;
    }
    this.friendRules = Object.assign({}, friendMatch.rules(), this.friendRoom.rules || {});
    this.friendRoom.rules = this.friendRules;
    this.friendLocalFallback = localMode;
    this.friendSeed = roomSeed;
    const service = this.ensureQuestionService();
    const questionCount = this.friendQuestionCount();
    const serverPuzzles = Array.isArray(this.friendRoom.puzzles) ? this.friendRoom.puzzles : [];
    this.puzzles = serverPuzzles.length === questionCount
      ? serverPuzzles.map((record) => ({ ...record, puzzleId: record.puzzleId || record.puzzle_id }))
      : service.getFriendQuestions(roomSeed, { count: questionCount });
    if (this.puzzles.length < questionCount) {
      this.status = service.lastError || '对战题目生成失败，请重新创建房间';
      this.screen = 'friend_lobby';
      return;
    }
    this.friendMatch = friendMatch.createMatch(this.friendRoom, this.puzzles);
    this.friendMatchContract = matchData.createMatchContract(this.friendRoom, this.puzzles);
    this.friendMatchContract.ranked = Boolean(this.friendRanked && !localMode);
    if (this.friendMatchContract.ranked) {
      const rank = rankService.normalize(this.progress && this.progress.rank);
      this.friendMatchContract.season_id = rank.season_id;
    }
    if (this.friendRoom.question_hash) this.friendMatchContract.question_hash = String(this.friendRoom.question_hash);
    if (Array.isArray(this.friendRoom.puzzle_ids) && this.friendRoom.puzzle_ids.length === questionCount) {
      this.friendMatchContract.puzzle_ids = this.friendRoom.puzzle_ids.map((id) => String(id));
    }
    this.friendAttempts = [];
    this.friendServerResult = null;
    this.friendMatchProgress = { players: [] };
    this.friendProgressLastPollAt = 0;
    this.friendProgressLastSentKey = '';
    this.friendMatchResolutionApplied = false;
    this.friendStartedAt = Date.now();
    this.friendPlayerSolved = 0;
    this.beginSession(this.friendTimeLimit());
    const now = Date.now();
    this.friendCountdownActive = true;
    this.friendCountdownUntil = Number(this.friendServerStartAt || 0) > now ? Number(this.friendServerStartAt) : now + 3200;
    this.friendCountdownLastNumber = 0;
    this.gamePaused = false;
    this.sendFriendMatchProgress(false);
  }

  showFriendLobby() {
    this.gameRequestToken += 1;
    this.popup = '';
    this.friendRoomFromInvite = false;
    this.friendLobbyView = 'entry';
    this.friendLocalFallback = false;
    this.friendRanked = false;
    this.friendRoomBackendStatus = 'idle';
    this.friendRoomBackendLoading = false;
    this.friendRoomLastPollAt = 0;
    this.friendRoom = null;
    this.friendRoomInput = '';
    this.friendRules = friendMatch.rules();
    this.friendBotDifficulty = 'standard';
    this.friendBotName = '';
    this.friendSelfReady = false;
    this.friendReadyRequestInFlight = false;
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
  }

  beginSession(timeLimit) {
    this.renderRecovery = false;
    this.settling = false;
    this.settleToken += 1;
    this.transitioning = false;
    this.autoNextAt = 0;
    this.autoNextToken += 1;
    this.currentQuestion = this.currentQuestion || 0;
    this.score = this.currentQuestion === 0 ? 0 : this.score;
    this.combo = this.currentQuestion === 0 ? 0 : this.combo;
    this.mistakes = this.currentQuestion === 0 ? 0 : this.mistakes;
    this.maxCombo = this.currentQuestion === 0 ? 0 : this.maxCombo;
    this.freeUndo = this.currentQuestion === 0 ? true : this.freeUndo;
    this.freeHint = this.currentQuestion === 0 ? true : this.freeHint;
    this.hintUsed = this.currentQuestion === 0 ? false : this.hintUsed;
    this.timerLimit = timeLimit;
    // 好友对战的 120 秒是整场共用，不会每答对一题就重新计时。
    if (this.mode !== 'friend' || this.currentQuestion === 0 || this.timeLeft <= 0) this.timeLeft = timeLimit;
    this.undoStack = [];
    this.questionOperators = [];
    this.questionSteps = [];
    this.status = this.mode === 'daily' && this.dailyChallenge ? this.dailyChallenge.rule_title : '';
    this.setupPuzzle(this.puzzles[this.currentQuestion]);
    if (this.mode !== 'friend') {
      this.friendCountdownActive = false;
      this.friendCountdownUntil = 0;
    }
    if (this.mode === 'friend' && !this.friendStartedAt) this.friendStartedAt = Date.now();
    this.screen = 'game';
  }

  beginEndlessQuestion(options = {}) {
    try {
      return this.beginEndlessQuestionInternal(options);
    } catch (error) {
      try { if (typeof console !== 'undefined' && console.error) console.error('[24点挑战][endless-fallback]', error); } catch (logError) { /* 静默降级 */ }
      const config = puzzle.endlessConfig(this.currentQuestion);
      const usedKeys = this.endlessUsedKeys || (this.endlessUsedKeys = {});
      const emergencyNumbers = [[1, 2, 4, 5], [1, 1, 2, 6], [3, 3, 4, 6], [2, 3, 4, 6], [3, 8, 1, 1]];
      for (const numbers of emergencyNumbers) {
        const key = numbers.slice().sort((a, b) => a - b).join(',');
        if (usedKeys[key]) continue;
        const record = puzzle.makeVerifiedRecord(numbers, 2000 + this.currentQuestion, this.currentQuestion, 1, 999999);
        if (!record || !Array.isArray(record.numbers) || record.numbers.length !== 4) continue;
        usedKeys[key] = true;
        this.puzzles = [record];
        this.timerLimit = config.timeLimit;
        this.timeLeft = config.timeLimit;
        this.setupPuzzle(record);
        this.status = '已切换备用题目，继续挑战';
        this.screen = 'game';
        this.transitioning = false;
        this.renderRecovery = false;
        return;
      }
      this.status = '本题暂时生成失败，请重新开始无尽模式';
      this.screen = 'result';
      this.result = { passed: false, score: this.score, stars: 0, combo: this.maxCombo, mistakes: this.mistakes, reason: this.status, rewardCoins: 0, bonusLabels: [], levelComplete: false, next: false };
    }
  }

  beginEndlessQuestionInternal(options = {}) {
    this.renderRecovery = false;
    this.transitioning = false;
    this.autoNextAt = 0;
    this.autoNextToken += 1;
    const config = puzzle.endlessConfig(this.currentQuestion);
    const runSeed = safeNumber(this.endlessSeed, makeEndlessSeed()) || 1;
    this.endlessSeed = runSeed;
    const usedKeys = this.endlessUsedKeys || (this.endlessUsedKeys = {});

    // 首题使用小型已验证候选池，后续题目仍走完整的统一题目服务。
    if (options.fastStart && this.currentQuestion === 0) {
      const fastRecord = makeFastEndlessRecord(this.currentQuestion, runSeed, usedKeys);
      if (fastRecord) {
        this.puzzles = [fastRecord];
        this.timerLimit = config.timeLimit;
        this.timeLeft = config.timeLimit;
        this.setupPuzzle(fastRecord);
        this.screen = 'game';
        return;
      }
    }

    const serverPuzzle = this.endlessRun && Array.isArray(this.endlessRun.puzzles)
      ? this.endlessRun.puzzles[this.currentQuestion]
      : null;
    if (serverPuzzle && Array.isArray(serverPuzzle.numbers) && serverPuzzle.numbers.length === 4) {
      const record = {
        ...serverPuzzle,
        puzzleId: serverPuzzle.puzzleId || serverPuzzle.puzzle_id,
        puzzle_id: serverPuzzle.puzzle_id || serverPuzzle.puzzleId,
        target: 24,
        rules: Object.assign({ integerIntermediateResults: true, integer_intermediate_results: true, allowNegativeIntermediate: true, allow_negative_intermediate: true }, serverPuzzle.rules || {}),
      };
      this.puzzles = [record];
      this.timerLimit = Math.max(18, Number(serverPuzzle.time_limit_ms || 45000) / 1000);
      this.timeLeft = this.timerLimit;
      this.setupPuzzle(record);
      this.screen = 'game';
      return;
    }

    // The shared service owns seeded generation and per-run de-duplication.
    // Keep the legacy path below as a safety net for unusual runtime failures.
    const service = this.ensureQuestionService();
    let serviceRecord = null;
    try {
      serviceRecord = service.getEndlessQuestion(this.currentQuestion, runSeed);
    } catch (error) {
      try { if (typeof console !== 'undefined' && console.error) console.error('[24点挑战][question-service-endless]', error); } catch (logError) { /* 静默降级 */ }
    }
    if (serviceRecord && Array.isArray(serviceRecord.numbers) && serviceRecord.numbers.length === 4) {
      const serviceKey = puzzle.numberKey(serviceRecord.numbers);
      usedKeys[serviceKey] = true;
      this.puzzles = [serviceRecord];
      this.timerLimit = config.timeLimit;
      this.timeLeft = config.timeLimit;
      this.setupPuzzle(serviceRecord);
      if (serviceRecord.generation && serviceRecord.generation.source !== 'seeded_endless_generation') this.status = '已切换备用题目，继续挑战';
      this.screen = 'game';
      return;
    }

    let record = null;
    let usedFallback = false;
    const seedCandidates = [
      runSeed + this.currentQuestion * 1013,
      runSeed + this.currentQuestion * 7919 + 17,
      runSeed + this.currentQuestion * 104729 + 31,
    ];

    // 每局使用独立 runSeed，并由 endlessMode 记录已出现的数字组合，避免重开后和同局内重复。
    for (const seed of seedCandidates) {
      try {
        record = endlessMode.generateQuestion(puzzle, this.currentQuestion, seed, usedKeys);
      } catch (error) {
        try { if (typeof console !== 'undefined' && console.error) console.error('[24点挑战][endless-generate]', error); } catch (logError) { /* 静默降级 */ }
      }
      if (record && Array.isArray(record.numbers) && record.numbers.length === 4) break;
      record = null;
    }

    // 高阶段严格解法数量可能生成失败，使用更宽松但仍经过验证的备用配置，不直接结束游戏。
    if (!record) {
      const fallbackConfigs = [
        { ...config, maxSolutions: 40, max_solutions: 40 },
        { ...config, minSolutions: 1, min_solutions: 1, maxSolutions: 999999, max_solutions: 999999 },
      ];
      for (let index = 0; index < fallbackConfigs.length && !record; index += 1) {
        const fallbackSeed = runSeed + this.currentQuestion * 131071 + index * 65537 + 97;
        const list = puzzle.generatePuzzleSet(fallbackConfigs[index], this.currentQuestion, 1, fallbackSeed);
        const candidate = list[0];
        const key = candidate && Array.isArray(candidate.numbers) ? candidate.numbers.slice().sort((a, b) => a - b).join(',') : '';
        if (candidate && key && !usedKeys[key]) {
          usedKeys[key] = true;
          record = candidate;
          usedFallback = true;
        }
      }
    }

    // 最终静态兜底只用于防止极端设备上生成器耗尽导致闪退；题目仍会经过同一套验证。
    if (!record) {
      const emergencyNumbers = [[1, 2, 4, 5], [1, 1, 2, 6], [3, 3, 4, 6], [2, 3, 4, 6], [3, 8, 1, 1]];
      for (const numbers of emergencyNumbers) {
        const key = numbers.slice().sort((a, b) => a - b).join(',');
        if (usedKeys[key]) continue;
        const candidate = puzzle.makeVerifiedRecord(numbers, 2000 + this.currentQuestion, this.currentQuestion, 1, 999999);
        if (candidate && Array.isArray(candidate.numbers) && candidate.numbers.length === 4) {
          usedKeys[key] = true;
          record = candidate;
          usedFallback = true;
          break;
        }
      }
    }

    if (!record) {
      this.status = '本题暂时生成失败，请重新开始无尽模式';
      this.screen = 'result';
      this.result = { passed: false, score: this.score, stars: 0, combo: this.maxCombo, mistakes: this.mistakes, reason: this.status, rewardCoins: 0, bonusLabels: [], levelComplete: false, next: false };
      return;
    }
    this.puzzles = [record];
    this.timerLimit = config.timeLimit;
    this.timeLeft = config.timeLimit;
    this.setupPuzzle(record);
    if (usedFallback) this.status = '已切换备用题目，继续挑战';
    this.screen = 'game';
  }

  setupPuzzle(record) {
    this.currentPuzzle = record;
    this.originalCards = record.numbers.map((value, index) => ({ id: index, sourceIndices: [index], value: Number(value) }));
    this.cards = this.originalCards.map((card) => ({ ...card }));
    this.undoStack = [];
    this.questionOperators = [];
    this.questionSteps = [];
    this.selectedIndex = -1;
    this.selectedOperator = '';
    this.status = this.mode === 'daily' && this.dailyChallenge ? this.dailyChallenge.rule_title : '';
  }

  selectCard(index) {
    if (!Number.isInteger(index) || index < 0 || index >= this.cards.length) {
      if (this.selectedIndex >= 0 && this.selectedOperator) this.applyOperation(index);
      else {
        this.selectedIndex = -1;
        this.selectedOperator = '';
      }
      return;
    }
    if (this.selectedIndex >= this.cards.length || !this.cards[this.selectedIndex]) {
      this.selectedIndex = -1;
      this.selectedOperator = '';
    }
    if (this.selectedIndex >= 0 && this.selectedOperator) {
      this.applyOperation(index);
      return;
    }
    this.selectedIndex = this.selectedIndex === index ? -1 : index;
    this.status = this.selectedIndex >= 0 ? '已选择第一个数字，请选择运算符' : '';
    this.audio.playCard();
    this.triggerFeedback('info', this.selectedIndex >= 0 ? '已选数字' : '已取消选择');
  }

  selectOperator(operator) {
    if (this.selectedIndex < 0) {
      this.status = '请先选择一个数字';
      return;
    }
    const puzzleRules = (this.currentPuzzle && this.currentPuzzle.rules) || {};
    const forbiddenOperator = this.mode === 'campaign' ? '' : (puzzleRules.forbiddenOperator || puzzleRules.forbidden_operator || '');
    if (forbiddenOperator && operator === forbiddenOperator) {
      this.status = `今日规则禁止使用 ${operator === '×' ? '乘法' : operator === '+' ? '加法' : operator === '-' ? '减法' : '除法'}`;
      return;
    }
    this.selectedOperator = operator;
    this.status = '已选择运算符，请选择第二个数字';
    this.audio.playOperator();
    this.triggerFeedback('info', `已选 ${operator}，请点第二个数字`);
  }

  applyOperation(secondIndex) {
    if (this.screen !== 'game' || this.transitioning || this.selectedIndex < 0 || !this.selectedOperator) return;
    if (!Number.isInteger(secondIndex) || secondIndex < 0 || secondIndex >= this.cards.length) {
      this.selectedIndex = -1;
      this.selectedOperator = '';
      this.status = '数字状态已刷新，请重新选择';
      return;
    }
    if (this.selectedIndex >= this.cards.length || !this.cards[this.selectedIndex]) {
      this.selectedIndex = -1;
      this.selectedOperator = '';
      this.status = '数字状态已刷新，请重新选择';
      return;
    }
    if (secondIndex === this.selectedIndex) {
      this.status = '请选择另一个数字';
      return;
    }
    const first = this.cards[this.selectedIndex];
    const second = this.cards[secondIndex];
    if (!first || !second || !Number.isFinite(Number(first.value)) || !Number.isFinite(Number(second.value))) {
      this.selectedIndex = -1;
      this.selectedOperator = '';
      this.status = '数字状态异常，请重新开始本题';
      return;
    }
    const operator = this.selectedOperator;
    let value = null;
    const puzzleRules = (this.currentPuzzle && this.currentPuzzle.rules) || {};
    const forbiddenOperator = this.mode === 'campaign' ? '' : (puzzleRules.forbiddenOperator || puzzleRules.forbidden_operator || '');
    if (forbiddenOperator && operator === forbiddenOperator) {
      this.mistakes += 1;
      this.selectedIndex = -1;
      this.selectedOperator = '';
      this.status = `今日规则禁止使用 ${operator === '×' ? '乘法' : operator === '+' ? '加法' : operator === '-' ? '减法' : '除法'}`;
      this.audio.playError();
      this.triggerFeedback('error', this.status);
      return;
    }
    if (this.selectedOperator === '+') value = first.value + second.value;
    else if (this.selectedOperator === '-') value = first.value - second.value;
    else if (this.selectedOperator === '×') value = first.value * second.value;
    else if (this.selectedOperator === '÷' && second.value !== 0 && first.value % second.value === 0) value = first.value / second.value;
    if (!Number.isInteger(value)) {
      if (this.mode === 'friend') this.timeLeft = Math.max(0, this.timeLeft - 5);
      this.mistakes += 1;
      this.status = this.selectedOperator === '÷' && second.value === 0 ? '除数不能为 0' : '第一版只允许整数结果';
      this.audio.playError();
      this.triggerFeedback('error', this.status);
      this.selectedIndex = -1;
      this.selectedOperator = '';
      if (this.mode === 'friend' && this.timeLeft <= 0) this.finish(false, '答错扣时后时间用尽');
      return;
    }
    const operationStep = {
      first_indices: (first.sourceIndices || [first.id]).slice(),
      second_indices: (second.sourceIndices || [second.id]).slice(),
      first: Number(first.value),
      second: Number(second.value),
      operator,
    };
    this.undoStack.push(this.cards.map((card) => ({ ...card })));
    this.questionSteps.push(operationStep);
    first.value = value;
    first.sourceIndices = (first.sourceIndices || [first.id]).concat(second.sourceIndices || [second.id]);
    this.cards.splice(secondIndex, 1);
    this.selectedIndex = -1;
    this.selectedOperator = '';
    this.status = '';
    this.questionOperators.push(operator);
    this.audio.playMerge();
    if (this.cards.length === 1) {
      const requiredOperator = puzzleRules.requiredOperator || puzzleRules.required_operator || '';
      const requiredMissing = requiredOperator && !this.questionOperators.includes(requiredOperator);
      if (Number(this.cards[0].value) === 24 && !requiredMissing) {
        // 统一走最终状态检查，避免“点击最后一个数字”和“主循环兜底”
        // 分别维护两套通关逻辑，导致真机偶发停在 24。
        this.checkSolvedState();
        return;
      }
      else {
        if (this.mode === 'friend') this.timeLeft = Math.max(0, this.timeLeft - 5);
        this.mistakes += 1;
        this.status = requiredMissing ? `今日规则要求使用 ${requiredOperator}，已自动重置本题` : this.mode === 'friend' ? '结果不是 24，扣除 5 秒并重置本题' : '还差一点，已自动重置本题';
        this.audio.playError();
        this.triggerFeedback('error', this.status);
        if (this.mode === 'friend' && this.timeLeft <= 0) this.finish(false, '答错扣时后时间用尽');
        else this.setupPuzzle(this.currentPuzzle);
      }
    }
  }

  resetPuzzle() {
    if (this.currentPuzzle) {
      this.setupPuzzle(this.currentPuzzle);
      this.triggerFeedback('info', '本题已重置');
    }
  }

  undo() {
    if (this.mode === 'daily' && this.dailyChallenge && this.dailyChallenge.rule_id === 'no_undo') {
      this.status = '今日规则禁止撤销，每一步都要先想清楚';
      return;
    }
    if (!this.undoStack.length) { this.status = '没有可以撤销的操作'; return; }
    const restore = () => {
      this.cards = this.undoStack.pop();
      this.questionOperators = this.questionOperators.slice(0, -1);
      this.questionSteps = this.questionSteps.slice(0, -1);
      this.selectedIndex = -1;
      this.selectedOperator = '';
      this.status = '';
    };
    if (this.freeUndo) { this.freeUndo = false; restore(); this.triggerFeedback('info', '已使用本局免费撤销'); return; }
    this.showRewarded('undo', () => { restore(); this.triggerFeedback('success', '看完广告，获得一次撤销'); });
  }

  hint() {
    const step = this.currentPuzzle && this.currentPuzzle.firstStep;
    if (!step) { this.status = '这道题暂时没有提示'; return; }
    if (this.mode === 'friend') {
      this.status = '好友对战不允许使用提示';
      return;
    }
    if (this.mode === 'campaign' && this.levels[this.currentLevel] && this.levels[this.currentLevel].allowHint === false) {
      this.status = '本关暂不允许使用提示';
      return;
    }
    const show = () => {
      this.hintPopup = { ...step };
      this.status = '提示已显示，点击屏幕任意位置关闭';
    };
    if (this.freeHint) { this.freeHint = false; this.hintUsed = true; show(); this.triggerFeedback('info', '已使用本局免费提示'); return; }
    this.showRewarded('hint', () => { this.hintUsed = true; show(); this.triggerFeedback('success', '看完广告，获得一次提示'); });
  }

}

function install(GameApp) {
  const source = ModeController.prototype;
  Object.getOwnPropertyNames(source).forEach((name) => {
    if (name !== 'constructor') GameApp.prototype[name] = source[name];
  });
}

module.exports = { install };
