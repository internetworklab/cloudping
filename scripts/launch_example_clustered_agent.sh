#!/bin/bash

script_path=$(realpath $0)
script_dir=$(dirname $script_path)

cd $script_dir/..

# --server-name="globalping-hub.exploro.one" \

bin/globalping agent \
  --node-name="vie1" \
  --peer-ca="https://github.com/internetworklab/globalping/raw/refs/heads/master/confed/hub/ca.pem" \
  --quic-server-address="globalping-hub.exploro.one:18448" \
  --exact-location-lat-lon="48.1952,16.3503" 
