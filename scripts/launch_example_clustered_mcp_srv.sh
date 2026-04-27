#!/bin/bash

script_path=$(realpath $0)
script_dir=$(dirname $script_path)

cd $script_dir/..

go run ./cmd/globalping mcp \
  --ipregistry-api-endpoint="http://localhost:8084/proxy/ipregistry" \
  --ipregistry-apikey-env="IPREGISTRY_API_KEY" \
  --ipregistry-add-bearer-header="true" \
  --listen-address=":8090" \
  --authentication="jwt" \
  --ping-resolver="127.0.0.53:53" \
  --upstream-jwt-sec-env="UPSTREAM_JWT_TOKEN" \
  --upstream-api-prefix="http://localhost:8084" \
  --jwt-auth-secret-from-env="JWT_SECRET"
