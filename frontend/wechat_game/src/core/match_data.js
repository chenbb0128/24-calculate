const PROTOCOL_VERSION = 2;
const CLOUD_FUNCTIONS = {
  createRoom: 'createFriendRoom',
  joinRoom: 'joinFriendRoom',
  submitMatch: 'submitFriendMatch',
};

function clone(value) {
  return JSON.parse(JSON.stringify(value === undefined ? null : value));
}

function safeNumber(value, fallback = 0) {
  const number = Number(value);
  return Number.isFinite(number) ? number : fallback;
}

function safeInteger(value, fallback = 0) {
  return Math.floor(safeNumber(value, fallback));
}

function hashText(text) {
  let state = 2166136261;
  String(text).split('').forEach((character) => {
    state ^= character.charCodeAt(0);
    state = Math.imul(state, 16777619);
  });
  return (state >>> 0).toString(16).padStart(8, '0');
}

function numberKey(numbers) {
  return (Array.isArray(numbers) ? numbers : []).map((value) => safeInteger(value)).sort((a, b) => a - b).join(',');
}

function questionFingerprint(roomSeed, puzzles, rules = {}) {
  const list = (Array.isArray(puzzles) ? puzzles : []).map((puzzle, index) => ({
    index,
    puzzle_id: String(puzzle && (puzzle.puzzleId || puzzle.puzzle_id) || `Q${index + 1}`),
    numbers: numberKey(puzzle && puzzle.numbers),
  }));
  return hashText(JSON.stringify({ room_seed: safeInteger(roomSeed), rules: clone(rules), puzzles: list }));
}

function createRoomContract(room) {
  const source = room || {};
  return {
    protocol_version: PROTOCOL_VERSION,
    type: 'friend_room',
    room_id: String(source.room_id || ''),
    room_code: String(source.room_code || ''),
    match_id: String(source.match_id || source.matchId || source.round_id || source.roundId || ''),
    room_seed: Math.abs(safeInteger(source.room_seed)),
    status: String(source.status || 'waiting'),
    owner: clone(source.owner || {}),
    rules: clone(source.rules || {}),
    created_at: safeInteger(source.created_at, 0),
    client_authoritative: false,
  };
}

function createMatch(matchId, puzzles, rules) {
  return {
    version: 1,
    protocol_version: PROTOCOL_VERSION,
    match_id: matchId,
    rules: clone(rules || {}),
    puzzles: clone(puzzles || []),
    players: {},
    events: [],
  };
}

function createMatchContract(room, puzzles) {
  const roomContract = createRoomContract(room);
  const list = Array.isArray(puzzles) ? puzzles : [];
  return {
    protocol_version: PROTOCOL_VERSION,
    type: 'friend_match',
    match_id: String(roomContract.match_id || roomContract.room_id || `friend-${roomContract.room_code}`),
    room_id: roomContract.room_id,
    room_code: roomContract.room_code,
    room_seed: roomContract.room_seed,
    question_count: list.length,
    question_hash: questionFingerprint(roomContract.room_seed, list, roomContract.rules),
    puzzle_ids: list.map((puzzle, index) => String(puzzle && (puzzle.puzzleId || puzzle.puzzle_id) || `Q${index + 1}`)),
    rules: clone(roomContract.rules),
    state: 'running',
    client_authoritative: false,
  };
}

function createAttempt(puzzleId, elapsedMs, solved, mistakes, score, details = {}) {
  // Keep the original positional signature for existing callers.
  return {
    protocol_version: PROTOCOL_VERSION,
    puzzle_id: String(puzzleId || ''),
    question_index: safeInteger(details.question_index, -1),
    elapsed_ms: Math.max(0, safeInteger(elapsedMs)),
    solved: Boolean(solved),
    mistakes: Math.max(0, safeInteger(mistakes)),
    score: Math.max(0, safeNumber(score)),
    score_delta: Math.max(0, safeNumber(details.score_delta)),
    room_seed: Math.abs(safeInteger(details.room_seed)),
    question_hash: String(details.question_hash || ''),
    event_id: String(details.event_id || ''),
    operations: clone(Array.isArray(details.operations) ? details.operations : []),
    solution_steps: clone(details.solution_steps || []),
    client_authoritative: false,
  };
}

