# dl shell integration for fish. See the zsh version for why a temp file is
# Install with:  dl init fish | source   in config.fish, AFTER your PATH setup
# (it runs the dl binary, so dl must already be on $PATH).
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
