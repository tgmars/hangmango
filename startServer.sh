function control_c {
    echo -e "\nBASH - Exiting hangmango server..."
}

trap control_c SIGINT
trap control_c SIGTERM

./app/hangmanserver -lport=$1

