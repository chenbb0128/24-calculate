#!/usr/bin/env bash
set -Eeuo pipefail

# Restore one backup into a newly created, isolated database for verification.
# This script refuses to use the source production database or an existing
# restore database, so the drill cannot overwrite live data accidentally.

: "${MYSQL_DEFAULTS_FILE:?Set MYSQL_DEFAULTS_FILE to a chmod-600 MySQL option file}"
: "${BACKUP_FILE:?Set BACKUP_FILE to a .sql.gz backup}"
: "${RESTORE_DATABASE:?Set RESTORE_DATABASE to a new isolated database name}"

if [[ ! -r "$MYSQL_DEFAULTS_FILE" ]]; then
  echo "MySQL option file is not readable: $MYSQL_DEFAULTS_FILE" >&2
  exit 1
fi
if [[ ! -r "$BACKUP_FILE" ]]; then
  echo "Backup file is not readable: $BACKUP_FILE" >&2
  exit 1
fi
if [[ ! "$RESTORE_DATABASE" =~ ^[A-Za-z0-9_]+$ ]]; then
  echo "RESTORE_DATABASE may contain only letters, numbers and underscores" >&2
  exit 1
fi
if [[ "$RESTORE_DATABASE" == "mysql" || "$RESTORE_DATABASE" == "information_schema" || "$RESTORE_DATABASE" == "performance_schema" || "$RESTORE_DATABASE" == "sys" ]]; then
  echo "RESTORE_DATABASE is a reserved system database" >&2
  exit 1
fi
if [[ ! "$RESTORE_DATABASE" =~ ^(restore_|verify_) ]]; then
  echo "RESTORE_DATABASE must start with restore_ or verify_" >&2
  exit 1
fi

gzip -t "$BACKUP_FILE"
if [[ -r "$BACKUP_FILE.sha256" ]]; then
  (cd "$(dirname "$BACKUP_FILE")" && sha256sum --check "$(basename "$BACKUP_FILE.sha256")")
fi

existing="$(mysql --defaults-extra-file="$MYSQL_DEFAULTS_FILE" --batch --skip-column-names \
  -e "SELECT SCHEMA_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = '$RESTORE_DATABASE';")"
if [[ -n "$existing" ]]; then
  echo "Refusing to import into an existing database: $RESTORE_DATABASE" >&2
  exit 1
fi

mysql --defaults-extra-file="$MYSQL_DEFAULTS_FILE" \
  -e "CREATE DATABASE \`$RESTORE_DATABASE\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

if ! gzip -dc "$BACKUP_FILE" | mysql --defaults-extra-file="$MYSQL_DEFAULTS_FILE" "$RESTORE_DATABASE"; then
  echo "Restore failed; inspect and remove the isolated database manually: $RESTORE_DATABASE" >&2
  exit 1
fi

table_count="$(mysql --defaults-extra-file="$MYSQL_DEFAULTS_FILE" --batch --skip-column-names \
  "$RESTORE_DATABASE" -e "SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = '$RESTORE_DATABASE';")"
if [[ "$table_count" -lt 1 ]]; then
  echo "Restore completed but no tables were found in $RESTORE_DATABASE" >&2
  exit 1
fi

echo "Restore verification succeeded: database=$RESTORE_DATABASE tables=$table_count"
echo "The isolated database is intentionally left in place for manual spot checks."
