const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {
  filterUsers,
  paginateUsers,
  setUserStatus,
  getStats,
  getInitials,
  getStatusChangeTargets,
  mapServerUser,
  moderationLabel
} = require('../app');

const root = path.join(__dirname, '..');
const html = fs.readFileSync(path.join(root, 'index.html'), 'utf8');
const appSource = fs.readFileSync(path.join(root, 'app.js'), 'utf8');
const apiSource = fs.readFileSync(path.join(root, 'api.js'), 'utf8');

for (const id of ['app', 'sidebar', 'statCards', 'searchInput', 'statusFilter', 'platformFilter',
  'userTableBody', 'pagination', 'detailDrawer', 'confirmDialog', 'toastRegion', 'loginGate',
  'loginForm', 'loginUsername', 'loginPassword']) {
  assert.match(html, new RegExp(`id=["']${id}["']`));
}
assert.match(html, /<script src=["']api\.js["']><\/script>\s*<script src=["']app\.js["']>/);
assert.match(html, /<title>三火算术 · 运营后台<\/title>/);
assert.match(html, /id=["']detailDrawer["'][^>]*aria-describedby=["']detailIdentitySummary["']/);
assert.match(html, /id=["']detailNicknameModeration["']/);
assert.match(html, /id=["']detailAvatarModeration["']/);
assert.match(html, /id=["']app["'][^>]*\binert\b/);

assert.doesNotMatch(appSource, /localStorage/);
assert.doesNotMatch(appSource, /SEED_USERS/);
assert.doesNotMatch(appSource, /innerHTML|\beval\s*\(/);
assert.doesNotMatch(apiSource, /localStorage|innerHTML|\beval\s*\(/);
assert.match(appSource, /window\.AdminApi\.createAdminApi/);
assert.match(appSource, /state\.api\.listUsers/);
assert.match(appSource, /state\.api\.getUser/);
assert.match(appSource, /state\.api\.disableUser/);
assert.match(appSource, /state\.api\.enableUser/);
assert.match(appSource, /returnFocus\.focus/);
assert.match(appSource, /event\.key !== 'Escape'/);
assert.match(appSource, /function closeConfirmation[\s\S]*dialog\.close/);
assert.match(appSource, /function openConfirmation[\s\S]*dialog\.showModal/);
assert.match(appSource, /function showLogin[\s\S]*app\.setAttribute\('inert', ''\)/);
assert.match(appSource, /function showDashboard[\s\S]*app\.removeAttribute\('inert'\)/);
assert.match(appSource, /replaceChildren\(createStatusTag\(user\.status\)\)/);

const users = [
  { id: '1', username: 'wx_1', nickname: '晴天小猫', platform: '微信', status: 'active' },
  { id: '2', username: 'tap_2', nickname: '夜航星', platform: 'TapTap', status: 'disabled' },
  { id: '3', username: 'wx_3', nickname: '小火花', platform: '微信', status: 'active' }
];
assert.deepEqual(getStats(users), { total: 3, newToday: 0, active: 2, disabled: 1 });
assert.equal(getInitials('晴天小猫'), '晴猫');
assert.equal(filterUsers(users, { query: '晴天', status: 'all', platform: 'all' }).length, 1);
assert.equal(filterUsers(users, { query: 'tap_2', status: 'all', platform: 'all' })[0].id, '2');
assert.deepEqual(paginateUsers(users, 2, 2), { page: 2, pageSize: 2, total: 3, totalPages: 2, items: [users[2]] });
assert.deepEqual(getStatusChangeTargets(users, ['2', '1', '1', 'unknown'], 'disabled'), ['1']);
assert.deepEqual(setUserStatus(users, ['1'], 'disabled').map((user) => user.status), ['disabled', 'disabled', 'active']);
assert.deepEqual(mapServerUser({ id: 7, username: 'operator', nickname: '安全昵称', avatar: '', platform: 'password', status: 0,
  created_at: '2026-09-16T00:00:00Z', updated_at: '2026-09-16T01:00:00Z' }), {
  id: '7', username: 'operator', nickname: '安全昵称', avatar: '', platform: '账号', status: 'disabled',
  createdAt: '2026-09-16T00:00:00Z', lastActiveAt: '2026-09-16T01:00:00Z',
  nicknameModerationStatus: undefined, avatarModerationStatus: undefined
});
assert.equal(moderationLabel('rejected'), '未通过');

console.log('PASS: admin API dashboard smoke test');
