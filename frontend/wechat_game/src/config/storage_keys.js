/* Centralized persistence contract. Do not rename keys without a migration. */

const KEY = 'twenty_four_progress';
const BACKUP_KEY = 'twenty_four_progress_backup';
const ERROR_LOG_KEY = 'twenty_four_runtime_logs';
// The unscoped keys above are kept only for anonymous/offline play. They are
// never copied into an authenticated account because device ownership cannot
// prove which backend user created the data.
const ACCOUNT_KEY_PREFIX = 'twenty_four_progress_account_';
const ACCOUNT_BACKUP_KEY_PREFIX = 'twenty_four_progress_account_backup_';
const ACCOUNT_ERROR_LOG_PREFIX = 'twenty_four_runtime_logs_account_';
const ACTIVE_ACCOUNT_KEY = 'twenty_four_active_account';
const COIN_CAP = 999999;
const STORAGE_VERSION = 13;

const LEGACY_KEYS = Object.freeze({
  coins: 'coins',
  unlockedLevel: 'unlockedLevel',
  dailyDone: 'dailyDone',
});

module.exports = Object.freeze({
  KEY,
  BACKUP_KEY,
  ERROR_LOG_KEY,
  ACCOUNT_KEY_PREFIX,
  ACCOUNT_BACKUP_KEY_PREFIX,
  ACCOUNT_ERROR_LOG_PREFIX,
  ACTIVE_ACCOUNT_KEY,
  COIN_CAP,
  STORAGE_VERSION,
  LEGACY_KEYS,
});
