#!/usr/bin/env bash
# Generic GitHub sync — brainstorm: file an issue for the new work item and link it.
# kf issues create reuses the title/body/meta logic (goal->title, spec->body, URL into .kfw.json).
. "$(dirname "$0")/lib-github.sh" || exit 0
[ -n "${KFW_REPOSITORY:-}" ] || exit 0
gh_ready || exit 0
command -v kf >/dev/null 2>&1 || { warn "kf not on PATH, skipping issue create"; exit 0; }

if ! issue_url_of "$KFW_FEATURE_DIR" >/dev/null 2>&1 || [ -z "$(issue_url_of "$KFW_FEATURE_DIR" 2>/dev/null)" ]; then
  kf issues create "$KFW_FEATURE" >/dev/null 2>&1 \
    && echo "[github-sync] issue created for $KFW_FEATURE" \
    || warn "kf issues create failed for $KFW_FEATURE"
fi
sync_project_status "$KFW_FEATURE_DIR" brainstorm
exit 0
