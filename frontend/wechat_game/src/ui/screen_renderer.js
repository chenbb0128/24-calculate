/*
 * Canvas screen compositor.
 *
 * GameApp owns the individual page renderers. This module owns the order in
 * which a frame is composed so new pages and overlays do not accidentally
 * render below one another during the WeChat Canvas migration.
 */

const SCREEN_RENDERERS = Object.freeze({
  home: 'drawHome',
  levels: 'drawLevels',
  game: 'drawGame',
  result: 'drawResult',
  friend_matchmaking: 'drawFriendMatchmaking',
  friend_lobby: 'drawFriendLobby',
  shop: 'drawShop',
  achievements: 'drawAchievements',
  leaderboard: 'drawLeaderboard',
  records: 'drawRecords',
});

function renderScreen(app, time) {
  const method = SCREEN_RENDERERS[app.screen];
  if (method && typeof app[method] === 'function') {
    app[method](time);
    return;
  }
  // Fail closed to the home page instead of leaving a blank canvas.
  if (typeof app.drawHome === 'function') app.drawHome(time);
}

function renderTransientLayers(app, time) {
  if (app.screen === 'game' && app.friendCountdownActive) app.drawFriendCountdown();
  if (app.popup) app.drawPopup();
  if (app.hintPopup) app.drawHintPopup();
  if (app.resultHelpPopup) app.drawResultHelpPopup();
  if ((app.screen === 'game' || app.screen === 'result' || app.screen === 'friend_lobby')
    && (app.friendConnectionState === 'reconnecting'
      || app.friendConnectionState === 'reconnect_timeout'
      || app.friendRoomExpired)) {
    app.drawFriendConnectionOverlay();
  }
  app.drawFeedback();
  app.drawTouchEffect(time);
}

function renderFrame(app, time) {
  if (app.renderRecovery) {
    app.drawRuntimeRecovery();
    return;
  }
  app.clear();
  // Hit areas are rebuilt every frame, including sliders and modal buttons.
  app.volumeDragAreas = {};
  app.drawStars(time);
  renderScreen(app, time);
  renderTransientLayers(app, time);
}

function drawFrame(app, time) {
  try {
    renderFrame(app, time);
  } catch (error) {
    app.handleRuntimeError(error, 'draw');
    app.drawRuntimeRecovery();
  }
}

function install(GameApp) {
  GameApp.prototype.draw = function drawCompat(time) {
    return drawFrame(this, time);
  };
}

module.exports = {
  SCREEN_RENDERERS,
  drawFrame,
  renderFrame,
  renderScreen,
  renderTransientLayers,
  install,
};
