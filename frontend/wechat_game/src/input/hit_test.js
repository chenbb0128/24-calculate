/* Hit testing only. This module does not mutate game state. */

function findGameCardAtPoint(app, x, y) {
  if (app.screen !== 'game' || !Array.isArray(app.cards) || !app.cards.length) return -1;
  const layout = app.gameLayout();
  const startX = (app.width - layout.cardWidth * 2 - layout.gapX) / 2;
  // Touch padding must never make two neighboring cards overlap. The old
  // fixed 12px padding was larger than half of the 16px card gap, so a tap
  // near the left edge of the right/bottom card could be claimed by the
  // previous card (most visible on real phones after scaling).
  const gapX = Math.max(0, Number(layout.gapX) || 0);
  const gapY = Math.max(0, Number(layout.gapY) || 0);
  const padding = Math.max(0, Math.min(6, Math.floor(Math.min(gapX, gapY) / 2) - 1));
  for (let index = 0; index < app.cards.length; index += 1) {
    const rect = app.cardRect(index, startX, layout.cardStartY, layout.cardWidth, layout.cardHeight, layout.gapX, layout.gapY);
    if (x >= rect.x - padding && x <= rect.x + rect.width + padding
      && y >= rect.y - padding && y <= rect.y + rect.height + padding) return index;
  }
  return -1;
}

function buttonTouchPadding(button) {
  const key = String(button && button.key || '');
  if (key === 'game-undo' || key === 'game-hint' || key === 'game-reset') return 22;
  if (key === 'game-restart' || key === 'game-leave') return 16;
  if (key.startsWith('operator-')) return 12;
  return 8;
}

function isButtonHit(button, x, y, padding = buttonTouchPadding(button)) {
  if (!button) return false;
  const safePadding = Math.max(0, Number(padding) || 0);
  return x >= button.x - safePadding
    && x <= button.x + button.width + safePadding
    && y >= button.y - safePadding
    && y <= button.y + button.height + safePadding;
}

function findButtonAtPoint(buttons, x, y) {
  const list = Array.isArray(buttons) ? buttons.slice().reverse() : [];
  // 先命中按钮本体，再使用手机触摸缓冲区，避免扩大的触摸区抢走相邻按钮边缘点击。
  return list.find((button) => isButtonHit(button, x, y, 0))
    || list.find((button) => isButtonHit(button, x, y));
}

module.exports = { findGameCardAtPoint, buttonTouchPadding, isButtonHit, findButtonAtPoint };
