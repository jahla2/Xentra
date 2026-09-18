#!/bin/sh
set -eu

if [ -z "$AUTHORIZED_KEY" ]; then
  echo "AUTHORIZED_KEY is required" >&2
  exit 1
fi

printf '%s\n' "$AUTHORIZED_KEY" > /home/xentra/.ssh/authorized_keys
chown -R xentra:xentra /home/xentra/.ssh
chmod 700 /home/xentra/.ssh
chmod 600 /home/xentra/.ssh/authorized_keys

exec /usr/sbin/sshd -D -e
