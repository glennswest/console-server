#!/bin/sh

# Start sshd in background
/usr/sbin/sshd -D -e &

# Run console-server as main process
cd /etc/console-server
exec /usr/local/bin/console-server
