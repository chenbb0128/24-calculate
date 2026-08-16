/* Development-only acceptance checks for storage recovery and diagnostics. */
const assert = require('assert');

const memory = Object.create(null);
global.wx = {
  getStorageSync(key) { return memory[key]; },
  setStorageSync(key, value) { memory[key] = value; },
};

const storage = require('../src/services/storage.js');

function check(condition, message) {
  assert.ok(condition, message);
}

const first = storage.normalize({ coins: 10, unlocked_level: 3, tutorial_seen: true });
storage.save(first);
const second = storage.normalize({ coins: 25, unlocked_level: 8, tutorial_seen: true });
storage.save(second);
check(memory[storage.KEY].coins === 25, '主存档没有保存最新数据');
check(memory[storage.BACKUP_KEY].coins === 10, '备用存档没有保留上一个正常版本');

memory[storage.KEY] = '{broken-json';
const recovered = storage.load();
const loadInfo = storage.getLastLoadInfo();
check(recovered.coins === 10 && recovered.unlocked_level === 3, '主存档损坏后没有恢复备用存档');
check(loadInfo.recovered === true && loadInfo.backupValid === true, '恢复状态没有正确记录');

storage.appendErrorLog('stability-audit', new Error('diagnostic-test'), { mode: 'test', screen: 'home' });
check(storage.getErrorLogs().some((entry) => entry.stage === 'stability-audit'), '错误日志没有写入');
storage.clearErrorLogs();
check(storage.getErrorLogs().length === 0, '错误日志没有清除');

console.log('STABILITY_OK');
