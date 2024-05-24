#!/bin/sh

# Function to stop WireGuard
stop() {
    wg-quick down wg0
    exit 0
}

# Set up trap to handle SIGTERM, SIGINT, and SIGQUIT
trap stop SIGTERM SIGINT SIGQUIT

# Wait for the /etc/wireguard directory to have contents
while [ "$(ls -A /etc/wireguard 2>/dev/null)" = "" ]; do
    echo "Waiting for configuration files in /etc/wireguard..."
    sleep 5
done

# Initial setup
wg-quick up /etc/wireguard/wg0.conf

# Reload the configuration every 30 seconds
while true; do wg syncconf wg0 <(wg-quick strip wg0); sleep 30; done &

# Monitor /etc/wireguard for changes and reload wg0 if changes are detected
inotifywait -m -e create -e delete -e modify -e moved_to -e moved_from --format '%w%f' /etc/wireguard | while read FILE
do
    CURRENT_IP_ADDRESS=$(ip -br addr show wg0 | awk '{print $3}')
    ADDRESS_FROM_CONFIG=$(grep "Address" $FILE | cut -d ' ' -f 3)
    echo "Current IP address of wg0 interface: $CURRENT_IP_ADDRESS"
    echo "Address from config: $ADDRESS_FROM_CONFIG"

    # If address from config is different from current IP address, hard reload using wg-quick down/up, otherwise use wg syncconf

    if [ "$CURRENT_IP_ADDRESS" != "$ADDRESS_FROM_CONFIG" ]; then
        echo "Hard reloading Wireguard configuration..."
        wg-quick down wg0
        wg-quick up $FILE
    else
        wg syncconf wg0 <(wg-quick strip wg0)
        echo "Soft reloading Wireguard configuration..."
    fi

    echo "Wireguard configuration reloaded from $FILE..."
done