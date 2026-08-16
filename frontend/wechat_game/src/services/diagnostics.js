/* Lightweight, client-side diagnostics for real-device acceptance testing. */

function safeNumber(value, fallback = 0) {
  const number = Number(value);
  return Number.isFinite(number) ? number : fallback;
}

function systemInfo() {
  try {
    if (typeof wx !== 'undefined' && wx.getSystemInfoSync) return wx.getSystemInfoSync() || {};
  } catch (error) {
    return {};
  }
  return {};
}

function report(context = {}) {
  const info = systemInfo();
  const storage = context.storage;
  const storageReport = storage && storage.getDiagnostics ? storage.getDiagnostics() : {};
  const audio = context.audio && context.audio.settings ? context.audio.settings() : {};
  const bankStats = context.questionService && context.questionService.campaignBankStats;
  const campaignStats = context.campaignStats || bankStats || {};
  const backend = context.backendAuth || {};
  return {
    device: {
      platform: String(info.platform || 'unknown'),
      brand: String(info.brand || ''),
      model: String(info.model || ''),
      system: String(info.system || ''),
      sdk: String(info.SDKVersion || ''),
      screen: `${safeNumber(info.windowWidth, context.viewportWidth || 0)}×${safeNumber(info.windowHeight, context.viewportHeight || 0)}`,
      pixelRatio: safeNumber(info.pixelRatio, context.dpr || 1),
    },
    storage: storageReport,
    audio: {
      music: audio.music_enabled !== false,
      sfx: audio.sfx_enabled !== false,
      track: safeNumber(audio.music_track, 0),
      musicVolume: safeNumber(audio.music_volume, 0),
      sfxVolume: safeNumber(audio.sfx_volume, 0),
      failed: Boolean(context.audio && context.audio.audioFailed),
    },
    questions: {
      campaignTotal: safeNumber(campaignStats.total, 0),
      campaignVerified: safeNumber(campaignStats.verifiedCount, 0),
      generatorReady: Boolean(context.questionService),
    },
    backend: {
      status: String(backend.status || 'unknown'),
      configured: Boolean(context.backendConfigured),
    },
    runtime: {
      lastError: context.lastRuntimeError || null,
      errorCount: storageReport.errorLogCount || 0,
    },
  };
}

function shortStatus(value, fallback = '未知') {
  if (value === true) return '正常';
  if (value === false) return '异常';
  return String(value || fallback);
}

module.exports = { report, shortStatus };
