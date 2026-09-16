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

async function run() {
  testProfileHasNicknameEditEntry();
  testNicknameEditOpensInputAndSubmitsValue();
  await testNicknameOnlyUpdateDoesNotResubmitExternalAvatar();
  console.log('NICKNAME_EDIT_AUDIT_OK');
}

run().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
