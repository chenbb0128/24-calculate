/* Structural smoke test for the frontend module boundaries. */
const assert = require('assert');
const fs = require('fs');
const path = require('path');

const root = path.resolve(__dirname, '..');
const appSource = fs.readFileSync(path.join(root, 'src/app/game_app.js'), 'utf8');
const screenRenderer = require(path.join(root, 'src/ui/screen_renderer.js'));
const pageLayout = require(path.join(root, 'src/ui/page_layout.js'));

assert.ok(appSource.includes("require('../ui/screen_renderer.js')"), 'screen renderer is not connected');
assert.ok(appSource.includes("require('../ui/page_layout.js')"), 'page layout is not connected');
assert.strictEqual(typeof screenRenderer.renderFrame, 'function');
assert.strictEqual(typeof screenRenderer.install, 'function');
assert.strictEqual(typeof pageLayout.gameLayout, 'function');

const expectedScreens = [
  'home', 'levels', 'game', 'result', 'friend_matchmaking',
  'friend_lobby', 'shop', 'achievements', 'leaderboard', 'records',
];
expectedScreens.forEach((screen) => {
  assert.ok(screenRenderer.SCREEN_RENDERERS[screen], `missing renderer for ${screen}`);
});

const calls = [];
const fakeApp = {
  screen: 'home',
  drawHome: () => calls.push('home'),
  drawStars: () => calls.push('stars'),
  clear: () => calls.push('clear'),
  drawFeedback: () => calls.push('feedback'),
  drawTouchEffect: () => calls.push('touch'),
  volumeDragAreas: null,
};
screenRenderer.renderFrame(fakeApp, 1);
assert.deepStrictEqual(calls, ['clear', 'stars', 'home', 'feedback', 'touch']);

const layoutApp = {
  renderScale: 1,
  renderOffsetX: 0,
  renderOffsetY: 0,
  menuButton: null,
  safeTop: 24,
  safeBottom: 24,
  visibleHeight: 1334,
  height: 1334,
  mode: 'campaign',
  cards: [1, 2, 3, 4],
};
const layout = pageLayout.gameLayout(layoutApp);
assert.ok(layout.cardStartY > layout.contentY, 'game cards must be below the header content');
assert.ok(layout.bottomY > layout.actionY, 'game controls must be ordered vertically');
assert.ok(pageLayout.cardRect(0, 10, 20, 100, 50, 8, 6).x === 10);

console.log('MODULE_AUDIT_OK');
