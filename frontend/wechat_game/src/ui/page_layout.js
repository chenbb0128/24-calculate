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
  const infoY = statsY + 94;
  const hasInfo = app.mode === 'friend' || app.mode === 'daily';
  const baseContentY = hasInfo ? infoY + 82 : statsY + 94;
  const compact = app.visibleHeight < 1500;
  const ultraCompact = app.visibleHeight < 1320;
  const cardWidth = compact ? 318 : 326;
  const cardHeight = ultraCompact ? 100 : compact ? 112 : 126;
  const gapX = compact ? 16 : 18;
  const gapY = ultraCompact ? 10 : compact ? 12 : 16;
  const cardOffset = ultraCompact ? 226 : compact ? (hasInfo ? 226 : 230) : 244;
  const operatorHeight = ultraCompact ? 58 : compact ? 64 : 72;
  const actionHeight = ultraCompact ? 54 : compact ? 56 : 62;
  const bottomButtonHeight = compact ? 58 : 64;
  const cardStartY = baseContentY + cardOffset;
  const cardRows = Math.max(1, Math.ceil(Math.max(1, app.cards ? app.cards.length : 4) / 2));
  const opTitleY = cardStartY + cardHeight * cardRows + gapY + (ultraCompact ? 22 : compact ? 24 : 30);
  const actionY = opTitleY + (ultraCompact ? 84 : compact ? 88 : 110);
  const actionButtonTop = actionY + 22;
  const bottomGap = ultraCompact ? 22 : compact ? 26 : 34;
  const bottomY = actionButtonTop + actionHeight + bottomGap;
  const footerGap = ultraCompact ? 16 : compact ? 20 : 26;
  const footerHeight = compact ? 52 : 62;
  const footerY = bottomY + bottomButtonHeight + footerGap;
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
