/* Centralized persistence contract. Do not rename keys without a migration. */

const KEY = 'twenty_four_progress';
const BACKUP_KEY = 'twenty_four_progress_backup';
const ERROR_LOG_KEY = 'twenty_four_runtime_logs';
const COIN_CAP = 999999;
const STORAGE_VERSION = 12;

const LEGACY_KEYS = Object.freeze({
  coins: 'coins',
  unlockedLevel: 'unlockedLevel',
  dailyDone: 'dailyDone',
});

module.exports = Object.freeze({
  KEY,
  BACKUP_KEY,
  ERROR_LOG_KEY,
  COIN_CAP,
  STORAGE_VERSION,
  LEGACY_KEYS,
});
