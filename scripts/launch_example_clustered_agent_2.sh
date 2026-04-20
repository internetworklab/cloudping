#!/bin/bash

script_path=$(realpath $0)
script_dir=$(dirname $script_path)

cd $script_dir/..

HUB_SERVER_NAME=cloudping-hub.example.com

go run ./cmd/globalping agent \
  --quic-server-address="127.0.0.1:18449" \
  --server-name="${HUB_SERVER_NAME}" \
  --peer-ca="test/certs/ca.pem" \
  --node-name="at/vie1" \
  --exact-location-lat-lon="48.1952,16.3503" \
  --country-code="AT" \
  --city-name="Vienna" \
  --asn="AS12345" \
  --isp="Example LLC" \
  --dn42-asn="AS4242421234" \
  --dn42-isp="EXAMPLE-DN42" \
  --http-listen-address=":8086"
