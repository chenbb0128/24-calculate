// 微信小游戏音频服务。
// 资源目前可以为空：没有把音频文件打包进项目时，所有方法都会静默降级，
// 不会影响题目、计时和触摸。以后只需要把文件放进 assets/audio，并补上路径即可。
const TRACK_NAMES = ['首页童趣', '关卡欢快'];
const TRACK_SOURCES = [
  'assets/audio/music_home_childlike.wav',
  'assets/audio/music_level_childlike.wav',
];
const SFX_SOURCES = {
  click: 'assets/audio/click.wav',
  card: 'assets/audio/card.wav',
  operator: 'assets/audio/operator.wav',
  merge: 'assets/audio/merge.wav',
  success: 'assets/audio/success.wav',
  error: 'assets/audio/error.wav',
  countdownTick: 'assets/audio/countdown_tick.wav',
  countdownUrgent: 'assets/audio/countdown_urgent.wav',
};

function safeVolume(value, fallback) {
  const number = Number(value);
  return Number.isFinite(number) ? Math.max(0, Math.min(1, number)) : fallback;
}

class AudioService {
  constructor(progress) {
    this.musicContext = null;
    this.sfxContexts = {};
    this.sfxSequence = 0;
    this.maxSfxContexts = 8;
    this.audioFailed = false;
    this.paused = false;
    this.lastCountdownSecond = -1;
    this.musicScene = 'home';
    this.applySettings((progress && progress.audio) || {}, false);
    this.musicTrack = 0;
    this.startMusic();
  }

  applySettings(settings, autoStart = true) {
    const previousMusicEnabled = this.musicEnabled;
    const previousTrack = this.musicTrack;
    this.musicEnabled = settings.music_enabled !== false;
    this.sfxEnabled = settings.sfx_enabled !== false;
    this.musicTrack = this.musicScene === 'game' ? 1 : 0;
    this.musicVolume = safeVolume(settings.music_volume, 0.42);
    this.sfxVolume = safeVolume(settings.sfx_volume, 0.72);
    this.updateContextVolumes();
    if (!autoStart) return;
    if (previousMusicEnabled === true && !this.musicEnabled) this.stopMusic();
    else if (this.musicEnabled && (!previousMusicEnabled || previousTrack !== this.musicTrack)) {
      if (previousTrack !== undefined && previousTrack !== this.musicTrack) this.stopMusic();
      this.startMusic();
    }
  }

  settings() {
    return {
      music_enabled: this.musicEnabled,
      sfx_enabled: this.sfxEnabled,
      music_track: this.musicTrack,
      music_volume: this.musicVolume,
      sfx_volume: this.sfxVolume,
    };
  }

  setMusicScene(scene) {
    const nextScene = scene === 'game' ? 'game' : 'home';
    const nextTrack = nextScene === 'game' ? 1 : 0;
    const changed = this.musicScene !== nextScene || this.musicTrack !== nextTrack;
    this.musicScene = nextScene;
    this.musicTrack = nextTrack;
    if (changed && this.musicEnabled) {
      this.stopMusic();
      this.startMusic();
    } else if (this.musicEnabled) {
      this.startMusic();
    }
  }

  getMusicSceneName() {
    return this.musicScene === 'game' ? '关卡音乐' : '主页面音乐';
  }

  setMusicEnabled(value) {
    this.musicEnabled = Boolean(value);
    if (this.musicEnabled) this.startMusic();
    else this.stopMusic();
  }

  setSfxEnabled(value) { this.sfxEnabled = Boolean(value); }

  setMusicVolume(value) {
    this.musicVolume = safeVolume(value, 0);
    this.updateContextVolumes();
  }

  setSfxVolume(value) {
    this.sfxVolume = safeVolume(value, 0);
    this.updateContextVolumes();
  }

  setMusicTrack(value) {
    this.musicTrack = ((Number(value) % TRACK_NAMES.length) + TRACK_NAMES.length) % TRACK_NAMES.length;
    if (this.musicEnabled) {
      this.stopMusic();
      this.startMusic();
    }
  }

  getMusicTrackName() { return TRACK_NAMES[this.musicTrack] || TRACK_NAMES[0]; }

  canUseWxAudio() {
    // 单个音频加载失败不能关闭其他音效，保证缺少或损坏一个资源时仍能静默降级。
    return typeof wx !== 'undefined' && typeof wx.createInnerAudioContext === 'function';
  }

