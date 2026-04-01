#!/bin/bash

set -e

script_path=$(realpath "$0")
script_dir=$(dirname "$script_path")
cd "${script_dir}/.."

source .env

cd "${script_dir}/../certs"

jq -c -n  --arg ca_cname cloudping-hub -f './ca.json.template' > ca.json

jq -c -n  --argjson cname '"cloudping-hub"' --argjson hosts "[ \"${MAIN_DOMAIN}\" ]" -f ./peer.json.template >peer.json

./gen-ca.sh
./gen-cert-pair.sh peer.json
