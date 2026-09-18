FROM alpine:3.20

RUN apk add --no-cache openssh \
    && adduser -D -s /bin/sh xentra \
    && echo 'xentra:unused-password' | chpasswd \
    && mkdir -p /home/xentra/.ssh /run/sshd \
    && chown -R xentra:xentra /home/xentra/.ssh \
    && chmod 700 /home/xentra/.ssh \
    && ssh-keygen -A \
    && printf '%s\n' \
       'Port 22' \
       'HostKey /etc/ssh/ssh_host_ed25519_key' \
       'PubkeyAuthentication yes' \
       'PasswordAuthentication no' \
       'PermitRootLogin no' \
       'AllowUsers xentra' \
       'AuthorizedKeysFile .ssh/authorized_keys' \
       'UsePAM no' \
       > /etc/ssh/sshd_config

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 22
ENTRYPOINT ["/entrypoint.sh"]
