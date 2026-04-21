#!/bin/bash

script_path=$(realpath $0)
script_dir=$(dirname $script_path)

cd $script_dir/..

go run ./cmd/globalping hub \
  --upstream-ipregistry-apiendpoint="https://api.ipregistry.co" \
  --upstream-ipregistry-apikey-env="IPREGISTRY_API_KEY" \
  --server-cert="test/certs/peer.pem" \
  --server-cert-key="test/certs/peer-key.pem" \
  --public-http-listen-address=":8084" \
  --jwt-quic-listen-address=":18449" \
  --min-pkt-interval="30ms" \
  --max-pkt-timeout="3000ms" \
  --jwt-auth-secret-from-env="JWT_SECRET"
