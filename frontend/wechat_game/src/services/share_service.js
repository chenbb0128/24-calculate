function safeRoomCode(value) {
  return String(value || '').replace(/\D/g, '').slice(-6);
}

function createFriendRoomPayload(room) {
  const roomCode = safeRoomCode(room && room.room_code);
  // 原生小游戏没有 pages/index/index 页面，正式分享只使用 query。
  return {
    title: '来和我挑战《三火算术练习》！',
    query: `mode=friend&room=${encodeURIComponent(roomCode)}`,
    room_code: roomCode,
    supported: true,
  };
}

function buildInviteText(room) {
  return `来和我比一局《三火算术练习》！房间号：${safeRoomCode(room && room.room_code)}`;
}

function parseLaunchParams(params = {}) {
  return {
    mode: String(params.mode || ''),
    room_code: safeRoomCode(params.room || params.room_code),
    // seed 仅兼容旧分享链接；正式模式由服务端根据 room_code 返回。
    room_seed: Number(params.seed || params.room_seed || 0),
  };
}

function createMatchResultPayload(result, room) {
  const outcome = String(result && result.outcome || 'draw');
  const text = outcome === 'win' ? '赢下了对战' : outcome === 'draw' ? '打成平局' : '完成了挑战';
  const roomCode = safeRoomCode(room && room.room_code);
  return {
    title: `我在《三火算术练习》中${text}！`,
    query: `mode=friend&room=${encodeURIComponent(roomCode)}`,
    room_code: roomCode,
    score: Number(result && result.player_score || 0),
    outcome,
    supported: true,
  };
}

function buildResultCardText(result = {}) {
  const outcome = String(result.outcome || 'draw');
  const text = outcome === 'win' ? '击败好友' : outcome === 'draw' ? '和好友打平' : '完成对战';
  return `我在《三火算术练习》中${text}！\n答对 ${Number(result.player_solved || 0)} 题 · ${Number(result.player_score || 0)} 分 · 用时 ${Number(result.player_elapsed || 0).toFixed(1)} 秒`;
}

module.exports = { createFriendRoomPayload, buildInviteText, parseLaunchParams, createMatchResultPayload, buildResultCardText };
