#!/usr/bin/env bash
# Generic GitHub sync pack, seeded by kf init into .kf/hooks/. Driven entirely by env:
#   KFW_REPOSITORY          owner/name, required — empty means no sync at all
#   KFW_PROJECT_OWNER       GitHub Projects board owner (user or org)
#   KFW_PROJECT_NUMBER      board number
#   KFW_PROJECT_STATUS_*    per-stage Status option name, e.g. KFW_PROJECT_STATUS_DONES="Done"
#   KFW_PROJECT_AC_GATE     "1" (default) gates dones on ticked acceptance criteria, "0" skips
# Field and item IDs are resolved per run via `gh project field-list`/`view` — the board can be
# recreated and only the option names matter. Every function is fail-open on GitHub errors:
# it warns and returns 0, so a stage move is never blocked by GitHub being unreachable —
# except the dones AC gate, which skips closing rather than guessing (fail-closed on data,
# fail-open on network, never silent).

[ -n "${KFW_REPOSITORY:-}" ] || return 0 2>/dev/null || exit 0
REPO="$KFW_REPOSITORY"

warn() { echo "[github-sync] $*" >&2; }

gh_ready() {
  command -v gh >/dev/null 2>&1 || { warn "gh not installed, skipping"; return 1; }
  # output withheld on purpose: it prints the account and token details
  gh auth status -h github.com >/dev/null 2>&1 || { warn "gh not authenticated (check with: gh auth status), skipping"; return 1; }
  return 0
}

project_ready() {
  [ -n "${KFW_PROJECT_OWNER:-}" ] && [ -n "${KFW_PROJECT_NUMBER:-}" ]
}

# Print the issue URL recorded in the work item's .kfw.json ("" when the item has none yet).
# Returns 2 with the parse error when .kfw.json is missing or unreadable: callers must not take
# that for "no issue", which would skip the dones gate or create a duplicate issue.
issue_url_of() {  # $1 = work item dir
  local out
  if out=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("issue",""))' "$1/.kfw.json" 2>&1); then
    printf '%s\n' "$out"; return 0
  fi
  warn "cannot read $1/.kfw.json: $(printf '%s' "$out" | tail -1)"; return 2
}

issue_number_of_url() { echo "${1##*/}"; }

# --- GitHub Projects board -------------------------------------------------------------------

PROJECT_FIELDS_FILE=$(mktemp); trap 'rm -f "$PROJECT_FIELDS_FILE" "${PROJECT_FIELDS_FILE}.view"' EXIT

# One fetch per process: the field list is a heavy GraphQL query and callers run inside $(...),
# where a variable cache would die with the subshell.
project_fields() {
  project_ready || return 1
  [ -s "$PROJECT_FIELDS_FILE" ] && return 0
  local err; err=$(mktemp)
  if gh project field-list "$KFW_PROJECT_NUMBER" --owner "$KFW_PROJECT_OWNER" --format json </dev/null >"$PROJECT_FIELDS_FILE" 2>"$err"; then
    rm -f "$err"; return 0
  fi
  warn "project field-list failed: $(head -c 300 "$err")"; : > "$PROJECT_FIELDS_FILE"; rm -f "$err"; return 1
}

project_id() {
  project_ready || return 1
  local f="$PROJECT_FIELDS_FILE.view"
  if [ ! -s "$f" ]; then
    gh project view "$KFW_PROJECT_NUMBER" --owner "$KFW_PROJECT_OWNER" --format json </dev/null >"$f" 2>/dev/null || { : > "$f"; return 1; }
  fi
  python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("id",""))' "$f" 2>/dev/null
}

# stage -> configured Status option name (KFW_PROJECT_STATUS_<STAGE>)
status_option_name() {
  local v="KFW_PROJECT_STATUS_$(printf '%s' "$1" | tr 'a-z' 'A-Z')"
  eval "printf '%s' \"\${$v:-}\""
}

# stage -> Status option id, resolved live so a recreated board still works
status_option_id() {  # $1 = stage
  local name; name=$(status_option_name "$1"); [ -n "$name" ] || return 0
  project_fields || return 0
  python3 -c 'import json,sys
d=json.load(open(sys.argv[1]))
for f in d.get("fields",[]):
    if f.get("name")=="Status" and f.get("options"):
        for o in f["options"]:
            if o["name"]==sys.argv[2]:
                print(o["id"]); sys.exit()
' "$PROJECT_FIELDS_FILE" "$name" 2>/dev/null
}

