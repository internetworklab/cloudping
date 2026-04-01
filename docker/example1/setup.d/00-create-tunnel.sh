#!/bin/bash

# Run this only at first time when the cloudflared tunnel object hasn't been created yet.
# It generates cfd_tunnel.json as the response content,
# cfd_credentials.json as credential file for your cloudflared client to authenticate itself to cloudflare endpoint,
# then it prints tunnel UUID to stdout.

set -e

script_path=$(realpath "$0")
script_dir=$(dirname "$script_path")
cd "${script_dir}/.."


source .env

if [ -z "${CF_ACCOUNT_ID}" ]; then
    echo "No CF_ACCOUNT_ID provided"
    exit 1
fi

if [ -z "${CF_API_TOKEN}" ]; then
    echo "No CF_API_TOKEN provided"
    exit 1
fi

if [ -z "${TUNNEL_NAME}" ]; then
    echo "No TUNNEL_NAME provided"
    exit 1
fi

# see https://developers.cloudflare.com/api/resources/zero_trust/subresources/tunnels/

curl -o ./cfd_tunnel.json "https://api.cloudflare.com/client/v4/accounts/${CF_ACCOUNT_ID}/cfd_tunnel" \
  --request POST \
  --header "Authorization: Bearer ${CF_API_TOKEN}" \
  --json "{
    \"name\": \"${TUNNEL_NAME}\",
    \"config_src\": \"local\"
  }"

# example response like:
# {
#   "success": true,
#   "errors": [],
#   "messages": [],
#   "result": {
#     "id": "c1744f8b-faa1-48a4-9e5c-02ac921467fa",
#     "account_tag": "699d98642c564d2e855e9661899b7252",
#     "created_at": "2025-02-18T22:41:43.534395Z",
#     "deleted_at": null,
#     "name": "example-tunnel",
#     "connections": [],
#     "conns_active_at": null,
#     "conns_inactive_at": "2025-02-18T22:41:43.534395Z",
#     "tun_type": "cfd_tunnel",
#     "metadata": {},
#     "status": "inactive",
#     "remote_config": true,
#     "credentials_file": {
#       "AccountTag": "699d98642c564d2e855e9661899b7252",
#       "TunnelID": "c1744f8b-faa1-48a4-9e5c-02ac921467fa",
#       "TunnelName": "api-tunnel",
#       "TunnelSecret": "bTSquyUGwLQjYJn8cI8S1h6M6wUc2ajIeT7JotlxI7TqNqdKFhuQwX3O8irSnb=="
#     },
#     "token": "eyJhIjoiNWFiNGU5Z..."
#   }
# }

jq '.result.credentials_file' cfd_tunnel.json > ./cfd_credentials.json
jq --raw-output '.result.id' cfd_tunnel.json
