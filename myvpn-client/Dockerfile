FROM ubuntu:14.04

COPY ./myvpn-client ./script /opt/myvpn/
COPY ./myvpn-client ./script /opt/myvpn/

CMD [ "/opt/myvpn/myvpn-client", "-up-script=/opt/myvpn/if-up.sh", "-down-script=/opt/myvpn/if-down.sh", "-logtostderr" ]