function createAttemptRecord(match, puzzle, details = {}) {
  const contract = match || {};
  const record = puzzle || {};
  return createAttempt(
    record.puzzleId || record.puzzle_id || `Q${safeInteger(details.question_index) + 1}`,
    details.elapsed_ms,
    details.solved,
    details.mistakes,
    details.score,
    {
      question_index: details.question_index,
      score_delta: details.score_delta,
      room_seed: contract.room_seed,
      question_hash: contract.question_hash,
      event_id: details.event_id,
      operations: details.operations,
      solution_steps: details.solution_steps,
    },
  );
}

function createResultSubmission(match, attempts, summary = {}) {
  const contract = match || {};
  return {
    protocol_version: PROTOCOL_VERSION,
    action: CLOUD_FUNCTIONS.submitMatch,
    match_id: String(contract.match_id || ''),
    room_id: String(contract.room_id || ''),
    room_seed: Math.abs(safeInteger(contract.room_seed)),
    ranked: Boolean(contract.ranked),
    season_id: String(contract.season_id || ''),
    question_count: Math.max(0, safeInteger(contract.question_count)),
    question_hash: String(contract.question_hash || ''),
    puzzle_ids: clone(contract.puzzle_ids || []),
    attempts: (Array.isArray(attempts) ? attempts : []).map(clone),
    summary: clone(summary || {}),
    validation: { replay_from_seed: true, verify_solution: true, verify_timing: true },
    client_authoritative: false,
  };
}

function validateRoomContract(room) {
  return Boolean(room
    && Number(room.protocol_version) === PROTOCOL_VERSION
    && String(room.room_id || '')
    && String(room.room_code || '')
    && safeInteger(room.room_seed) > 0
    && room.rules && typeof room.rules === 'object');
}

function validateMatchContract(match) {
  return Boolean(match
    && Number(match.protocol_version) === PROTOCOL_VERSION
    && String(match.match_id || '')
    && String(match.room_id || '')
    && safeInteger(match.room_seed) > 0
    && safeInteger(match.question_count) > 0
    && /^[0-9a-f]{8}$/.test(String(match.question_hash || ''))
    && Array.isArray(match.puzzle_ids)
    && match.puzzle_ids.length === safeInteger(match.question_count));
}

function validateAttempt(attempt, match) {
  if (!attempt || Number(attempt.protocol_version) !== PROTOCOL_VERSION || !validateMatchContract(match)) return false;
  const index = safeInteger(attempt.question_index, -1);
  return index >= 0
    && index < safeInteger(match.question_count)
    && String(attempt.puzzle_id || '') === String(match.puzzle_ids[index] || '')
    && safeInteger(attempt.room_seed) === safeInteger(match.room_seed)
    && String(attempt.question_hash || '') === String(match.question_hash)
    && safeInteger(attempt.elapsed_ms, -1) >= 0
    && safeInteger(attempt.mistakes, -1) >= 0
    && safeNumber(attempt.score, -1) >= 0
    && (!attempt.operations || Array.isArray(attempt.operations));
}

function validateResultSubmission(submission) {
  if (!submission || Number(submission.protocol_version) !== PROTOCOL_VERSION || !validateMatchContract(submission)) return false;
  if (!Array.isArray(submission.attempts)) return false;
  const indexes = new Set();
  return submission.attempts.every((attempt) => {
    const index = safeInteger(attempt && attempt.question_index, -1);
    if (indexes.has(index) || !validateAttempt(attempt, submission)) return false;
    indexes.add(index);
    return true;
  });
}

function buildCloudRequest(action, data = {}) {
  return {
    protocol_version: PROTOCOL_VERSION,
    action: String(action || ''),
    data: clone(data),
    client_authoritative: false,
  };
}

module.exports = {
  PROTOCOL_VERSION,
  CLOUD_FUNCTIONS,
  createRoomContract,
  createMatch,
  createMatchContract,
  createAttempt,
  createAttemptRecord,
  createResultSubmission,
  questionFingerprint,
  validateRoomContract,
  validateMatchContract,
  validateAttempt,
  validateResultSubmission,
  buildCloudRequest,
};
