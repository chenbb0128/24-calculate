const assert = require('node:assert/strict');

const { createAdminApi } = require('../api');

function createStorage() {
  const values = new Map();
  return {
    getItem(key) { return values.has(key) ? values.get(key) : null; },
    setItem(key, value) { values.set(key, String(value)); },
    removeItem(key) { values.delete(key); },
    has(key) { return values.has(key); }
  };
}

function response(status, body) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body
  };
}

function createFetch(responses) {
  const calls = [];
  return {
    calls,
    fetch: async (url, options) => {
      calls.push({ url, options });
      const next = responses.shift();
      if (!next) throw new Error('unexpected request');
      return typeof next === 'function' ? next(url, options) : next;
    }
  };
}

async function run() {
  const accessA = 'fixture-access-a';
  const refreshA = 'fixture-refresh-a';
  const accessB = 'fixture-access-b';
  const refreshB = 'fixture-refresh-b';
  const storage = createStorage();
  const loginFetch = createFetch([
    response(200, { code: 0, message: 'success', data: {
      access_token: accessA, refresh_token: refreshA, token_type: 'Bearer', expires_in: 300
    } }),
    response(200, { code: 0, message: 'success', data: {
      items: [], page: 2, page_size: 25, total: 0,
      stats: { total: 0, new_today: 0, active: 0, disabled: 0 }
    } }),
    response(200, { code: 0, message: 'success', data: { id: 42, status: 0, updated_at: '2026-09-16T00:00:00Z' } }),
    response(200, { code: 0, message: 'success', data: { id: 42, status: 1, updated_at: '2026-09-16T00:00:00Z' } }),
    response(200, { code: 0, message: 'success', data: { revoked: true } })
  ]);
  const api = createAdminApi({ baseUrl: 'https://admin.example.test/api/v1/admin', fetchImpl: loginFetch.fetch, storage });

  const login = await api.login('operator', 'sample-only');
  assert.equal(login.access_token, accessA);
  assert.equal(loginFetch.calls[0].url, 'https://admin.example.test/api/v1/admin/auth/login');
  assert.equal(loginFetch.calls[0].options.method, 'POST');
  assert.equal(loginFetch.calls[0].options.body, JSON.stringify({ username: 'operator', password: 'sample-only' }));

  const list = await api.listUsers({ query: '晴天', status: 'active', platform: 'wechat', page: 2, pageSize: 25 });
  assert.equal(list.page_size, 25);
  assert.equal(loginFetch.calls[1].url, 'https://admin.example.test/api/v1/admin/users?q=%E6%99%B4%E5%A4%A9&status=active&platform=wechat&page=2&page_size=25');
  assert.equal(loginFetch.calls[1].options.method, 'GET');
  assert.equal(loginFetch.calls[1].options.headers.Authorization, `Bearer ${accessA}`);

  await api.disableUser(42);
  await api.enableUser('42');
  assert.equal(loginFetch.calls[2].url, 'https://admin.example.test/api/v1/admin/users/42/disable');
  assert.equal(loginFetch.calls[2].options.method, 'POST');
  assert.equal(loginFetch.calls[3].url, 'https://admin.example.test/api/v1/admin/users/42/enable');
  assert.equal(loginFetch.calls[3].options.method, 'POST');
  assert.throws(() => api.getUser('42x'), /用户 ID 无效/);

  await api.logout();
  assert.equal(loginFetch.calls[4].url, 'https://admin.example.test/api/v1/admin/auth/logout');
  assert.equal(loginFetch.calls[4].options.body, JSON.stringify({ refresh_token: refreshA }));
  assert.equal(api.hasAccessToken(), false);

  const retryStorage = createStorage();
  retryStorage.setItem('sanhuo.admin.access_token.v1', accessA);
  retryStorage.setItem('sanhuo.admin.refresh_token.v1', refreshA);
  const retryFetch = createFetch([
    response(401, { code: 20001, message: 'expired', data: null }),
    response(200, { code: 0, message: 'success', data: {
      access_token: accessB, refresh_token: refreshB, token_type: 'Bearer', expires_in: 300
    } }),
    response(200, { code: 0, message: 'success', data: { id: 42, username: 'operator' } })
  ]);
  const retryApi = createAdminApi({ baseUrl: '/api/v1/admin', fetchImpl: retryFetch.fetch, storage: retryStorage });
  const detail = await retryApi.getUser(42);
  assert.equal(detail.id, 42);
  assert.equal(retryFetch.calls.length, 3);
  assert.equal(retryFetch.calls[0].options.headers.Authorization, `Bearer ${accessA}`);
  assert.equal(retryFetch.calls[1].url, '/api/v1/admin/auth/refresh');
  assert.equal(retryFetch.calls[1].options.body, JSON.stringify({ refresh_token: refreshA }));
  assert.equal(retryFetch.calls[2].url, '/api/v1/admin/users/42');
  assert.equal(retryFetch.calls[2].options.headers.Authorization, `Bearer ${accessB}`);

  const clearStorage = createStorage();
  clearStorage.setItem('sanhuo.admin.access_token.v1', accessA);
  clearStorage.setItem('sanhuo.admin.refresh_token.v1', refreshA);
  const clearFetch = createFetch([
    response(401, { code: 20001, message: 'expired', data: null }),
    response(200, { code: 0, message: 'success', data: {
      access_token: accessB, refresh_token: refreshB, token_type: 'Bearer', expires_in: 300
    } }),
    response(401, { code: 20001, message: 'still expired', data: null })
  ]);
  const clearApi = createAdminApi({ fetchImpl: clearFetch.fetch, storage: clearStorage });
  await assert.rejects(() => clearApi.listUsers({}), (error) => error.name === 'AdminApiError'
    && error.status === 401 && error.code === 20001 && error.isAuthError === true);
  assert.equal(clearFetch.calls.length, 3);
  assert.equal(clearStorage.has('sanhuo.admin.access_token.v1'), false);
  assert.equal(clearStorage.has('sanhuo.admin.refresh_token.v1'), false);

  const moderationFetch = createFetch([
    response(400, { code: 40001, business_code: 'NICKNAME_REJECTED', message: 'ignored', data: null }),
    response(400, { code: 40002, business_code: 'AVATAR_REJECTED', message: 'ignored', data: null }),
    response(400, { code: 40003, business_code: 'MODERATION_PENDING', message: 'ignored', data: null }),
    response(400, { code: 40004, business_code: 'NICKNAME_EDIT_DISABLED', message: 'ignored', data: null })
  ]);
  const moderationApi = createAdminApi({ fetchImpl: moderationFetch.fetch, storage: createStorage() });
  for (const [businessCode, message] of [
    ['NICKNAME_REJECTED', '昵称未通过审核'],
    ['AVATAR_REJECTED', '头像未通过审核'],
    ['MODERATION_PENDING', '资料审核尚未完成'],
    ['NICKNAME_EDIT_DISABLED', '暂不支持修改昵称']
  ]) {
    await assert.rejects(() => moderationApi.login('operator', 'sample-only'), (error) => error.businessCode === businessCode
      && error.message === message && Number.isInteger(error.code));
  }
}

run().then(() => {
  console.log('PASS: admin API adapter contract');
}).catch((error) => {
  console.error(error && error.stack ? error.stack : error);
  process.exitCode = 1;
});
