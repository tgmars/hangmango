function control_c {
    echo -e "\nBASH - Exiting hangmango client..."
}

trap control_c SIGINT
trap control_c SIGTERM
./app/hangmanclient -dhost=$1 -dport=$2 

