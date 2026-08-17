function hasWx() {
  return typeof wx !== 'undefined';
}

// 微信不会允许游戏在没有用户操作的情况下静默读取头像和昵称。
// 资料导入必须由玩家点击按钮触发，并兼容不同基础库的接口名称。
function requestWechatProfile() {
  return new Promise((resolve, reject) => {
    if (!hasWx()) {
      reject(new Error('当前环境不支持微信资料授权'));
      return;
    }
    const done = (result) => {
      const info = result && (result.userInfo || result.user_info || result) || {};
      const nickname = String(info.nickName || info.nickname || '').trim().slice(0, 12);
      const avatar = String(info.avatarUrl || info.avatar || '').trim().slice(0, 500);
      if (!nickname && !avatar) {
        reject(new Error('微信未返回头像或昵称'));
        return;
      }
      resolve({ nickname, avatar });
    };
    const fail = (error) => reject(new Error(String(error && (error.errMsg || error.message) || '用户未授权微信资料')));
    try {
      if (typeof wx.getUserProfile === 'function') {
        wx.getUserProfile({ desc: '用于设置游戏头像和昵称', success: done, fail });
        return;
      }
      if (typeof wx.getUserInfo === 'function') {
        wx.getUserInfo({ withCredentials: false, success: done, fail });
        return;
      }
      reject(new Error('当前基础库不支持微信资料授权'));
    } catch (error) {
      reject(error);
    }
  });
}

// 正式广告位尚未配置时保持安全关闭。空值不会创建广告实例，也不会
// 消耗次数或发放奖励；拿到公众平台真实广告位 ID 后只需在这里替换。
const AD_UNIT_IDS = Object.freeze({
  hint: '',
  undo: '',
  continue: '',
  double_reward: '',
});

function share(payload = {}) {
  if (!hasWx() || !wx.shareAppMessage) return Promise.resolve(false);
  return new Promise((resolve) => {
    let settled = false;
    let fallbackTimer = null;
    const done = (success) => {
      if (settled) return;
      settled = true;
      if (fallbackTimer) clearTimeout(fallbackTimer);
      resolve(Boolean(success));
    };
    try {
      wx.shareAppMessage({
        title: payload.title || '来玩《三火算术练习》！',
        query: payload.query || '',
        imageUrl: payload.imageUrl,
        success: () => done(true),
        fail: () => done(false),
      });
      // 部分基础库不会触发 success/fail 回调，分享面板成功打开即可视为成功。
      if (!settled) fallbackTimer = setTimeout(() => done(true), 900);
    } catch (error) {
      done(false);
    }
  });
}

function showRewardedAd(rewardType) {
  return new Promise((resolve) => {
    const adUnitId = AD_UNIT_IDS[String(rewardType || '')] || AD_UNIT_IDS.hint;
    if (!adUnitId || !hasWx() || !wx.createRewardedVideoAd) {
      resolve(false);
      return;
    }
    let ad;
    let settled = false;
    let timeoutTimer = null;
    const done = (success) => {
      if (settled) return;
      settled = true;
      if (timeoutTimer) clearTimeout(timeoutTimer);
      resolve(Boolean(success));
    };
    try {
      ad = wx.createRewardedVideoAd({ adUnitId });
    } catch (error) {
      resolve(false);
      return;
    }
    try {
      if (ad.onClose) ad.onClose((result) => done(result && result.isEnded === true));
      if (ad.onError) ad.onError(() => done(false));
      timeoutTimer = setTimeout(() => done(false), 12000);
      const show = () => {
        const result = ad.show();
        return result && result.catch ? result.catch(() => {
          if (!ad.load) return Promise.reject(new Error('广告加载失败'));
          return ad.load().then(() => ad.show());
        }) : result;
      };
      const result = show();
      if (result && result.catch) result.catch(() => done(false));
    } catch (error) {
      done(false);
    }
  });
}

function roomPayload(roomId, seed) {
  return {
    title: '来和我比一局《三火算术练习》！',
    query: `mode=friend&room=${encodeURIComponent(roomId)}&seed=${Number(seed) || 0}`,
  };
}

module.exports = { AD_UNIT_IDS, hasWx, requestWechatProfile, share, showRewardedAd, roomPayload };
