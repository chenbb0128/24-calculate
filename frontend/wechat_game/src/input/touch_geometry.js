/* Coordinate normalization shared by touch handlers. */

const { safeNumber } = require('../app/app_utils.js');

function touchCoordinate(touch, axis) {
  if (!touch) return 0;
  const names = axis === 'x'
    ? ['clientX', 'pageX', 'x', 'canvasX']
    : ['clientY', 'pageY', 'y', 'canvasY'];
  for (const name of names) {
    if (touch[name] !== undefined && touch[name] !== null) return safeNumber(touch[name]);
  }
  return 0;
}

function touchPointCandidates(app, touch) {
  const rawX = touchCoordinate(touch, 'x');
  const rawY = touchCoordinate(touch, 'y');
  const scale = Math.max(0.0001, safeNumber(app.renderScale, 1));
  const offsetX = safeNumber(app.renderOffsetX, 0);
  const offsetY = safeNumber(app.renderOffsetY, 0);
  const candidates = [
    { x: (rawX - offsetX) / scale, y: (rawY - offsetY) / scale },
    { x: rawX, y: rawY },
  ];
  const dpr = Math.max(1, safeNumber(app.dpr, 1));
  if (dpr !== 1) candidates.push({ x: (rawX / dpr - offsetX) / scale, y: (rawY / dpr - offsetY) / scale });
  const result = [];
  candidates.forEach((point) => {
    if (!result.some((item) => Math.abs(item.x - point.x) < 0.01 && Math.abs(item.y - point.y) < 0.01)) result.push(point);
  });
  return result;
}

module.exports = { touchCoordinate, touchPointCandidates };
