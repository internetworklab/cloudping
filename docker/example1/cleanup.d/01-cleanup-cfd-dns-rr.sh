#!/bin/bash

# Clean up cloudflared tunnel relevant DNS Resource Records
# Call this before 01-cleanup-cfd-tunnel.sh

set -e

script_path=$(realpath "$0")
script_dir=$(dirname "$script_path")
# .env and credentials live one level up from cleanup.d/
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

IDS_FILE="./created_dns_resource_ids.txt"

if [ ! -f "${IDS_FILE}" ]; then
    echo "No ${IDS_FILE} found. Nothing to clean up."
    exit 0
fi

while IFS= read -r DNS_RECORD_ID; do
    if [ -z "${DNS_RECORD_ID}" ] || [ "${DNS_RECORD_ID}" = "null" ]; then
        continue
    fi

    echo "Deleting DNS record ID: ${DNS_RECORD_ID}"
    # See https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/delete/
    curl -s "https://api.cloudflare.com/client/v4/zones/${CF_ZONE_ID}/dns_records/${DNS_RECORD_ID}" \
      --request DELETE \
      --header "Authorization: Bearer ${CF_API_TOKEN}" | jq .
done < "${IDS_FILE}"

rm -f "${IDS_FILE}"
echo "DNS records have been deleted."
