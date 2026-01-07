#!/bin/bash

scriptPath=$(realpath "$0")
scriptDir=$(dirname "$scriptPath")

domain="ping.dn42"
email="i@exploro.one"
burbleACME="https://acme.burble.dn42/v1/dn42/acme/directory"
burbleStagingACME="https://acme.burble.dn42/v1/staging/acme/directory"
acmeServer="$burbleACME"

docker run \
  -v globalping_acme_state:/acme.sh \
  -v globalping_acme_challenge:/usr/share/nginx/acme-challenge \
  -v $scriptDir/../nginx/custom-ca:/etc/nginx/custom-ca:ro \
  --network host \
  --rm \
  --pull always \
  -e 'http_proxy=http://192.168.250.2:1081' \
  -e 'https_proxy=http://192.168.250.2:1081' \
  -it docker.io/neilpang/acme.sh:latest acme.sh \
    --email "$email" \
    --ca-bundle /etc/nginx/custom-ca/dn42-ca.crt \
    --server "$acmeServer" \
    --issue \
    --domain "$domain" \
    --force \
    -w /usr/share/nginx/acme-challenge
