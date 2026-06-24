#!/bin/zsh

typeset -F last_mtime
last_mtime=0

while true; do
    # Get the current modification time of the file
    current_mtime=$(stat -f "%m" ./cmd/LiSP/scm.go 2>/dev/null)

    # Check if the file was modified since the last check
    if (( current_mtime > last_mtime )); then
        clear
        echo "$(date): File updated, running ./test/rercl.sh..."
        ./test/rercl.sh
        last_mtime=$current_mtime
    fi

    # Wait for one second before checking again
    sleep 1
done
