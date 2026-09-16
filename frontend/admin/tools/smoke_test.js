const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {
  filterUsers,
  paginateUsers,
  setUserStatus,
  serializeUserState,
  restoreUserState,
  STORAGE_KEY,
  SEED_USERS
} = require('../app');

const html = fs.readFileSync(path.join(__dirname, '..', 'index.html'), 'utf8');
for (const id of ['app', 'sidebar', 'statCards', 'searchInput', 'statusFilter', 'platformFilter',
  'userTableBody', 'pagination', 'detailDrawer', 'confirmDialog', 'toastRegion']) {
  assert.match(html, new RegExp(`id=["']${id}["']`));
}
assert.match(html, /app\.js/);
assert.match(html, /styles\.css/);

const users = [
  { id: 'U1001', username: 'wx_1001', nickname: '晴天小猫', platform: '微信', status: 'active' },
  { id: 'U1002', username: 'tap_1002', nickname: '夜航星', platform: 'TapTap', status: 'disabled' },
  { id: 'U1003', username: 'wx_1003', nickname: '小火花', platform: '微信', status: 'active' }
];

assert.equal(filterUsers(users, { query: '晴天', status: 'all', platform: 'all' }).length, 1);
assert.equal(filterUsers(users, { query: 'U1002', status: 'all', platform: 'all' })[0].id, 'U1002');
assert.equal(filterUsers(users, { query: '', status: 'active', platform: '微信' }).length, 2);
assert.equal(filterUsers(users, { query: '不存在', status: 'all', platform: 'all' }).length, 0);

const page = paginateUsers(users, 2, 2);
assert.deepEqual(page, { page: 2, pageSize: 2, total: 3, totalPages: 2, items: [users[2]] });
assert.equal(paginateUsers(users, 9, 2).page, 2);

const changed = setUserStatus(users, ['U1001', 'U1003'], 'disabled');
assert.equal(changed[0].status, 'disabled');
assert.equal(changed[1].status, 'disabled');
assert.equal(changed[2].status, 'disabled');
assert.equal(users[0].status, 'active');

const unsupportedStatus = setUserStatus(users, ['U1001'], 'pending');
assert.deepEqual(unsupportedStatus.map((user) => user.status), ['active', 'disabled', 'active']);
assert.deepEqual(JSON.parse(serializeUserState(unsupportedStatus)), [
  { id: 'U1001', status: 'active' },
  { id: 'U1002', status: 'disabled' },
  { id: 'U1003', status: 'active' }
]);
assert.deepEqual(JSON.parse(serializeUserState([{ id: 'U1001', status: 'pending' }])), []);

const restored = restoreUserState(serializeUserState(changed), users);
assert.deepEqual(JSON.parse(serializeUserState(changed)), [
  { id: 'U1001', status: 'disabled' },
  { id: 'U1002', status: 'disabled' },
  { id: 'U1003', status: 'disabled' }
]);
assert.deepEqual(restored.map((user) => user.status), ['disabled', 'disabled', 'disabled']);
assert.equal(restoreUserState('{broken', users)[0].status, 'active');
assert.equal(restoreUserState(JSON.stringify([{ id: 'UNKNOWN', status: 'disabled' }]), users)[0].status, 'active');
assert.equal(restoreUserState(JSON.stringify([{ id: 'U1001', status: 'pending' }]), users)[0].status, 'active');

assert.equal(STORAGE_KEY, 'sanhuo.admin.users.v1');
assert.equal(SEED_USERS.length, 18);
assert.deepEqual([...new Set(SEED_USERS.map((user) => user.platform))].sort(), ['TapTap', '微信']);
assert.deepEqual([...new Set(SEED_USERS.map((user) => user.status))].sort(), ['active', 'disabled']);
for (const user of SEED_USERS) {
  assert.ok(user.id && user.username && user.nickname && user.avatar && user.platform && user.status);
  assert.ok(user.createdAt && user.lastActiveAt);
  assert.equal(typeof user.sessions, 'number');
  assert.equal(typeof user.totalMatches, 'number');
}

console.log('PASS: admin data core smoke test');
