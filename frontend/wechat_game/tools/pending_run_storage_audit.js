/* Verify pending Run checkpoints use the authenticated account namespace. */
const assert = require('assert');
const memory = Object.create(null);
global.wx = {
  getStorageSync(key) { return memory[key]; },
  setStorageSync(key, value) { memory[key] = value; },
  removeStorageSync(key) { delete memory[key]; },
};

const storage = require('../src/services/storage.js');
function check(condition, message) { assert.ok(condition, message); }

storage.clearAccount();
storage.setAccount('pending-account-a');
storage.savePendingRun({ mode: 'campaign', run_id: 'run-a', level_id: 4, question_index: 1, score: 32, attempts: [{ question_index: 0, solved: true }] });
check(storage.getPendingRun('campaign').run_id === 'run-a', '账号 A pending run 没有保存');

storage.setAccount('pending-account-b');
check(!storage.getPendingRun('campaign'), '账号 B 读取了账号 A 的 pending run');
storage.savePendingRun({ mode: 'daily', run_id: 'run-b', date_key: '2026-08-18', attempts: [] });
storage.setAccount('pending-account-a');
check(storage.getPendingRun('campaign').run_id === 'run-a', '切回账号 A 后 pending run 丢失');
check(!storage.getPendingRun('daily'), '账号 A 读取了账号 B 的 pending run');

storage.clearPendingRun('campaign', 'run-a');
check(!storage.getPendingRun('campaign'), '已清理的 pending run 仍存在');
console.log('PENDING_RUN_STORAGE_OK');
