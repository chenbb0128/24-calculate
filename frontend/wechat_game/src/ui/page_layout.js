/* Shared coordinate calculations for Canvas pages. */

const { clamp, safeNumber } = require('../app/app_utils.js');

function pageTop(app) {
  const scale = Math.max(0.001, app.renderScale);
  const menuBottom = app.menuButton
    ? (safeNumber(app.menuButton.bottom, 0) - app.renderOffsetY) / scale
    : 0;
  const safeTop = safeNumber(app.safeTop, 24) / scale;
  const targetTop = menuBottom > 0 ? menuBottom + 14 : safeTop + 10;
  return Math.round(clamp(targetTop, 42, 210));
}

function screenContentTop(app, gap = 76) {
  return pageTop(app) + gap;
}

function contentHeight(app) {
  return Math.max(1280, app.height);
}

function visibleBottom(app, padding = 24) {
  const safeBottom = safeNumber(app.safeBottom, 24) / Math.max(0.001, app.renderScale);
  return Math.max(720, app.visibleHeight - safeBottom - padding);
}

function modalTop(app, modalHeight, preferred = null) {
  const topLimit = pageTop(app) + 76;
  const bottomLimit = visibleBottom(app, 18);
  const centered = preferred === null ? (app.visibleHeight - modalHeight) / 2 : preferred;
  return Math.round(clamp(centered, topLimit, Math.max(topLimit, bottomLimit - modalHeight)));
}

function gameLayout(app) {
  const headerY = pageTop(app);
  const statsY = headerY + 76;
  const infoY = headerY + 170;
  const hasInfo = app.mode === 'friend' || app.mode === 'daily';
  const baseContentY = headerY + 86;
  const compact = app.visibleHeight < 1500;
  // 320px-class devices have a shorter logical viewport because the canvas is
  // scaled from 750px. Use a compact board there too, otherwise the footer
  // can sit below the safe area even though every button is technically valid.
  const tiny = safeNumber(app.viewportWidth, 0) > 0
    ? app.viewportWidth <= 360
    : app.renderScale < 0.46;
  const ultraCompact = app.visibleHeight < 1320 || tiny;
  // Reserve a stable breathing space below the question panel. This keeps
  // the card hitboxes away from the progress panel on small phones.
  const cardWidth = compact ? 326 : 338;
  const cardHeight = tiny ? 190 : compact ? (hasInfo ? 190 : 240) : (hasInfo ? 220 : 260);
  const gapX = compact ? 14 : 16;
  const gapY = tiny ? 12 : compact ? 16 : 18;
  const cardOffset = 226;
  const operatorHeight = tiny ? 54 : compact ? (hasInfo ? 70 : 76) : (hasInfo ? 76 : 88);
  const actionHeight = tiny ? 50 : compact ? 52 : 58;
  const bottomButtonHeight = compact ? 54 : 58;
  const cardStartY = baseContentY + cardOffset;
  const cardRows = Math.max(1, Math.ceil(Math.max(1, app.cards ? app.cards.length : 4) / 2));
  const opTitleY = cardStartY + cardHeight * cardRows + gapY + 22;
  const actionY = opTitleY + operatorHeight + 18;
  const actionButtonTop = actionY + 22;
  const bottomGap = tiny ? 10 : compact ? 14 : 18;
  const bottomY = actionButtonTop + actionHeight + bottomGap;
  const footerY = bottomY + bottomButtonHeight;
  const footerHeight = 0;
  return {
    headerY, statsY, infoY, contentY: baseContentY, bottomY, footerY,
    footerHeight, cardWidth, cardHeight, gapX, gapY, cardStartY, cardRows,
    opTitleY, actionY, operatorHeight, actionHeight, bottomButtonHeight,
  };
}

function cardRect(index, startX, startY, cardWidth, cardHeight, gapX, gapY) {
  const col = index % 2;
  const row = Math.floor(index / 2);
  return { x: startX + col * (cardWidth + gapX), y: startY + row * (cardHeight + gapY), width: cardWidth, height: cardHeight };
}

function install(GameApp) {
  GameApp.prototype.pageTop = function pageTopCompat() { return pageTop(this); };
  GameApp.prototype.screenContentTop = function screenContentTopCompat(gap = 76) { return screenContentTop(this, gap); };
  GameApp.prototype.contentHeight = function contentHeightCompat() { return contentHeight(this); };
  GameApp.prototype.visibleBottom = function visibleBottomCompat(padding = 24) { return visibleBottom(this, padding); };
  GameApp.prototype.modalTop = function modalTopCompat(modalHeight, preferred = null) { return modalTop(this, modalHeight, preferred); };
  GameApp.prototype.gameLayout = function gameLayoutCompat() { return gameLayout(this); };
  GameApp.prototype.cardRect = function cardRectCompat(index, startX, startY, cardWidth, cardHeight, gapX, gapY) {
    return cardRect(index, startX, startY, cardWidth, cardHeight, gapX, gapY);
  };
}

module.exports = {
  pageTop,
  screenContentTop,
  contentHeight,
  visibleBottom,
  modalTop,
  gameLayout,
  cardRect,
  install,
};
