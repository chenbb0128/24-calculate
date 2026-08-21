/* Friend room entry and matchmaking fallback audit. */
const assert = require('assert');
const friend = require('../src/core/friend_match_service.js');
const api = require('../src/services/api_client.js');
const { GameApp } = require('../src/app.js');

function check(condition, message) {
  assert.ok(condition, message);
}

check(friend.MATCHMAKING_TIMEOUT === 20, 'matchmaking timeout must be 20 seconds');
check(friend.sanitizeRoomCode('a12-3456789') === '123456', 'room code sanitizer must keep the first six digits');
check(friend.botProfile('easy').difficulty === 'easy', 'easy bot profile is invalid');
check(friend.botProfile('hard').difficulty === 'hard', 'hard bot profile is invalid');

const entry = Object.create(GameApp.prototype);
Object.assign(entry, {
  popup: '',
  friendRoomFromInvite: false,
  friendLobbyView: 'entry',
  friendLocalFallback: false,
  friendRoomBackendStatus: 'idle',
  friendRoomBackendLoading: false,
  friendRoomLastPollAt: 0,
  friendRoom: null,
  friendRules: friend.rules(),
  friendMatch: null,
  friendMatchProgress: null,
  friendRoomInput: '',
  friendInputKeyboardActive: false,
  friendMatchmaking: null,
  friendMatchmakingRunId: 0,
  friendMatchmakingRequestInFlight: false,
  friendMatchmakingLastPollAt: 0,
  friendMatchmakingLocal: true,
  friendBotDifficulty: 'standard',
  friendBotName: '',
  progress: {},
  // An explicit null auth object represents the offline/local test harness.
  // A configured backend with status=offline must not enter the bot fallback.
  backendAuth: null,
  screen: 'home',
  triggerFeedback() {},
  syncFriendRoomWithBackend() {},
});
entry.showFriendLobby();
check(entry.screen === 'friend_lobby' && entry.friendLobbyView === 'entry', 'friend entry page did not open');
entry.friendRoomInput = '628391';
entry.joinFriendRoomEntry();
check(entry.friendLobbyView === 'room' && entry.friendRoom.room_code === '628391', 'manual room join did not open the requested room');

entry.startFriendMatchmaking();
check(entry.screen === 'friend_matchmaking', 'quick matchmaking page did not open');
// The client deliberately jitters the local fallback window within the
// configured bounds. Advance past the actual deadline instead of assuming
// the nominal 20-second midpoint, otherwise this audit is time-dependent.
entry.friendMatchmaking.startedAt = Date.now() - (entry.friendMatchmaking.botFallbackAfter + 1) * 1000;
entry.updateFriendMatchmaking();
check(entry.friendMatchmaking.status === 'bot_ready', 'matchmaking timeout did not prepare a bot');
check(entry.friendRoom && entry.friendRoom.players[1].bot === true, 'bot opponent was not added to the room');
check(['easy', 'standard', 'hard'].includes(entry.friendBotDifficulty), 'bot difficulty is invalid');

const countdown = Object.create(GameApp.prototype);
Object.assign(countdown, {
  screen: 'game',
  friendCountdownActive: true,
  friendCountdownUntil: Date.now() - 1,
  friendCountdownLastNumber: 3,
  gamePaused: true,
  triggerFeedback() {},
});
countdown.updateFriendCountdown();
check(countdown.friendCountdownActive === false && countdown.gamePaused === false, 'friend countdown did not finish cleanly');

check(typeof api.joinMatchmaking === 'function', 'join matchmaking API is missing');
check(typeof api.getMatchmakingStatus === 'function', 'matchmaking status API is missing');
check(typeof api.cancelMatchmaking === 'function', 'cancel matchmaking API is missing');

console.log('MATCHMAKING_OK');

async function networkRecoveryAudit() {
  const originalGetFriendRoom = api.getFriendRoom;
  const makeClient = () => Object.assign(Object.create(GameApp.prototype), {
    mode: 'friend',
    screen: 'game',
    friendLocalFallback: false,
    friendRoomBackendStatus: 'ready',
    backendAuth: { status: 'ready' },
    friendRoom: { room_code: '628391', room_seed: 123, status: 'running', players: [] },
    friendRules: friend.rules(),
    friendRoomLastPollAt: 0,
    friendProgressLastPollAt: 0,
    friendRoomRequestInFlight: false,
    friendReconnectRequestInFlight: false,
    friendReconnectStartedAt: Date.now(),
    friendReconnectDeadline: Date.now() + 15000,
    friendReconnectNextAt: 0,
    friendConnectionState: 'reconnecting',
    friendRoomExpired: false,
    friendRoomError: '',
    triggerFeedback() {},
  });

  try {
    const recovered = makeClient();
    api.getFriendRoom = () => Promise.resolve({ room_code: '628391', room_seed: 123, status: 'running', players: [] });
    recovered.updateFriendReconnect();
    await new Promise((resolve) => setTimeout(resolve, 0));
    check(recovered.friendConnectionState === 'connected', 'successful reconnect did not restore connected state');
    check(recovered.friendRoomExpired === false, 'successful reconnect incorrectly expired the room');

    const expired = makeClient();
    api.getFriendRoom = () => Promise.reject(Object.assign(new Error('room expired'), { statusCode: 410 }));
    expired.updateFriendReconnect();
    await new Promise((resolve) => setTimeout(resolve, 0));
    check(expired.friendRoomExpired === true, '410 room response did not mark room expired');
    check(expired.friendConnectionState === 'expired', 'expired room did not enter terminal state');
  } finally {
    api.getFriendRoom = originalGetFriendRoom;
  }
  console.log('FRIEND_NETWORK_OK');
}

networkRecoveryAudit().catch((error) => {
  console.error(error && error.stack || error);
  process.exitCode = 1;
});
