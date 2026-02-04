#!/bin/bash

script_path=$(realpath $0)
script_dir=$(dirname $script_path)

cd $script_dir/..

bin/globalping hub \
  --peer-ca=https://github.com/internetworklab/cloudping/raw/refs/heads/master/confed/hub/ca.pem \
  --peer-ca=https://github.com/internetworklab/cloudping/raw/refs/heads/master/confed/jason/ca.pem \
  --peer-ca=https://github.com/internetworklab/cloudping/raw/refs/heads/master/confed/moohr/ca.pem \
  --server-cert="/root/services/globalping/hub/certs/peer.pem" \
  --server-cert-key="/root/services/globalping/hub/certs/peer-key.pem" \
  --public-http-listen-address=":8084" \
  --jwt-quic-listen-address=":18448" \
  --min-pkt-interval="300ms" \
  --max-pkt-timeout="3000ms" \
  --jwt-auth-secret-from-env="JWT_SECRET"
