/* Friend battle contract and local fallback audit. */
const assert = require('assert');
const puzzle = require('../src/core/puzzle_generator.js');
const friend = require('../src/core/friend_match_service.js');
const matchData = require('../src/core/match_data.js');
const questionCore = require('../src/core/question_service.js');
const api = require('../src/services/api_client.js');
const share = require('../src/services/share_service.js');
const { GameApp } = require('../src/app.js');
const storage = require('../src/services/storage.js');

function check(condition, message) {
  assert.ok(condition, message);
}

check(api.isConfigured() === false, 'loopback backend must stay disabled until an HTTPS domain is configured');

const roomA = friend.createLocalRoom('628391');
const roomB = friend.createLocalRoom('628391');
check(friend.isRoomReady(roomA), 'local fallback room is not ready');
check(roomA.room_code === roomB.room_code && roomA.room_seed === roomB.room_seed, 'same room code must resolve to the same local seed');
check(roomA.players.length >= 2, 'local fallback room must include a simulated opponent');

const serviceA = questionCore.createQuestionService({});
const serviceB = questionCore.createQuestionService({});

const localApp = Object.create(GameApp.prototype);
Object.assign(localApp, {
  friendRoom: roomA,
  friendRoomBackendStatus: 'local',
  friendLocalFallback: true,
  friendRules: friend.rules(),
  friendRoomFromInvite: false,
  friendMatch: null,
  friendMatchContract: null,
  friendAttempts: [],
  friendMatchProgress: null,
  friendProgressLastPollAt: 0,
  friendProgressRequestInFlight: false,
  friendProgressLastSentKey: '',
  friendMatchResolutionApplied: false,
  friendStartedAt: 0,
  friendPlayerSolved: 0,
  mode: 'campaign',
  currentQuestion: 0,
  puzzles: [],
  score: 0,
  combo: 0,
  mistakes: 0,
  maxCombo: 0,
  progress: storage.normalize({}),
  dailyChallenge: null,
  popup: '',
  hintPopup: null,
  resultHelpPopup: false,
  autoNextToken: 0,
  renderRecovery: false,
  audio: { playSuccess() {}, playError() {}, playMerge() {}, playCard() {}, playOperator() {} },
  questionService: serviceA,
});
localApp.startFriend();
check(localApp.screen === 'game' && localApp.puzzles.length === friend.QUESTION_COUNT, 'local friend mode did not start immediately');

const puzzlesA = serviceA.getFriendQuestions(roomA.room_seed, { count: friend.QUESTION_COUNT });
const puzzlesB = serviceB.getFriendQuestions(roomB.room_seed, { count: friend.QUESTION_COUNT });
check(puzzlesA.length === friend.QUESTION_COUNT, 'friend question count is incomplete');
check(JSON.stringify(puzzlesA.map((item) => item.numbers)) === JSON.stringify(puzzlesB.map((item) => item.numbers)), 'same room seed must produce the same questions');

const contract = matchData.createMatchContract(roomA, puzzlesA);
check(matchData.validateMatchContract(contract), 'friend match contract is invalid');
const attempt = matchData.createAttemptRecord(contract, puzzlesA[0], {
  question_index: 0,
  elapsed_ms: 12000,
  solved: true,
  mistakes: 0,
  score: 80,
  score_delta: 80,
  event_id: 'event-1',
  operations: puzzlesA[0].solutionSteps,
  solution_steps: puzzlesA[0].solutionSteps,
});
check(matchData.validateAttempt(attempt, contract), 'friend attempt with operation proof is invalid');
check(matchData.validateResultSubmission(matchData.createResultSubmission(contract, [attempt], { outcome: 'win' })), 'friend result submission is invalid');

const invite = share.createFriendRoomPayload(roomA);
check(!Object.prototype.hasOwnProperty.call(invite, 'path'), 'native game share must not use a pages/index path');
check(share.parseLaunchParams({ mode: 'friend', room: roomA.room_code }).room_code === roomA.room_code, 'friend invite query cannot be parsed');

const opponentPlan = friend.buildOpponentPlan(roomA.room_seed);
check(Array.isArray(opponentPlan.solve_times) && opponentPlan.solve_times.length === friend.QUESTION_COUNT, 'opponent solve plan is incomplete');
check(opponentPlan.solve_times[0] >= 5, 'opponent must spend time thinking about the first question');
for (let index = 1; index < opponentPlan.solve_times.length; index += 1) {
  check(opponentPlan.solve_times[index] > opponentPlan.solve_times[index - 1], 'opponent questions must be solved sequentially');
}
const beforeFirst = friend.opponentSnapshot(opponentPlan, Math.max(0, opponentPlan.solve_times[0] - 0.1), friend.QUESTION_COUNT, false);
const afterFirst = friend.opponentSnapshot(opponentPlan, opponentPlan.solve_times[0] + 0.1, friend.QUESTION_COUNT, false);
check(beforeFirst.solved === 0, 'opponent solved the first question before its thinking time elapsed');
check(afterFirst.solved === 1, 'opponent did not advance one question at a time');
const opponent = friend.opponentSnapshot(opponentPlan, 30, friend.QUESTION_COUNT, false);
check(opponent.finished === (opponent.solved === friend.QUESTION_COUNT) && opponent.solved >= 0, 'local opponent snapshot is invalid');

console.log('FRIEND_MATCH_OK');
