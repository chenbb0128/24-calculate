/* Pure helpers shared by the application layer. No wx or game state access. */

const { UI_FONT, FONT_SCALE } = require('../ui/theme.js');

function clamp(value, min, max) {
  return Math.max(min, Math.min(max, value));
}

function safeNumber(value, fallback = 0) {
  const number = Number(value);
  return Number.isFinite(number) ? number : fallback;
}

function previousDateKey(dateKey) {
  const date = new Date(`${dateKey}T00:00:00Z`);
  if (Number.isNaN(date.getTime())) return '';
  date.setUTCDate(date.getUTCDate() - 1);
  return date.toISOString().slice(0, 10);
}

function uiFont(size, weight = 400) {
  return `${weight} ${size}px ${UI_FONT}`;
}

function scaleFont(font, scale = FONT_SCALE) {
  return String(font || '').replace(/(\d+(?:\.\d+)?)px/g, (_, value) => `${Math.round(Number(value) * scale)}px`);
}

function resizeFont(font, size) {
  const source = String(font || '');
  const clamped = Math.max(8, Math.round(size));
  return source.replace(/(\d+(?:\.\d+)?)px/, `${clamped}px`);
}

function uiSafeText(value) {
  return String(value || '')
    .replace(/[\uD800-\uDFFF]/g, '')
    .replace(/[🚩🌟💎⚡🔥♾🚀👑🌱🏆🎨🏅⭐★☆✓♫♪]/g, '')
    .replace(/\s+/g, ' ')
    .trim();
}

module.exports = {
  clamp,
  safeNumber,
  previousDateKey,
  uiFont,
  scaleFont,
  resizeFont,
  uiSafeText,
};
