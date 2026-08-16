const storage = require('./storage.js');

const ACHIEVEMENTS = [
  { id: 'first_clear', title: '迈出第一步', description: '完成第一个闯关关卡', icon: '🚩', reward: 30 },
  { id: 'three_star', title: '三星闪耀', description: '首次获得三星评价', icon: '🌟', reward: 50 },
  { id: 'perfect_clear', title: '完美解题', description: '无错误、无提示完成关卡', icon: '💎', reward: 80 },
  { id: 'combo_5', title: '连击小能手', description: '单局连击达到 5', icon: '⚡', reward: 30 },
  { id: 'combo_10', title: '连击大师', description: '单局连击达到 10', icon: '🔥', reward: 80 },
  { id: 'endless_5', title: '无尽热身', description: '无尽模式连续答对 5 题', icon: '♾', reward: 30 },
  { id: 'endless_10', title: '无尽进阶', description: '无尽模式连续答对 10 题', icon: '🚀', reward: 60 },
  { id: 'endless_30', title: '无尽传说', description: '无尽模式连续答对 30 题', icon: '👑', reward: 150 },
  { id: 'daily_3', title: '三日坚持', description: '连续完成每日挑战 3 天', icon: '🌱', reward: 60 },
  { id: 'daily_7', title: '一周坚持', description: '连续完成每日挑战 7 天', icon: '🏆', reward: 120 },
  { id: 'friend_first_win', title: '好友对决首胜', description: '在好友对战中获得首胜', icon: '⚔️', reward: 40 },
  { id: 'skin_unlock', title: '换个心情', description: '兑换第一个主题皮肤', icon: '🎨', reward: 40 },
];

function ensureProgress(progress) {
  if (!progress.achievements || typeof progress.achievements !== 'object') progress.achievements = { unlocked: {}, claimed: {} };
  if (!progress.achievements.unlocked || typeof progress.achievements.unlocked !== 'object') progress.achievements.unlocked = {};
  if (!progress.achievements.claimed || typeof progress.achievements.claimed !== 'object') progress.achievements.claimed = {};
}
function all() { return JSON.parse(JSON.stringify(ACHIEVEMENTS)); }
function getAchievement(id) { return all().find((item) => item.id === id) || {}; }
function isUnlocked(progress, id) { ensureProgress(progress); return Boolean(progress.achievements.unlocked[id]); }
function unlock(progress, id) {
  const achievement = getAchievement(id); if (!achievement.id) return {};
  ensureProgress(progress); if (progress.achievements.unlocked[id]) return {};
  progress.achievements.unlocked[id] = true; progress.achievements.claimed[id] = true;
  storage.addCoins(progress, achievement.reward || 0);
  return { ...achievement, newly_unlocked: true };
}
function unlockMany(progress, ids) { return (ids || []).map((id) => unlock(progress, String(id))).filter((item) => item.id); }
function unlockedCount(progress) { ensureProgress(progress); return ACHIEVEMENTS.filter((item) => progress.achievements.unlocked[item.id]).length; }
function nextHint(progress) { ensureProgress(progress); return all().find((item) => !progress.achievements.unlocked[item.id]) || {}; }
function formatUnlocks(unlocks) { return unlocks && unlocks.length ? ['🏅 新成就解锁！', ...unlocks.map((item) => `${item.icon || '🏅'} ${item.title}  +${item.reward || 0} 金币`)].join('\n') : ''; }

module.exports = { all, getAchievement, ensureProgress, isUnlocked, unlock, unlockMany, unlockedCount, nextHint, formatUnlocks };
