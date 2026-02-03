#!/bin/bash

script_path=$(realpath $0)
script_dir=$(dirname $script_path)

cd $script_dir/..

bin/globalping agent \
  --node-name="vie1" \
  --peer-ca="https://github.com/internetworklab/globalping/raw/refs/heads/master/confed/hub/ca.pem" \
  --server-name="globalping-hub.exploro.one" \
  --quic-server-address="globalping-hub.exploro.one:18447" \
  --client-cert="/root/services/globalping/agent/certs/peer.pem" \
  --client-cert-key="/root/services/globalping/agent/certs/peer-key.pem" \
  --exact-location-lat-lon="48.1952,16.3503" \
  --support-udp="true" \
  --support-pmtu="true" \
  --support-tcp="true" \
  --support-dns="true" \
  --ip2location-api-endpoint="https://api.ip2location.io" \
  --dn42-ip2location-api-endpoint="https://regquery.ping2.sh/ip2location/v1/query" \
  --jwt-token-from-env-var="JWT_TOKEN" \
  --log-echo-replies="false"
