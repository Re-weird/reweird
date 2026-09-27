#!/bin/sh
# Runs as root so it can fix ownership of a freshly-mounted volume (Railway,
# and most hosts, mount volumes owned by root regardless of the image's
# USER), then drops to the unprivileged "reweird" user to actually run the
# server. Without this, DATABASE_PATH/UPLOAD_DIR under a mounted volume are
# unwritable by "reweird" and sqlite fails to open the database file.
set -e

for path in "${DATABASE_PATH:-./reweird.db}" "${UPLOAD_DIR:-./data/uploads}"; do
  dir=$(dirname "$path")
  mkdir -p "$dir"
  chown -R reweird:reweird "$dir" 2>/dev/null || true
done

exec su-exec reweird "$@"
