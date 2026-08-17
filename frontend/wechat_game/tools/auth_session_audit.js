/* Verify that each production app session performs a fresh wx.login. */
const assert = require('assert');

const memory = Object.create(null);
let loginCalls = 0;
let requestLog = [];
let currentCode = 'wx-code-account-b';

memory.twenty_four_auth = {
  access_token: 'old-account-access',
  refresh_token: 'old-account-refresh',
  token_type: 'Bearer',
};

global.wx = {
  getStorageSync(key) { return memory[key]; },
  setStorageSync(key, value) { memory[key] = value; },
  removeStorageSync(key) { delete memory[key]; },
  login(options) {
    loginCalls += 1;
    options.success({ code: currentCode });
  },
  request(options) {
    requestLog.push(options);
    const path = String(options.url || '').replace(/^https:\/\/calc-api\.pdurl\.cn/, '');
    if (path === '/api/v1/auth/wechat-login') {
      const token = `fresh-${currentCode}`;
      options.success({
        statusCode: 200,
        data: JSON.stringify({
          code: 0,
          message: 'success',
          data: { access_token: token, refresh_token: `${token}-refresh`, token_type: 'Bearer', expires_in: 900 },
        }),
      });
      return;
    }
    if (path === '/api/v1/users/me') {
      options.success({
        statusCode: 200,
        data: JSON.stringify({ code: 0, message: 'success', data: { id: 2, nickname: 'Account B', avatar: '' } }),
      });
      return;
    }
    options.fail({ errMsg: `unexpected request ${path}` });
  },
};

function check(condition, message) {
  assert.ok(condition, message);
}

async function run() {
  const modulePath = require.resolve('../src/services/api_client.js');
  delete require.cache[modulePath];
  const apiClient = require('../src/services/api_client.js');

  check(apiClient.requiresFreshWechatLogin(), '正式 API 没有启用冷启动强制微信登录');
  await apiClient.ensureLogin();
  check(loginCalls === 1, '首次正式登录没有调用 wx.login');
  check(memory.twenty_four_auth.access_token === 'fresh-wx-code-account-b', '旧账号令牌没有被新账号令牌替换');
  check(requestLog[0].url.endsWith('/api/v1/auth/wechat-login'), '正式冷启动先请求了旧账号接口');
  check(requestLog[0].data.code === 'wx-code-account-b', '登录没有使用当前 wx.login code');

  await apiClient.ensureLogin();
  check(loginCalls === 1, '同一运行会话重复触发 wx.login');
  check(requestLog.some((request) => request.url.endsWith('/api/v1/users/me')), '登录完成后没有验证当前账号');

  // Simulate a second cold start after the device switches to account C.
  currentCode = 'wx-code-account-c';
  delete require.cache[modulePath];
  const reloadedClient = require('../src/services/api_client.js');
  await reloadedClient.ensureLogin();
  check(loginCalls === 2, '应用重启后没有重新调用 wx.login');
  check(memory.twenty_four_auth.access_token === 'fresh-wx-code-account-c', '账号 C 仍复用了账号 B 令牌');

  console.log('AUTH_SESSION_OK');
}

run().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
