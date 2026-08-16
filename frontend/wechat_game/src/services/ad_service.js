const DAILY_REWARDED_LIMIT = 3;
class AdService {
  constructor() { this.dateKey = ''; this.rewardedUsedToday = 0; this.inFlight = false; }
  configure(saved, currentDateKey) { this.dateKey = currentDateKey; this.rewardedUsedToday = String(saved.date || '') === currentDateKey ? Math.max(0, Math.min(DAILY_REWARDED_LIMIT, Number(saved.rewarded_used || 0))) : 0; }
  isAvailable() { return this.rewardedUsedToday < DAILY_REWARDED_LIMIT; }
  async showRewarded(rewardType, platform) {
    if (this.inFlight || !this.isAvailable()) return false;
    this.inFlight = true;
    try {
      const success = platform && platform.showRewardedAd ? await platform.showRewardedAd(rewardType) : false;
      if (!success) return false;
      this.rewardedUsedToday += 1;
      return true;
    } finally {
      this.inFlight = false;
    }
  }
  usage() { return { date: this.dateKey, rewarded_used: this.rewardedUsedToday, daily_limit: DAILY_REWARDED_LIMIT, remaining: Math.max(0, DAILY_REWARDED_LIMIT - this.rewardedUsedToday) }; }
  showInterstitial() { return false; }
}
module.exports = { AdService, DAILY_REWARDED_LIMIT };
