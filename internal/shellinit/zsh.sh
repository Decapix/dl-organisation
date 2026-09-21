# dl shell integration for zsh.
#
# A program cannot change its parent shell's directory, so dl writes the target
# path into the file named by $DL_CD_FILE and this function performs the cd.
# A temp file is used rather than capturing stdout because capturing stdout
# would break the interactive browser, which needs the real terminal.
dl() {
  local cdfile rc
  cdfile="$(mktemp "${TMPDIR:-/tmp}/dl-cd.XXXXXX")" || return 1
  DL_CD_FILE="$cdfile" command dl "$@"
  rc=$?
  if [[ -s "$cdfile" ]]; then
    builtin cd -- "$(<"$cdfile")" || rc=$?
  fi
  command rm -f "$cdfile"
  return $rc
}
