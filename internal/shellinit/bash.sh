# dl shell integration for bash. See the zsh version for why a temp file is
# used instead of capturing stdout.
dl() {
  local cdfile rc
  cdfile="$(mktemp "${TMPDIR:-/tmp}/dl-cd.XXXXXX")" || return 1
  DL_CD_FILE="$cdfile" command dl "$@"
  rc=$?
  if [ -s "$cdfile" ]; then
    builtin cd -- "$(cat "$cdfile")" || rc=$?
  fi
  command rm -f "$cdfile"
  return $rc
}
