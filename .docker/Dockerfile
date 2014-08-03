############################################################
# Dockerfile to run Neverdown inside a container
# Based on Ubuntu Image
############################################################

FROM ubuntu
MAINTAINER Thomas Sileo
EXPOSE 8000
EXPOSE 7000
ENV UPCHECK_ADDR :8000
ENV UPCHECK_PREFIX data
ENV UPCHECK_HTTP :7000
WORKDIR /data/neverdown
ADD ./neverdown /opt/neverdown/bin/neverdown
ENTRYPOINT /opt/neverdown/bin/neverdown