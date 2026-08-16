const storage = require('./storage.js');

const TASKS = {
  campaign_clear: { title: '完成 1 个闯关关卡', target: 1, reward: 15 },
  endless_questions: { title: '无尽模式答对 5 题', target: 5, reward: 25 },
  combo: { title: '单局连击达到 5', target: 5, reward: 20 },
};
const WEEKLY_TASKS = {
  weekly_campaign: { title: '本周完成 5 个闯关关卡', target: 5, reward: 45 },
  weekly_daily: { title: '本周完成 3 次每日挑战', target: 3, reward: 35 },
  weekly_endless: { title: '本周无尽模式答对 20 题', target: 20, reward: 50 },
  weekly_friend: { title: '本周好友对战获胜 2 次', target: 2, reward: 40 },
};

function dateParts(dateKey) { const d = new Date(`${dateKey}T00:00:00Z`); return Number.isNaN(d.getTime()) ? 0 : Math.floor(d.getTime() / 604800000); }
function ensureDay(progress, dateKey) { if (!progress.tasks || progress.tasks.date !== dateKey) progress.tasks = { date: dateKey, values: {}, claimed: {} }; ensureWeek(progress, dateKey); }
function ensureWeek(progress, dateKey) { const week = dateParts(dateKey); if (!progress.weekly_tasks || progress.weekly_tasks.week !== String(week)) progress.weekly_tasks = { week: String(week), values: {}, claimed: {} }; }
function recordInternal(progress, taskId, amount, dateKey, useMax) {
  if (!TASKS[taskId]) return { reward: 0, completed: false };
  ensureDay(progress, dateKey); const task = TASKS[taskId]; const old = Number(progress.tasks.values[taskId] || 0);
  const next = Math.min(task.target, useMax ? Math.max(old, Number(amount || 0)) : old + Number(amount || 0)); progress.tasks.values[taskId] = next;
  const completed = next >= task.target; let reward = 0;
  if (completed && !progress.tasks.claimed[taskId]) { progress.tasks.claimed[taskId] = true; reward = task.reward; storage.addCoins(progress, reward); }
  return { task_id: taskId, title: task.title, value: next, target: task.target, reward, completed };
}
function record(progress, taskId, amount, dateKey) { return recordInternal(progress, taskId, amount, dateKey, false); }
function recordMax(progress, taskId, amount, dateKey) { return recordInternal(progress, taskId, amount, dateKey, true); }
function snapshot(progress, dateKey) {
  ensureDay(progress, dateKey);
  return Object.fromEntries(Object.entries(TASKS).map(([id, task]) => [id, {
    ...task,
    value: Math.min(task.target, Number(progress.tasks.values[id] || 0)),
    claimed: Boolean(progress.tasks.claimed[id]),
  }]));
}
function recordWeekly(progress, taskId, amount, dateKey) {
  if (!WEEKLY_TASKS[taskId]) return { reward: 0, completed: false };
  ensureWeek(progress, dateKey); const task = WEEKLY_TASKS[taskId]; const next = Math.min(task.target, Number(progress.weekly_tasks.values[taskId] || 0) + Number(amount || 0)); progress.weekly_tasks.values[taskId] = next;
  const completed = next >= task.target; let reward = 0;
  if (completed && !progress.weekly_tasks.claimed[taskId]) { progress.weekly_tasks.claimed[taskId] = true; reward = task.reward; storage.addCoins(progress, reward); }
  return { task_id: taskId, title: task.title, value: next, target: task.target, reward, completed };
}
function weeklySnapshot(progress, dateKey) {
  ensureWeek(progress, dateKey);
  return Object.fromEntries(Object.entries(WEEKLY_TASKS).map(([id, task]) => [id, {
    ...task,
    value: Math.min(task.target, Number(progress.weekly_tasks.values[id] || 0)),
    claimed: Boolean(progress.weekly_tasks.claimed[id]),
  }]));
}

module.exports = { TASKS, WEEKLY_TASKS, ensureDay, ensureWeek, record, recordMax, snapshot, recordWeekly, weeklySnapshot };
