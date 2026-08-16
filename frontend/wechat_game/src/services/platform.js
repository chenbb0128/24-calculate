function hasWx() {
  return typeof wx !== 'undefined';
}

// 正式广告 ID 只在这里配置。占位 ID 保持静默，不会在开发者工具中反复报错。
const AD_UNIT_IDS = {
  hint: 'adunit-placeholder',
  undo: 'adunit-placeholder',
  continue: 'adunit-placeholder',
  double_reward: 'adunit-placeholder',
};

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
    if (adUnitId === 'adunit-placeholder' || !hasWx() || !wx.createRewardedVideoAd) {
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

function submitLeaderboard(mode, score, metadata) {
  // 正式版在这里调用云函数，客户端分数只作为候选值。
  const payload = {
    contractVersion: 1,
    mode,
    score: Math.max(0, Number(score || 0)),
    metadata: metadata || {},
    clientAuthoritative: false,
    submitTo: 'server_or_cloud_function',
  };
  if (hasWx() && wx.cloud && wx.cloud.callFunction) {
    try {
      const request = wx.cloud.callFunction({ name: 'submitLeaderboard', data: payload });
      // 开发者工具、未初始化云环境或测试桩可能返回 undefined；排行榜失败不能阻断结算。
      if (!request || typeof request.then !== 'function') return Promise.resolve(payload);
      return request.then((result) => result && result.result ? result.result : payload).catch(() => payload);
    } catch (error) {
      return Promise.resolve(payload);
    }
  }
  return Promise.resolve(payload);
}

function submitFriendMatch(matchPayload) {
  const protocolPayload = matchPayload && Number(matchPayload.protocol_version) >= 2;
  const payload = protocolPayload
    ? { ...matchPayload, client_authoritative: false, submitTo: 'server_or_cloud_function' }
    : {
      contractVersion: 1,
      match: matchPayload || {},
      clientAuthoritative: false,
      submitTo: 'server_or_cloud_function',
    };
  if (hasWx() && wx.cloud && wx.cloud.callFunction) {
    try {
      const request = wx.cloud.callFunction({ name: 'submitFriendMatch', data: payload });
      if (!request || typeof request.then !== 'function') return Promise.resolve(payload);
      return request.then((result) => result && result.result ? result.result : payload).catch(() => payload);
    } catch (error) {
      return Promise.resolve(payload);
    }
  }
  return Promise.resolve(payload);
}

function roomPayload(roomId, seed) {
  return {
    title: '来和我比一局《三火算术练习》！',
    query: `mode=friend&room=${encodeURIComponent(roomId)}&seed=${Number(seed) || 0}`,
  };
}

module.exports = { AD_UNIT_IDS, hasWx, share, showRewardedAd, submitLeaderboard, submitFriendMatch, roomPayload };
