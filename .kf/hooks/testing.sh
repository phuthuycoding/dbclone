#!/usr/bin/env bash
# Generic GitHub sync — testing: move the Project card only.
. "$(dirname "$0")/lib-github.sh" || exit 0
[ -n "${KFW_REPOSITORY:-}" ] || exit 0
sync_project_status "$KFW_FEATURE_DIR" testing
exit 0
