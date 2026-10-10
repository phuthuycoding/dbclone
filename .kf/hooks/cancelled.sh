#!/usr/bin/env bash
# Generic GitHub sync — cancelled: close the issue as not planned, leave the card.
. "$(dirname "$0")/lib-github.sh" || exit 0
[ -n "${KFW_REPOSITORY:-}" ] || exit 0
gh_ready || exit 0
close_issue_cancelled "$KFW_FEATURE_DIR"
exit 0
