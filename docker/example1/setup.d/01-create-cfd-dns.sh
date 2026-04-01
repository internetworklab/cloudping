#!/bin/bash

# Create DNS record for cloudflared proxied domain, call this after ./create-tunnel.sh

set -e

script_path=$(realpath "$0")
script_dir=$(dirname "$script_path")
cd "${script_dir}/.."

source .env

if [ -z "${CF_API_TOKEN}" ]; then
    echo "No CF_API_TOKEN provided"
    exit 1
fi

if [ -z "${CF_ZONE_ID}" ]; then
    echo "No CF_ZONE_ID provided"
    exit 1
fi

CREDENTIALS_FILE="./cfd_credentials.json"

if [ ! -f "${CREDENTIALS_FILE}" ]; then
    echo "${CREDENTIALS_FILE} not found. Run ./create-tunnel.sh first."
    exit 1
fi

if [ ! -s "${CREDENTIALS_FILE}" ]; then
    echo "${CREDENTIALS_FILE} is empty."
    exit 1
fi

CLOUDFLARED_UUID=$(jq --raw-output '.TunnelID' "${CREDENTIALS_FILE}")

if [ -z "${CLOUDFLARED_UUID}" ]; then
    echo "Failed to extract TunnelID from ${CREDENTIALS_FILE}"
    exit 1
fi

if [ -z "${MAIN_DOMAIN}" ]; then
    echo "MAIN_DOMAIN is not set"
    exit 1
fi

if [ -z "${BOT_DOMAIN}" ]; then
    echo "BOT_DOMAIN is not set"
    exit 1
fi

curl --silent -o - "https://api.cloudflare.com/client/v4/zones/${CF_ZONE_ID}/dns_records" \
  --request POST \
  --header "Authorization: Bearer ${CF_API_TOKEN}" \
  --json "{
    \"type\": \"CNAME\",
    \"proxied\": true,
    \"name\": \"${MAIN_DOMAIN}\",
    \"content\": \"${CLOUDFLARED_UUID}.cfargotunnel.com\"
  }" | jq -r '.result?.id' | tee ./created_dns_resource_ids.txt

curl --silent -o - "https://api.cloudflare.com/client/v4/zones/${CF_ZONE_ID}/dns_records" \
  --request POST \
  --header "Authorization: Bearer ${CF_API_TOKEN}" \
  --json "{
    \"type\": \"CNAME\",
    \"proxied\": true,
    \"name\": \"${BOT_DOMAIN}\",
    \"content\": \"${CLOUDFLARED_UUID}.cfargotunnel.com\"
  }" | jq -r '.result?.id' | tee --append ./created_dns_resource_ids.txt
