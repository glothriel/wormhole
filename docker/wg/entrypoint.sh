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

# Monitor /etc/wireguard for changes and reload wg0 if changes are detected
inotifywait -m -e create -e delete -e modify -e moved_to -e moved_from --format '%w%f' /etc/wireguard | while read FILE
do
    wg syncconf wg0 <(wg-quick strip wg0)
    # If for some reason the above doesn't work, you can stick to
    # wg-quick down wg0
    # wg-quick up /etc/wireguard/wg0.conf
    echo "Wireguard configuration reloaded from $FILE..."
done