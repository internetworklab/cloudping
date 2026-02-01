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
  --address=:28082 \
  --address-public=:8084 \
  --min-pkt-interval=300ms \
  --max-pkt-timeout=3000ms \
  --quic-listen-address=":18447" \
  --jwt-auth-listener-address=":18448" \
  --jwt-auth-listener-cert=/root/services/globalping/hub/certs/peer.pem \
  --jwt-auth-listener-cert-key=/root/services/globalping/hub/certs/peer-key.pem 
