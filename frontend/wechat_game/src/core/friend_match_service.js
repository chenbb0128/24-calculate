const QUESTION_COUNT = 8;
const TIME_LIMIT = 120;
const MATCHMAKING_TIMEOUT = 15;
const MATCHMAKING_POLL_INTERVAL = 1000;
const DAILY_REWARD_MATCH_LIMIT = 3;
const ROOM_STATUS = Object.freeze({
  WAITING: 'waiting',
  READY: 'ready',
  COUNTDOWN: 'countdown',
  RUNNING: 'running',
  FINISHED: 'finished',
  EXPIRED: 'expired',
  CANCELLED: 'cancelled',
});

function safeRoomCode(value) {
  const digits = String(value || '').replace(/\D/g, '').slice(-6);
  return digits.length === 6 ? digits : '';
}

function sanitizeRoomCode(value) {
  return String(value || '').replace(/\D/g, '').slice(0, 6);
}

function roomSeedFromCode(roomCode) {
  const code = safeRoomCode(roomCode);
  if (!code) return 1;
  let state = 2166136261;
  code.split('').forEach((character) => {
    state ^= character.charCodeAt(0);
    state = Math.imul(state, 16777619);
  });
  return (state >>> 0) || 1;
}

function localRoomCode(seedValue) {
  const safeSeed = Math.abs(Number(seedValue) || Date.now());
  return String(100000 + (safeSeed % 900000)).padStart(6, '0');
}

function createRoom(seedValue, ownerName = '我') {
  const safeSeed = Math.abs(Number(seedValue) || 0);
  const roomCode = String(100000 + (safeSeed % 900000)).padStart(6, '0');
  return {
    version: 1, room_id: `friend-${roomCode}`, room_code: roomCode, room_seed: safeSeed,
    owner: { id: 'local-player', name: ownerName }, players: [{ id: 'local-player', name: ownerName, ready: true }],
    status: 'waiting', rules: rules(),
  };
}

function rules() { return { question_count: QUESTION_COUNT, time_limit: TIME_LIMIT, target: 24, no_hint: true, use_same_seed: true, integer_intermediate_results: true }; }
function joinRoom(room, playerId = 'friend-local', playerName = '好友') {
  const joined = JSON.parse(JSON.stringify(room || {}));
  const players = Array.isArray(joined.players) ? joined.players : [];
  if (!players.some((player) => String(player.id) === playerId)) players.push({ id: playerId, name: playerName, ready: true });
  joined.players = players; joined.status = 'ready'; return joined;
}

function createLocalRoom(roomCode = '', ownerName = '我', opponentName = '好友') {
  const requestedCode = safeRoomCode(roomCode);
  const code = requestedCode || localRoomCode(Date.now() + Math.floor(Math.random() * 900000));
  const room = createRoom(roomSeedFromCode(code), ownerName);
  room.room_code = code;
  room.room_id = `friend-${code}`;
  room.local_fallback = true;
  room.status = 'waiting';
  return joinRoom(room, 'friend-local', opponentName);
}

function normalizeRoom(room, fallbackRoom = null) {
  const source = room && typeof room === 'object' ? room : {};
  const fallback = fallbackRoom && typeof fallbackRoom === 'object' ? fallbackRoom : {};
  const code = safeRoomCode(source.room_code || fallback.room_code);
  const baseRules = rules();
  const mergedRules = Object.assign({}, baseRules, source.rules || fallback.rules || {});
  const players = Array.isArray(source.players) ? source.players : (Array.isArray(fallback.players) ? fallback.players : []);
  return Object.assign({}, fallback, source, {
    version: Number(source.version || fallback.version || 1),
    protocol_version: Number(source.protocol_version || fallback.protocol_version || 2),
    room_id: String(source.room_id || fallback.room_id || `friend-${code}`),
    room_code: code,
    room_seed: Math.abs(Number(source.room_seed || fallback.room_seed || roomSeedFromCode(code)) || 1),
    players,
    rules: mergedRules,
    status: String(source.status || fallback.status || (players.length >= 2 ? ROOM_STATUS.READY : ROOM_STATUS.WAITING)),
    local_fallback: Boolean(source.local_fallback || fallback.local_fallback),
  });
}

function isRoomReady(room) {
  const target = normalizeRoom(room);
  return [ROOM_STATUS.READY, ROOM_STATUS.COUNTDOWN, ROOM_STATUS.RUNNING].includes(target.status)
    && Array.isArray(target.players)
    && target.players.length >= 2;
}

function generatePuzzles(generator, roomSeed) {
  const config = { min_digit: 1, max_digit: 9, min_solutions: 1, max_solutions: 12, questionCount: QUESTION_COUNT, question_count: QUESTION_COUNT };
  let puzzles = generator.generatePuzzleSet(config, Math.abs(Number(roomSeed)) % 97, QUESTION_COUNT, Math.abs(Number(roomSeed)));
  if (puzzles.length < QUESTION_COUNT) puzzles = generator.generatePuzzleSet({ ...config, max_solutions: 999999 }, Math.abs(Number(roomSeed)) % 97, QUESTION_COUNT, Math.abs(Number(roomSeed)));
  return puzzles;
}

