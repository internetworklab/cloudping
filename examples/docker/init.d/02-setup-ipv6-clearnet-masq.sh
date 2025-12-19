#!/bin/bash

script_path=$(realpath $0)
script_dir=$(dirname $script_path)

source $script_dir/../.env

if [ -z "$DN42_IPV6" ]; then
    echo "Error: DN42_IPV6 is not set"
    exit 1
fi

nft delete chain ip6 nat gping-masq 2>/dev/null || true
nft add chain ip6 nat gping-masq { type nat hook postrouting priority srcnat ';' policy accept ';' } 
nft add rule ip6 nat gping-masq ip6 saddr $DN42_IPV6/128 masquerade
