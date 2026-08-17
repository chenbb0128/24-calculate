/* Contract checks for Run recovery, avatar upload and logout session hygiene. */
const assert = require('assert');

const memory = Object.create(null);
const requestLog = [];
const uploadLog = [];
let uploadAttempts = 0;
let refreshCalls = 0;
let forceUpload401 = false;

memory.twenty_four_auth = {
  access_token: 'access-old',
  refresh_token: 'refresh-old',
  token_type: 'Bearer',
};

global.wx = {
  getStorageSync(key) { return memory[key]; },
  setStorageSync(key, value) { memory[key] = value; },
  removeStorageSync(key) { delete memory[key]; },
  request(options) {
    requestLog.push(options);
    const path = String(options.url || '').replace(/^https:\/\/calc-api\.pdurl\.cn/, '');
    const ok = (data) => options.success({ statusCode: 200, data: JSON.stringify({ code: 0, message: 'success', data }) });
    if (path === '/api/v1/auth/refresh') {
      refreshCalls += 1;
      ok({ access_token: 'access-new', refresh_token: 'refresh-new', token_type: 'Bearer', expires_in: 900 });
      return;
    }
    if (path.includes('/api/v1/player/campaign/runs/')
      || path.includes('/api/v1/player/daily/runs/')
      || path.includes('/api/v1/player/endless/runs/')) { ok({}); return; }
    if (path === '/api/v1/auth/logout') { ok({}); return; }
    options.fail({ errMsg: `unexpected request ${path}` });
  },
  uploadFile(options) {
    uploadLog.push(options);
    uploadAttempts += 1;
    if (forceUpload401 || uploadAttempts === 1) {
      options.success({ statusCode: 401, data: JSON.stringify({ code: 20003, message: 'Token 已过期', data: null }) });
    } else {
      options.success({ statusCode: 200, data: JSON.stringify({ code: 0, message: 'success', data: { avatar: 'https://cdn.example/avatar.webp' } }) });
    }
    return { abort() {} };
  },
};

function check(condition, message) { assert.ok(condition, message); }

async function run() {
  const modulePath = require.resolve('../src/services/api_client.js');
  delete require.cache[modulePath];
  const api = require('../src/services/api_client.js');

  const campaign = await api.resumeCampaignRun('campaign-1');
  const daily = await api.resumeDailyRun('daily-1');
  const endless = await api.resumeEndlessRun('endless-1');
  check(campaign && daily && endless, 'resume API 没有按标准包装返回 data');
  check(requestLog[0].url.endsWith('/api/v1/player/campaign/runs/campaign-1'), 'campaign resume 路径错误');
  check(requestLog[1].url.endsWith('/api/v1/player/daily/runs/daily-1'), 'daily resume 路径错误');
  check(requestLog[2].url.endsWith('/api/v1/player/endless/runs/endless-1'), 'endless resume 路径错误');

  // Use a fresh mocked client request response for the three resume contracts.
  // The path checks above are the important part; the empty data is accepted by
  // the transport wrapper and never interpreted as a local puzzle.
  check(typeof api.uploadAvatar === 'function', '缺少 uploadAvatar');
  const uploaded = await api.uploadAvatar('wxfile://avatar.jpg');
  check(uploaded.avatar === 'https://cdn.example/avatar.webp', '头像上传没有解析标准响应 data');
  check(uploadLog.length === 2, '头像 401 没有只刷新后重试一次');
  check(uploadLog[0].name === 'file' && uploadLog[1].name === 'file', '头像上传字段名不是 file');
  check(!uploadLog[0].header['Content-Type'] && !uploadLog[0].header['content-type'], '头像上传手动设置了 multipart Content-Type');
  check(refreshCalls === 1, '头像上传 401 刷新次数不是一次');
  check(uploadLog[1].header.Authorization === 'Bearer access-new', '头像重试没有使用刷新后的 token');

  forceUpload401 = true;
  const attemptsBeforeSecondFailure = uploadAttempts;
  const refreshBeforeSecondFailure = refreshCalls;
  let failedAfterOneRetry = false;
  try { await api.uploadAvatar('wxfile://avatar-again.jpg'); } catch (error) { failedAfterOneRetry = true; }
  check(failedAfterOneRetry, '头像连续 401 没有向调用方返回错误');
  check(uploadAttempts === attemptsBeforeSecondFailure + 2, '连续 401 没有严格只重试一次');
  check(refreshCalls === refreshBeforeSecondFailure + 1, '连续 401 触发了超过一次 refresh');

  const logoutResult = await api.logout();
  check(logoutResult && logoutResult.ok === true, 'logout 没有完成标准请求');
  check(!memory.twenty_four_auth, 'logout 后旧 token 仍然存在');
  check(requestLog.some((request) => request.url.endsWith('/api/v1/auth/logout')
    && request.data.refresh_token === 'refresh-new'), 'logout 没有发送当前 refresh token');

  console.log('RUN_AVATAR_LOGOUT_OK');
}

run().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
