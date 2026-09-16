/* Regression audit for moderated nickname editing. */
const assert = require('assert');
const { GameApp } = require('../src/app.js');
const apiClient = require('../src/services/api_client.js');

function check(condition, message) {
  assert.ok(condition, message);
}

function fakeContext() {
  return new Proxy({
    measureText(value) { return { width: String(value || '').length * 8 }; },
  }, {
    get(target, property) {
      if (property in target) return target[property];
      return () => {};
    },
    set(target, property, value) {
      target[property] = value;
      return true;
    },
  });
}

function profilePopupHarness() {
  const app = Object.create(GameApp.prototype);
  Object.assign(app, {
    width: 750,
    height: 1334,
    visibleHeight: 1334,
    renderScale: 1,
    safeBottom: 24,
    popup: 'profile',
    buttons: [],
    profileNotice: '',
    profileSaving: false,
    progress: { profile: { nickname: '微信玩家', avatar: 'https://thirdwx.qlogo.cn/example/132' } },
    backendAuth: { status: 'ready', user: { nickname: '微信玩家', avatar: 'https://thirdwx.qlogo.cn/example/132' } },
    ctx: fakeContext(),
  });
  app.pageTop = () => 42;
  app.visibleBottom = () => 1280;
  app.modalTop = () => 120;
  app.getPlayerProfile = GameApp.prototype.getPlayerProfile;
  app.drawModalFrame = () => {};
  app.drawGamePanel = () => {};
  app.drawProfileAvatar = () => {};
  app.drawFitText = () => {};
  app.drawNeonButton = (x, y, width, height, label, action, variant, options = {}) => {
    app.buttons.push({ x, y, width, height, label, action, key: options.key || label, disabled: Boolean(options.disabled) });
  };
  return app;
}

function testProfileHasNicknameEditEntry() {
  const app = profilePopupHarness();
  app.drawPopup();
  const editButton = app.buttons.find((button) => button.key === 'profile-edit-name');
  check(editButton && !editButton.disabled, '资料页缺少可用的修改昵称入口');
}

function testNicknameEditOpensInputAndSubmitsValue() {
  let showModalCalls = 0;
  let submittedNickname = '';
  global.wx = {
    showModal(options) {
      showModalCalls += 1;
      options.success({ confirm: true, content: '新昵称' });
    },
  };
  const app = profilePopupHarness();
  app.triggerFeedback = () => {};
  app.saveProfileChanges = (changes) => { submittedNickname = changes.nickname; };
  app.editProfileNickname();
  check(showModalCalls === 1, '点击修改昵称没有打开微信输入框');
  check(submittedNickname === '新昵称', '输入的昵称没有交给后端保存流程');
  delete global.wx;
}

async function testNicknameOnlyUpdateDoesNotResubmitExternalAvatar() {
  const originalUpdate = apiClient.updateProfile;
  let payload = null;
  const app = profilePopupHarness();
  app.isBackendRequired = () => false;
  app.triggerFeedback = () => {};
  apiClient.updateProfile = (data) => {
    payload = data;
    return Promise.resolve({ nickname: '新昵称', avatar: 'https://thirdwx.qlogo.cn/example/132' });
  };
  try {
    app.saveProfileChanges({ nickname: '新昵称' });
    await new Promise((resolve) => setTimeout(resolve, 20));
    check(payload && Object.keys(payload).length === 1 && payload.nickname === '新昵称', '只改昵称时重复提交了旧头像');
  } finally {
    apiClient.updateProfile = originalUpdate;
  }
}

async function testWechatAuthorizationUsesVerifiedSyncFlow() {
  const originalRequestProfile = require('../src/services/platform.js').requestWechatProfile;
  const originalSync = apiClient.syncWechatProfile;
  const originalUpdate = apiClient.updateProfile;
  let syncPayload = null;
  let patchCalled = false;
  const platform = require('../src/services/platform.js');
  platform.requestWechatProfile = () => Promise.resolve({
    nickname: '授权昵称',
    avatar: 'https://thirdwx.qlogo.cn/example/132',
  });
  apiClient.syncWechatProfile = (profile) => {
    syncPayload = profile;
    return Promise.resolve({
      nickname: '授权昵称',
      avatar: 'https://thirdwx.qlogo.cn/example/132',
    });
  };
  apiClient.updateProfile = () => {
    patchCalled = true;
    return Promise.reject(new Error('微信头像不应提交到普通 PATCH'));
  };
  try {
    const app = profilePopupHarness();
    app.triggerFeedback = () => {};
    app.isBackendRequired = () => false;
    app.importWechatProfile();
    await new Promise((resolve) => setTimeout(resolve, 20));
    check(syncPayload && syncPayload.avatar === 'https://thirdwx.qlogo.cn/example/132', '微信授权资料没有走服务端验证同步');
    check(!patchCalled, '微信头像仍然提交到了普通资料 PATCH');
    check(app.progress.profile.avatar === 'https://thirdwx.qlogo.cn/example/132', '授权成功后头像没有保留');
  } finally {
    platform.requestWechatProfile = originalRequestProfile;
    apiClient.syncWechatProfile = originalSync;
    apiClient.updateProfile = originalUpdate;
  }
}

async function testWechatAuthorizationDoesNotResubmitExistingBackendAvatar() {
  const platform = require('../src/services/platform.js');
  const originalRequestProfile = platform.requestWechatProfile;
  const originalSync = apiClient.syncWechatProfile;
  const originalUpdate = apiClient.updateProfile;
  let syncPayload = null;
  let patchCalled = false;
  platform.requestWechatProfile = () => Promise.resolve({ nickname: '仅授权昵称', avatar: '' });
  apiClient.syncWechatProfile = (profile) => {
    syncPayload = profile;
    return Promise.resolve({ nickname: '仅授权昵称', avatar: 'https://calc-api.pdurl.cn/avatars/7/avatar.webp' });
  };
  apiClient.updateProfile = () => {
    patchCalled = true;
    return Promise.reject(new Error('微信授权不应调用普通 PATCH'));
  };
  try {
    const app = profilePopupHarness();
    app.progress.profile.avatar = 'https://calc-api.pdurl.cn/avatars/7/avatar.webp';
    app.backendAuth.user.avatar = app.progress.profile.avatar;
    app.triggerFeedback = () => {};
    app.isBackendRequired = () => false;
    app.importWechatProfile();
    await new Promise((resolve) => setTimeout(resolve, 20));
    check(syncPayload && syncPayload.avatar === '', '微信未返回新头像时重复提交了已有后端头像');
    check(!patchCalled, '微信授权仍然调用了普通资料 PATCH');
    check(app.progress.profile.avatar === 'https://calc-api.pdurl.cn/avatars/7/avatar.webp', '已有后端头像被覆盖');
  } finally {
    platform.requestWechatProfile = originalRequestProfile;
    apiClient.syncWechatProfile = originalSync;
    apiClient.updateProfile = originalUpdate;
  }
}

async function run() {
  testProfileHasNicknameEditEntry();
  testNicknameEditOpensInputAndSubmitsValue();
  await testNicknameOnlyUpdateDoesNotResubmitExternalAvatar();
  await testWechatAuthorizationUsesVerifiedSyncFlow();
  await testWechatAuthorizationDoesNotResubmitExistingBackendAvatar();
  console.log('NICKNAME_EDIT_AUDIT_OK');
}

run().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
