#!/usr/bin/env bash
# Generic GitHub sync — planning: push the confirmed requirement into the issue body.
. "$(dirname "$0")/lib-github.sh" || exit 0
[ -n "${KFW_REPOSITORY:-}" ] || exit 0
gh_ready || exit 0
if command -v kf >/dev/null 2>&1; then
  kf issues sync "$KFW_FEATURE" >/dev/null 2>&1 \
    && echo "[github-sync] requirement synced for $KFW_FEATURE" \
    || warn "kf issues sync failed for $KFW_FEATURE (spec filled?)"
fi
sync_project_status "$KFW_FEATURE_DIR" planning
exit 0
