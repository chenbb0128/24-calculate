/* Sequential campaign unlock rules. The stored unlocked_level is only a
 * cached hint; a level is playable only when the previous level is complete. */

const { safeNumber } = require('../app/app_utils.js');

function isCampaignLevelCompleted(app, index, progress = app.progress) {
  const levelIndex = Math.floor(Number(index));
  if (!Number.isFinite(levelIndex) || levelIndex < 0) return false;
  const record = progress && progress.levels && progress.levels[String(levelIndex)];
  if (!record || typeof record !== 'object') return false;
  return Boolean(record.completed)
    || safeNumber(record.stars, 0) > 0
    || safeNumber(record.best_score, 0) > 0;
}

function isCampaignBlockUnlocked(app, blockIndex) {
  const block = Math.max(0, Math.floor(Number(blockIndex) || 0));
  if (block <= 0) return true;

  const previousBlockStart = (block - 1) * 100;
  const previousBlockEnd = Math.min(200, previousBlockStart + 100);
  for (let index = previousBlockStart; index < previousBlockEnd; index += 1) {
    if (!isCampaignLevelCompleted(app, index)) return false;
  }
  return app.campaignBlockScore(block - 1) >= app.campaignBlockGateScore();
}

function isCampaignLevelUnlocked(app, index) {
  const levelIndex = Math.floor(Number(index));
  if (!Number.isFinite(levelIndex) || levelIndex < 0 || levelIndex >= 200) return false;
  if (levelIndex === 0) return true;
  if (!isCampaignBlockUnlocked(app, Math.floor(levelIndex / 100))) return false;
  return isCampaignLevelCompleted(app, levelIndex - 1);
}

function highestPlayableLevelNumber(app) {
  let highest = 1;
  for (let index = 0; index < 200; index += 1) {
    if (isCampaignLevelUnlocked(app, index)) highest = index + 1;
    else break;
  }
  return highest;
}

function install(GameApp) {
  GameApp.prototype.isCampaignLevelCompleted = function isCampaignLevelCompletedCompat(index, progress = this.progress) {
    return isCampaignLevelCompleted(this, index, progress);
  };
  GameApp.prototype.isCampaignBlockUnlocked = function isCampaignBlockUnlockedCompat(blockIndex) {
    return isCampaignBlockUnlocked(this, blockIndex);
  };
  GameApp.prototype.isCampaignLevelUnlocked = function isCampaignLevelUnlockedCompat(index) {
    return isCampaignLevelUnlocked(this, index);
  };
  GameApp.prototype.highestPlayableLevelNumber = function highestPlayableLevelNumberCompat() {
    return highestPlayableLevelNumber(this);
  };
}

module.exports = {
  isCampaignLevelCompleted,
  isCampaignBlockUnlocked,
  isCampaignLevelUnlocked,
  highestPlayableLevelNumber,
  install,
};
