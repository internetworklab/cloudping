#!/bin/bash

domain="ping.dn42"

# install cert for lounge
docker run \
  --rm \
  -it \
  -v globalping_acme_state:/acme.sh \
  -v globalping_acme_certs:/etc/nginx/acme-certs \
  docker.io/neilpang/acme.sh:latest \
    acme.sh \
    --install-cert \
    --domain $domain \
    --fullchain-file /etc/nginx/acme-certs/ping.dn42.crt \
    --key-file /etc/nginx/acme-certs/ping.dn42.key
