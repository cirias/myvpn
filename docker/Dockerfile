FROM ubuntu:16.04

MAINTAINER Sirius

RUN apt-get update && \
  apt-get install -y iproute2 iputils-ping traceroute && \
  rm -rf /tmp/* /var/tmp/* /var/cache/apk/* /var/cache/distfiles/*

COPY docker-entrypoint.sh /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]

CMD ["/bin/bash"]
