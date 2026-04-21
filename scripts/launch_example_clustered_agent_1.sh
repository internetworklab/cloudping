#!/bin/bash

script_path=$(realpath $0)
script_dir=$(dirname $script_path)

cd $script_dir/..

HUB_SERVER_NAME=cloudping-hub.example.com

go run ./cmd/globalping agent \
  --ipregistry-api-endpoint=http://localhost:8084/proxy/ipregistry \
  --ipregistry-apikey-env=IPREGISTRY_API_KEY \
  --ipregistry-add-bearer-header=true \
  --quic-server-address="127.0.0.1:18449" \
  --server-name="${HUB_SERVER_NAME}" \
  --peer-ca="test/certs/ca.pem" \
  --node-name="us-lax1" \
  --exact-location-lat-lon="34.0200392,-118.7413874" \
  --country-code="US" \
  --city-name="Los Angeles" \
  --asn="AS12345" \
  --isp="Example LLC" \
  --dn42-asn="AS4242421234" \
  --dn42-isp="EXAMPLE-DN42" \
  --http-listen-address=":8085"
