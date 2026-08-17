#!/usr/bin/env bash
set -Eeuo pipefail

# Create a consistent logical backup for the native MySQL deployment.
# Credentials must be supplied through a chmod-600 MySQL option file, for example:
#   /etc/24-calculate/mysql-backup.cnf
# containing [client], user, password, host and port entries.

: "${MYSQL_DEFAULTS_FILE:?Set MYSQL_DEFAULTS_FILE to a chmod-600 MySQL option file}"
: "${MYSQL_DATABASE:?Set MYSQL_DATABASE to the production database name}"
: "${BACKUP_DIR:=/var/backups/24-calculate}"

if [[ ! -r "$MYSQL_DEFAULTS_FILE" ]]; then
  echo "MySQL option file is not readable: $MYSQL_DEFAULTS_FILE" >&2
  exit 1
fi

umask 077
mkdir -p "$BACKUP_DIR"
chmod 700 "$BACKUP_DIR"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
backup_file="$BACKUP_DIR/${MYSQL_DATABASE}_${timestamp}.sql.gz"
temporary_file="$backup_file.tmp.$$"
trap 'rm -f "$temporary_file"' EXIT

mysqldump \
  --defaults-extra-file="$MYSQL_DEFAULTS_FILE" \
  --single-transaction \
  --routines \
  --triggers \
  --events \
  --hex-blob \
  --no-tablespaces \
  "$MYSQL_DATABASE" \
  | gzip -9 > "$temporary_file"

gzip -t "$temporary_file"
mv -- "$temporary_file" "$backup_file"
trap - EXIT
sha256sum "$backup_file" > "$backup_file.sha256"

echo "Backup created: $backup_file"
echo "Checksum created: $backup_file.sha256"
echo "Keep backups according to the server retention policy; this script never deletes old backups."