# The Project item id for an issue url, adding the issue to the board when absent.
PROJECT_ITEMS_FILE=$(mktemp); trap 'rm -f "$PROJECT_ITEMS_FILE"' EXIT
project_item_for() {  # $1 = issue url
  project_ready || return 0
  local num id err
  num=$(issue_number_of_url "$1")
  if [ ! -s "$PROJECT_ITEMS_FILE" ]; then
    err=$(mktemp)
    gh project item-list "$KFW_PROJECT_NUMBER" --owner "$KFW_PROJECT_OWNER" --format json --limit 500 </dev/null >"$PROJECT_ITEMS_FILE" 2>"$err" \
      || { warn "project item-list failed: $(head -c 300 "$err")"; : > "$PROJECT_ITEMS_FILE"; rm -f "$err"; return 0; }
    rm -f "$err"
  fi
  id=$(cat "$PROJECT_ITEMS_FILE" \
      | python3 -c 'import json,sys; n=int(sys.argv[1]); r=sys.argv[2]; print(next((i["id"] for i in json.load(sys.stdin).get("items",[]) if (i.get("content") or {}).get("number")==n and (i.get("content") or {}).get("repository")==r),""))' "$num" "$REPO" 2>/dev/null)
  if [ -z "$id" ]; then
    err=$(mktemp)
    id=$(gh project item-add "$KFW_PROJECT_NUMBER" --owner "$KFW_PROJECT_OWNER" --url "$1" --format json </dev/null 2>"$err" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("id",""))' 2>/dev/null)
    if [ -n "$id" ]; then warn "added $1 to project"; else warn "project item-add failed for $1: $(head -c 300 "$err")"; fi
    rm -f "$err"
  fi
  echo "$id"
}

# Set the board Status of the work item's issue to the stage's configured option.
sync_project_status() {  # $1 = dir, $2 = stage
  project_ready || return 0
  gh_ready || return 0
  local url opt item pid
  url=$(issue_url_of "$1") || return 0
  [ -n "$url" ] || { warn "no issue url in $1/.kfw.json"; return 0; }
  opt=$(status_option_id "$2"); [ -n "$opt" ] || return 0
  item=$(project_item_for "$url"); [ -n "$item" ] || { warn "could not resolve project item for $url"; return 0; }
  pid=$(project_id); [ -n "$pid" ] || { warn "could not resolve project id"; return 0; }
  local err
  if err=$(gh project item-edit --project-id "$pid" --id "$item" --field-id "$(status_field_id)" --single-select-option-id "$opt" </dev/null 2>&1 >/dev/null); then
    echo "[github-sync] $url -> $2"
  else
    warn "status update failed for $url: ${err:0:300}"
  fi
}

status_field_id() {
  project_fields || return 0
  python3 -c 'import json,sys
d=json.load(open(sys.argv[1]))
for f in d.get("fields",[]):
    if f.get("name")=="Status" and f.get("options"):
        print(f["id"]); sys.exit()
' "$PROJECT_FIELDS_FILE" 2>/dev/null
}

# --- Issue state helpers ---------------------------------------------------------------------

issue_state() {  # $1 = issue url -> OPEN/CLOSED; return 2 on failure
  local out; out=$(gh issue view "$1" -R "$REPO" --json state --jq .state </dev/null 2>&1) || { warn "cannot read state of $1: ${out:0:300}"; return 2; }
  printf '%s\n' "$out"
}

close_issue_done() {  # $1 = dir, $2 = comment
  gh_ready || return 0
  local url err; url=$(issue_url_of "$1") || return 0; [ -n "$url" ] || return 0
  if err=$(gh issue close "$url" -R "$REPO" --comment "$2" </dev/null 2>&1 >/dev/null); then echo "[github-sync] closed $url"; else warn "close failed for $url: ${err:0:300}"; fi
}

close_issue_cancelled() {  # $1 = dir
  gh_ready || return 0
  local url err; url=$(issue_url_of "$1") || return 0; [ -n "$url" ] || return 0
  if err=$(gh issue close "$url" -R "$REPO" --reason "not planned" --comment "Work item cancelled in kf ($(date -u +%F))." </dev/null 2>&1 >/dev/null); then
    echo "[github-sync] closed $url as not planned"; else warn "close (not planned) failed for $url: ${err:0:300}"; fi
}

assign_issue_self() {  # $1 = dir
  gh_ready || return 0
  local url me err; url=$(issue_url_of "$1") || return 0; [ -n "$url" ] || return 0
  me=$(gh api user --jq .login </dev/null 2>&1) || { warn "cannot read gh login: ${me:0:300}"; return 0; }
  if err=$(gh issue edit "$url" -R "$REPO" --add-assignee "$me" </dev/null 2>&1 >/dev/null); then echo "[github-sync] assigned $url -> $me"; else warn "assign failed for $url: ${err:0:300}"; fi
}

reopen_issue() {  # $1 = dir
  gh_ready || return 0
  local url err; url=$(issue_url_of "$1") || return 1; [ -n "$url" ] || return 0
  err=$(gh issue reopen "$url" -R "$REPO" </dev/null 2>&1 >/dev/null) || { warn "reopen failed for $url: ${err:0:300}"; return 1; }
}

# --- Acceptance criteria (the dones gate) ----------------------------------------------------

# Print an issue body; return 2 when GitHub cannot be read — an empty body must never pass the gate.
issue_body() {  # $1 = issue url; DRY_BODY_FILE overrides for dry runs
  if [ -n "${DRY_BODY_FILE:-}" ] && [ -r "$DRY_BODY_FILE" ]; then cat "$DRY_BODY_FILE"; return 0; fi
  local err out; err=$(mktemp)
  if out=$(gh issue view "$1" -R "$REPO" --json body --jq .body </dev/null 2>"$err"); then rm -f "$err"; printf '%s\n' "$out"; return 0; fi
  warn "cannot read $1: $(head -c 300 "$err")"; rm -f "$err"; return 2
}

