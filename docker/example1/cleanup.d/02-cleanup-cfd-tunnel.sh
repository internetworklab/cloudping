#!/bin/bash

# Clean up cloudflared tunnel object created for this demo

set -e

script_path=$(realpath $0)
script_dir=$(dirname $script_path)
cd "${script_dir}/.."

source .env

if [ -z "${CF_API_TOKEN}" ]; then
    echo "No CF_API_TOKEN provided"
    exit 1
fi

if [ -z "${CF_ACCOUNT_ID}" ]; then
    echo "No CF_ACCOUNT_ID provided"
    exit 1
fi

CREDENTIALS_FILE="./cfd_credentials.json"

if [ ! -f "${CREDENTIALS_FILE}" ]; then
    echo "${CREDENTIALS_FILE} not found. Nothing to clean up."
    exit 0
fi

if [ ! -s "${CREDENTIALS_FILE}" ]; then
    echo "${CREDENTIALS_FILE} is empty. Nothing to clean up."
    exit 0
fi

TUNNEL_ID=$(jq --raw-output '.TunnelID' "${CREDENTIALS_FILE}")

if [ -z "${TUNNEL_ID}" ] || [ "${TUNNEL_ID}" = "null" ]; then
    echo "Failed to extract TunnelID from ${CREDENTIALS_FILE}"
    exit 1
fi

echo "Deleting tunnel ${TUNNEL_ID} ..."

# see https://developers.cloudflare.com/api/resources/zero_trust/subresources/tunnels/subresources/cloudflared/methods/delete/
curl "https://api.cloudflare.com/client/v4/accounts/${CF_ACCOUNT_ID}/cfd_tunnel/${TUNNEL_ID}" \
  --request DELETE \
  --header "Authorization: Bearer ${CF_API_TOKEN}"

echo ""
echo "Tunnel ${TUNNEL_ID} deleted."
