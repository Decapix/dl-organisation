# dl shell integration for fish. See the zsh version for why a temp file is
# used instead of capturing stdout.
function dl
    set -l cdfile (mktemp)
    DL_CD_FILE=$cdfile command dl $argv
    set -l rc $status
    if test -s $cdfile
        builtin cd (cat $cdfile); or set rc $status
    end
    command rm -f $cdfile
    return $rc
end