# Print unchecked AC lines ("" = none). Heading match is case-insensitive: kf templates write
# "## Acceptance Criteria", hand-written issues "## Acceptance criteria". Returns 2 unreadable.
issue_unchecked_ac() {  # $1 = issue url
  local body; body=$(issue_body "$1") || return 2
  printf '%s\n' "$body" | python3 -c '
import sys,re
body=sys.stdin.read()
m=re.search(r"^## Acceptance criteria.*?(?=^## |\Z)", body, re.S|re.M|re.I)
sec=m.group(0) if m else ""
for line in sec.splitlines():
    if re.match(r"^\s*- \[ \]", line): print(line.strip())
'
}

# Tick the AC in the issue body from phase-4-testing-result.md (status: PASS) and append a
# "Kết quả thực hiện" section. Returns 1 when no PASS report exists, 2 when the issue cannot be
# read or edited. DRY=1 prints the new body instead of editing.
sync_issue_result_from_reports() {  # $1 = dir, $2 = issue url
  local dir="$1" url="$2"
  local t="$dir/phase-4-testing-result.md" r="$dir/phase-5-review-report.md"
  [ -f "$t" ] || return 1
  local status; status=$(sed -n 's/^status: *//p' "$t" | head -1 | tr -d '\r')
  [ "$status" = "PASS" ] || return 1
  local body tmp; tmp=$(mktemp)
  issue_body "$url" > "$tmp.body" || { rm -f "$tmp" "$tmp.body"; return 2; }
  python3 - "$tmp.body" "$t" "$r" "$dir" > "$tmp" <<'PY'
import sys,re,datetime
body=open(sys.argv[1]).read(); t=open(sys.argv[2]).read(); rpath=sys.argv[3]; d=sys.argv[4]
exe=(re.search(r"^execution: *\"?([^\"\n]+)",t,re.M) or [None,"?"])[1]
tested=(re.search(r"^tested: *\"?([^\"\n]+)",t,re.M) or [None,"?"])[1]
today=datetime.date.today().isoformat()
def tick(sec):
    return re.sub(r"^(\s*)- \[ \] (.*)$", lambda m: f"{m.group(1)}- [x] {m.group(2)} — PASS theo `phase-4-testing-result.md` (execution `{exe}`, {tested})", sec, flags=re.M)
m=re.search(r"^## Acceptance criteria.*?(?=^## |\Z)", body, re.S|re.M|re.I)
if m: body=body[:m.start()]+tick(m.group(0))+body[m.end():]
def section(text,title):
    mm=re.search(rf"^## {re.escape(title)}\s*\n(.*?)(?=^## |\Z)", text, re.S|re.M); return mm.group(1).strip() if mm else ""
summary=section(t,"Summary"); results=section(t,"Test Results"); concl=section(t,"Conclusion")
review=""
try:
    rv=open(rpath).read(); rs=(re.search(r"^status: *(\S+)",rv,re.M) or [None,"?"])[1]; review=f"**Review:** {rs}. "+section(rv,"Final Decision").replace("\n"," ")
except FileNotFoundError: pass
block=f"\n\n## Kết quả thực hiện {today}\n**Testing:** PASS, execution `{exe}`, {tested}.\n\n{summary}\n\n{results}\n\n{concl}\n\n{review}\n\n_Synced automatically from `{d}` by the dones hook._\n"
if "## Kết quả thực hiện" in body:
    # stop before the next heading or the kf footer so a re-run replaces the section, never the footer
    # lambda: block holds report text; as a plain repl string, a "\d" in it would crash re.sub
    body=re.sub(r"\n*## Kết quả thực hiện.*?(?=\n## |\n---\n\*\*Work item kf:\*\*|\Z)", lambda _m: block, body, count=1, flags=re.S)
else:
    # insert before the trailing "---\n**Work item kf:**" footer when present
    i=body.rfind("\n---\n**Work item kf:**")
    body = body[:i]+block+body[i:] if i!=-1 else body+block
sys.stdout.write(body)
PY
  if [ $? -ne 0 ] || [ ! -s "$tmp" ]; then warn "rewriting the body of $url failed; issue left untouched"; rm -f "$tmp" "$tmp.body"; return 2; fi
  if [ "${DRY:-0}" = "1" ]; then cat "$tmp"; DRY_BODY_FILE="$tmp"; rm -f "$tmp.body"; return 0; fi
  local err; err=$(gh issue edit "$url" -R "$REPO" --body-file "$tmp" </dev/null 2>&1 >/dev/null)
  local rc=$?; rm -f "$tmp" "$tmp.body"
  if [ $rc -ne 0 ]; then warn "AC sync failed for $url: ${err:0:300}"; return 2; fi
  echo "[github-sync] AC + result synced into $url"; return 0
}
