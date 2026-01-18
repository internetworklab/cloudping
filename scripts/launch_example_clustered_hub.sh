#!/bin/bash

script_path=$(realpath $0)
script_dir=$(dirname $script_path)

cd $script_dir/..

bin/globalping hub \
  --peer-c-as=https://github.com/internetworklab/globalping/raw/refs/heads/master/confed/hub/ca.pem \
  --peer-c-as=https://github.com/internetworklab/globalping/raw/refs/heads/master/confed/jason/ca.pem \
  --peer-c-as=https://github.com/internetworklab/globalping/raw/refs/heads/master/confed/moohr/ca.pem \
  --client-cert=/root/services/globalping/hub/certs/peer.pem \
  --client-cert-key=/root/services/globalping/hub/certs/peer-key.pem \
  --server-cert=/root/services/globalping/hub/certs/peer.pem \
  --server-cert-key=/root/services/globalping/hub/certs/peer-key.pem \
  --web-socket-path=/ws \
  --address=:28080 \
  --address-public=:8082 \
  --min-pkt-interval=300ms \
  --max-pkt-timeout=3000ms
