/* Centralized persistence contract. Do not rename keys without a migration. */

const KEY = 'twenty_four_progress';
const BACKUP_KEY = 'twenty_four_progress_backup';
const ERROR_LOG_KEY = 'twenty_four_runtime_logs';
// The unscoped keys above are kept for anonymous/offline play and one-time
// migration. Authenticated players use a separate namespace per backend user.
const ACCOUNT_KEY_PREFIX = 'twenty_four_progress_account_';
const ACCOUNT_BACKUP_KEY_PREFIX = 'twenty_four_progress_account_backup_';
const ACCOUNT_ERROR_LOG_PREFIX = 'twenty_four_runtime_logs_account_';
const ACTIVE_ACCOUNT_KEY = 'twenty_four_active_account';
const LEGACY_MIGRATION_KEY = 'twenty_four_progress_account_migration_v1';
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
  LEGACY_MIGRATION_KEY,
  COIN_CAP,
  STORAGE_VERSION,
  LEGACY_KEYS,
});
