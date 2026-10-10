#!/usr/bin/env bash
# Generic GitHub sync — dones: the archive gate.
# Fail-closed on unreadable .kfw.json (exit 1 refuses the transition), fail-open on GitHub
# outages (the archive proceeds but the issue and board are left untouched). With AC_GATE=1 a
# PASS report ticks the acceptance criteria and the evidence lands in the issue; AC that stay
# unchecked refuse the archive. acGate: false in config skips the check entirely.
. "$(dirname "$0")/lib-github.sh" || exit 0
[ -n "${KFW_REPOSITORY:-}" ] || exit 0
gh_ready || exit 0

url=$(issue_url_of "$KFW_FEATURE_DIR"); rc=$?
if [ $rc -eq 2 ]; then
  echo "[github-sync] ERROR cannot read .kfw.json for $KFW_FEATURE — refusing dones" >&2
  exit 1
fi
[ -n "$url" ] || exit 0

if [ "${KFW_PROJECT_AC_GATE:-1}" = "1" ]; then
  sync_issue_result_from_reports "$KFW_FEATURE_DIR" "$url"; rc=$?
  if [ $rc -eq 2 ]; then
    warn "could not verify AC on $url — dones proceeds, issue left open"; exit 0
  fi
  # rc 0 (synced) or 1 (no PASS report): check what remains unchecked either way
  unchecked=$(issue_unchecked_ac "$url"); rc=$?
  if [ $rc -eq 2 ]; then
    warn "could not read AC on $url — dones proceeds, issue left open"; exit 0
  fi
  if [ -n "$unchecked" ]; then
    echo "[github-sync] $url still has unchecked acceptance criteria — kf archive refused:" >&2
    echo "$unchecked" >&2
    exit 1
  fi
fi
sync_project_status "$KFW_FEATURE_DIR" dones
close_issue_done "$KFW_FEATURE_DIR" "Work item archived in kf."
exit 0
