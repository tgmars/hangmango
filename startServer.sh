timestamp() {
  date +"%Y/%m/%d %T"
}

function control_c {
    echo -e "$(timestamp) - BASH - Exiting hangmango server..."
}

trap control_c SIGINT
trap control_c SIGTERM

echo -e "$(timestamp) - BASH - Copying hangmango.crt from server to client..."
cp ./app/server/hangmango.crt ./app/client/hangmango.crt || echo -e "$(timestamp) - BASH - Failed to copy hangmango.crt - Ensure that the hangmango server has been built first."

./app/hangmanserver -lport=$1 -wordlist="./app/wordlist.txt"

