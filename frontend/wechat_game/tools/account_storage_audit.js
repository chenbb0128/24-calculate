/* Acceptance checks for per-account local storage isolation. */
const assert = require('assert');

const memory = Object.create(null);
global.wx = {
  getStorageSync(key) { return memory[key]; },
  setStorageSync(key, value) { memory[key] = value; },
  removeStorageSync(key) { delete memory[key]; },
};

const storage = require('../src/services/storage.js');

function check(condition, message) {
  assert.ok(condition, message);
}

storage.clearAccount();
storage.save(storage.normalize({ coins: 11, unlocked_level: 2, tutorial_seen: true }));
check(memory[storage.KEY] && memory[storage.KEY].coins === 11, '匿名旧存档没有写入');

const accountA = storage.setAccount('wechat-account-a');
check(storage.getActiveAccountID() === 'wechat-account-a', '账号 A 没有激活');
check(accountA.coins === 11 && accountA.unlocked_level === 2, '旧存档没有只迁移给第一个账号');
const keysA = storage.accountStorageKeys('wechat-account-a');
check(memory[keysA.primary] && memory[keysA.primary].coins === 11, '账号 A 存档没有使用专属键');

accountA.coins = 99;
storage.save(accountA);
const accountB = storage.setAccount('wechat-account-b');
check(accountB.coins === 0 && accountB.unlocked_level === 0, '账号 B 读取了账号 A 的存档');
accountB.coins = 7;
storage.save(accountB);

storage.setAccount('server-authority-test');
const authoritative = storage.mergeServerProgress(
  storage.normalize({ coins: 999999, unlocked_level: 99, levels: { '98': { stars: 3, best_score: 100 } } }),
  { coins: 4, unlocked_level: 1, last_level: 0, levels: {}, owned_skins: ['classic'], equipped_skin: 'classic' },
  { authoritative: true },
);
check(authoritative.coins === 4 && authoritative.unlocked_level === 1, 'server authoritative sync retained local coins or unlocks');
check(Object.keys(authoritative.levels).length === 0, 'server authoritative sync retained local level records');
check(Object.keys(authoritative.leaderboards).length === 0, 'server authoritative sync retained local leaderboard snapshot');
storage.setAccount('wechat-account-b');
storage.save(accountB);

check(storage.setAccount('wechat-account-a').coins === 99, '切回账号 A 后进度不正确');
check(storage.setAccount('wechat-account-b').coins === 7, '切回账号 B 后进度不正确');

storage.appendErrorLog('account-b-only', 'isolated');
const keysB = storage.accountStorageKeys('wechat-account-b');
check(Array.isArray(memory[keysB.errorLog]) && memory[keysB.errorLog].length === 1, '账号 B 错误日志没有隔离');
storage.setAccount('wechat-account-a');
check(storage.getErrorLogs().length === 0, '账号 A 读取了账号 B 的错误日志');

// Once the legacy global record has been claimed, a later account must not
// inherit a newly written shared record.
storage.clearAccount();
memory[storage.KEY] = storage.normalize({ coins: 123 });
check(storage.setAccount('wechat-account-c').coins === 0, '后续账号错误继承了共享旧存档');

const storagePath = require.resolve('../src/services/storage.js');
delete require.cache[storagePath];
const reloadedStorage = require('../src/services/storage.js');
check(reloadedStorage.getActiveAccountID() === 'wechat-account-c', '应用重启后没有恢复当前账号命名空间');
check(reloadedStorage.load().coins === 0, '应用重启后账号隔离失效');

console.log('ACCOUNT_STORAGE_OK');