function seeded(seed) {
  let state = (Math.abs(Number(seed)) >>> 0) || 1;
  return {
    next() {
      state = (Math.imul(state, 1664525) + 1013904223) >>> 0;
      return state / 0x100000000;
    },
  };
}

function buildOpponentPlan(roomSeed, questionCount = QUESTION_COUNT, difficulty = 'standard') {
  const count = Math.max(1, Math.floor(Number(questionCount) || QUESTION_COUNT));
  const random = seeded(Number(roomSeed) + 24024);
  const profile = {
    easy: { min: 9, spread: 5, ramp: 0.35, scoreMin: 34, scoreSpread: 28 },
    standard: { min: 7, spread: 5, ramp: 0.35, scoreMin: 45, scoreSpread: 35 },
    hard: { min: 5.5, spread: 4, ramp: 0.25, scoreMin: 58, scoreSpread: 35 },
  }[String(difficulty || 'standard').toLowerCase()] || null;
  const selected = profile || { min: 8, spread: 8, ramp: 1.7, scoreMin: 45, scoreSpread: 35 };
  const solveTimes = [];
  const scoreDeltas = [];
  let elapsed = 0;
  for (let index = 0; index < count; index += 1) {
    // 每道题的完成时间是上一题之后继续累加的思考时间，不能把每题当成独立的绝对时间点。
    // 这样人机只能按顺序一题一题完成，不会在开局瞬间跳过多道题。
    const thinkTime = selected.min + random.next() * selected.spread + index * selected.ramp;
    elapsed += thinkTime;
    solveTimes.push(Number(elapsed.toFixed(2)));
    scoreDeltas.push(selected.scoreMin + Math.floor(random.next() * selected.scoreSpread));
  }
  return { question_count: count, difficulty: String(difficulty || 'standard'), solve_times: solveTimes, score_deltas: scoreDeltas };
}

function randomBotDifficulty(roomSeed) {
  const random = seeded(Number(roomSeed) + 90909);
  const roll = random.next();
  return roll < 0.34 ? 'easy' : roll < 0.82 ? 'standard' : 'hard';
}

function botProfile(difficulty) {
  const profiles = {
    easy: { id: 'bot-easy', name: '算术小火苗', label: '轻松', difficulty: 'easy' },
    standard: { id: 'bot-standard', name: '算术小能手', label: '标准', difficulty: 'standard' },
    hard: { id: 'bot-hard', name: '算术挑战者', label: '挑战', difficulty: 'hard' },
  };
  return profiles[String(difficulty || 'standard').toLowerCase()] || profiles.standard;
}

function opponentSnapshot(plan, elapsedSeconds, questionCount = QUESTION_COUNT, forceFinish = false) {
  const source = plan || {};
  const count = Math.max(1, Math.floor(Number(questionCount || source.question_count) || QUESTION_COUNT));
  const elapsed = Math.max(0, Number(elapsedSeconds) || 0);
  const solveTimes = Array.isArray(source.solve_times) ? source.solve_times : [];
  const scoreDeltas = Array.isArray(source.score_deltas) ? source.score_deltas : [];
  let solved = 0;
  let score = 0;
  let lastSolve = 0;
  for (let index = 0; index < count; index += 1) {
    const solveAt = Number(solveTimes[index] || (8 + index * 2));
    if (solveAt > elapsed) break;
    solved += 1;
    score += Math.max(0, Math.floor(Number(scoreDeltas[index] || 50)));
    lastSolve = solveAt;
  }
  return {
    solved,
    score,
    elapsed: lastSolve || elapsed,
    finished: Boolean(forceFinish || solved >= count),
  };
}

function createMatch(room, puzzles) { return { version: 1, protocol_version: 2, match_id: String(room.room_id), room_id: String(room.room_id), room_code: String(room.room_code), room_seed: Number(room.room_seed), rules: Object.assign({}, rules(), room && room.rules || {}), puzzles: JSON.parse(JSON.stringify(puzzles || [])), players: JSON.parse(JSON.stringify(room.players || [])), events: [] }; }

function calculateResult(playerSolved, playerScore, playerMistakes, playerElapsed, opponent) {
  const opponentSolved = Number(opponent.solved || 0); const opponentScore = Number(opponent.score || 0);
  const outcome = playerSolved > opponentSolved || (playerSolved === opponentSolved && playerScore > opponentScore) ? 'win' : playerSolved < opponentSolved || (playerSolved === opponentSolved && playerScore < opponentScore) ? 'lose' : 'draw';
  return { outcome, player_solved: playerSolved, player_score: playerScore, player_mistakes: playerMistakes, player_elapsed: playerElapsed, opponent_solved: opponentSolved, opponent_score: opponentScore, opponent_elapsed: Number(opponent.elapsed || 0) };
}

module.exports = {
  QUESTION_COUNT,
  TIME_LIMIT,
  MATCHMAKING_TIMEOUT,
  MATCHMAKING_POLL_INTERVAL,
  DAILY_REWARD_MATCH_LIMIT,
  ROOM_STATUS,
  sanitizeRoomCode,
  createRoom,
  createLocalRoom,
  normalizeRoom,
  isRoomReady,
  roomSeedFromCode,
  rules,
  joinRoom,
  generatePuzzles,
  buildOpponentPlan,
  randomBotDifficulty,
  botProfile,
  opponentSnapshot,
  createMatch,
  calculateResult,
};
