#!/usr/bin/env bash
# Generic GitHub sync — implementation: reopen a closed issue, self-assign, move the card.
. "$(dirname "$0")/lib-github.sh" || exit 0
[ -n "${KFW_REPOSITORY:-}" ] || exit 0
gh_ready || exit 0
url=$(issue_url_of "$KFW_FEATURE_DIR") || exit 0
if [ -n "$url" ]; then
  state=$(issue_state "$url") || state=""
  [ "$state" = "CLOSED" ] && reopen_issue "$KFW_FEATURE_DIR"
fi
assign_issue_self "$KFW_FEATURE_DIR"
sync_project_status "$KFW_FEATURE_DIR" implementation
exit 0
