#!/bin/bash

script_path=$(realpath $0)
script_dir=$(dirname $script_path)

cd $script_dir/..

bin/globalping agent \
  --node-name="lax1" \
  --exact-location-lat-lon="48.1952,16.3503" \
  --country-code="US" \
  --city-name="Los Angeles" \
  --asn="AS35916" \
  --isp="MULTACOM" \
  --dn42-asn="AS4242421771" \
  --dn42-isp="DUSTSTARS"
