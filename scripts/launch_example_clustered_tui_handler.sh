#!/bin/bash

script_path=$(realpath $0)
script_dir=$(dirname $script_path)

cd $script_dir/..

go run ./cmd/globalping tui \
  --listen-address=":8088" \
  --authentication="none" \
  --ping-resolver="127.0.0.53:53" \
  --upstream-jwt-sec-env="UPSTREAM_JWT_TOKEN" \
  --upstream-api-prefix="http://localhost:8084" \
  --cloudflare-team-name="ideaignites" \
  --cloudflare-aud-env="CF_TUI_SVC_AUD"
