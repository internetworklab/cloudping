#!/bin/bash

script_path=$(realpath $0)
script_dir=$(dirname $script_path)

cd $script_dir/..

NODE_NAME=vie1
HTTP_ENDPOINT=https://vie1.exploro.one:18081/simpleping
HUB_SERVER_ADDRESS=wss://globalping-hub.exploro.one:28080/ws
HUB_SERVER_NAME=globalping-hub.exploro.one
TLS_LISTEN_ADDRESS=:18081
EXACT_LOCATION_LAT_LON=48.1952,16.3503
VERSION=latest

bin/globalping agent \
  --server-address=${HUB_SERVER_ADDRESS} \
  --node-name=${NODE_NAME} \
  --http-endpoint=${HTTP_ENDPOINT} \
  --peer-c-as=https://github.com/internetworklab/globalping/raw/refs/heads/master/confed/hub/ca.pem \
  --server-name=${HUB_SERVER_NAME} \
  --client-cert=/root/services/globalping/agent/certs/peer.pem \
  --client-cert-key=/root/services/globalping/agent/certs/peer-key.pem \
  --server-cert=/root/services/globalping/agent/certs/peer.pem \
  --server-cert-key=/root/services/globalping/agent/certs/peer-key.pem \
  --tls-listen-address=${TLS_LISTEN_ADDRESS} \
  --shared-quota=10 \
  --exact-location-lat-lon=${EXACT_LOCATION_LAT_LON} \
  --support-udp=true \
  --support-pmtu=true \
  --support-tcp=true \
  --ip-2-location-api-endpoint=https://api.ip2location.io \
  --dn-42-ip-2-location-api-endpoint=https://regquery.ping2.sh/ip2location/v1/query \
  --metrics-listen-address=:12112 \
  --quic-server-address=globalping-hub.exploro.one:18443 \
  --log-echo-replies