  createContext(source, loop = false) {
    if (!source || !this.canUseWxAudio()) return null;
    try {
      const context = wx.createInnerAudioContext();
      context.src = source;
      context.loop = loop;
      if (context.onError) context.onError(() => {
        context.__audioFailed = true;
        this.audioFailed = true;
        // 音频资源失败时立即释放上下文。否则真机连续点击会不断积累失效对象，
        // 最终可能造成声音失效或额外内存占用，但不能影响游戏逻辑。
        if (context === this.musicContext) this.stopMusicContext(context);
        else this.releaseSfxContext(context);
      });
      return context;
    } catch (error) {
      this.audioFailed = true;
      return null;
    }
  }

  startMusic() {
    if (!this.musicEnabled || this.paused || !TRACK_SOURCES[this.musicTrack]) return;
    // 微信可能拦截初始化阶段的自动播放；首次点击时再次调用 play() 即可恢复。
    if (this.musicContext && this.musicContext.__audioFailed) this.stopMusicContext(this.musicContext);
    if (this.musicContext) {
      try {
        const result = this.musicContext.play();
        if (result && result.catch) result.catch(() => {});
      } catch (error) { /* 静默降级 */ }
      return;
    }
    const context = this.createContext(TRACK_SOURCES[this.musicTrack], true);
    if (!context) return;
    this.musicContext = context;
    this.updateContextVolumes();
    try {
      const result = this.musicContext.play();
      if (result && result.catch) result.catch(() => {});
    } catch (error) {
      this.stopMusicContext(context);
    }
  }

  stopMusicContext(context) {
    if (!context) return;
    try { if (context.stop) context.stop(); } catch (error) { /* 静默降级 */ }
    try { if (context.destroy) context.destroy(); } catch (error) { /* 静默降级 */ }
    if (this.musicContext === context) this.musicContext = null;
  }

  stopMusic() {
    this.stopMusicContext(this.musicContext);
  }

  releaseSfxContext(context) {
    if (!context) return;
    Object.keys(this.sfxContexts).forEach((key) => {
      if (this.sfxContexts[key] === context) delete this.sfxContexts[key];
    });
    try { if (context.stop) context.stop(); } catch (error) { /* 静默降级 */ }
    try { if (context.destroy) context.destroy(); } catch (error) { /* 静默降级 */ }
  }

  playSfx(name) {
    if (this.paused) return;
    // 即使用户关闭了按键音效，也允许第一次点击解锁背景音乐的播放权限。
    this.startMusic();
    if (!this.sfxEnabled) return;
    const source = SFX_SOURCES[name];
    if (!source) return;
    const activeIDs = Object.keys(this.sfxContexts);
    while (activeIDs.length >= this.maxSfxContexts) {
      const oldestID = activeIDs.shift();
      this.releaseSfxContext(this.sfxContexts[oldestID]);
    }
    const context = this.createContext(source, false);
    if (!context) return;
    const contextId = ++this.sfxSequence;
    this.sfxContexts[contextId] = context;
    const cleanup = () => {
      delete this.sfxContexts[contextId];
      try { if (context.destroy) context.destroy(); } catch (error) { /* 静默降级 */ }
    };
    try {
      context.volume = this.sfxVolume;
      const result = context.play();
      if (context.onEnded) context.onEnded(cleanup);
      if (result && result.catch) result.catch(cleanup);
    } catch (error) {
      cleanup();
    }
  }

  updateContextVolumes() {
    try { if (this.musicContext) this.musicContext.volume = this.musicVolume; } catch (error) { /* 静默降级 */ }
    Object.values(this.sfxContexts).forEach((context) => {
      try { context.volume = this.sfxVolume; } catch (error) { /* 静默降级 */ }
    });
  }

  playClick() { this.playSfx('click'); }
  playCard() { this.playSfx('card'); }
  playOperator() { this.playSfx('operator'); }
  playMerge() { this.playSfx('merge'); }
  playSuccess() { this.playSfx('success'); }
  playError() { this.playSfx('error'); }

  updateCountdown(timeLeft) {
    const second = Math.ceil(Number(timeLeft));
    if (second > 10) this.lastCountdownSecond = -1;
    if (this.sfxEnabled && second <= 10 && second >= 1 && second !== this.lastCountdownSecond) {
      this.lastCountdownSecond = second;
      this.playSfx(second <= 3 ? 'countdownUrgent' : 'countdownTick');
    }
  }

  pause() {
    this.paused = true;
    try { if (this.musicContext) this.musicContext.pause(); } catch (error) { /* 静默降级 */ }
    Object.values(this.sfxContexts).forEach((context) => this.releaseSfxContext(context));
  }

  resume() {
    this.paused = false;
    this.lastCountdownSecond = -1;
    if (this.musicEnabled && this.musicContext) {
      try { this.musicContext.play(); } catch (error) { /* 静默降级 */ }
    } else if (this.musicEnabled) this.startMusic();
  }
}

module.exports = { AudioService, TRACK_NAMES, TRACK_SOURCES, SFX_SOURCES };
