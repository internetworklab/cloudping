#!/bin/bash

script_path=$(realpath "$0")
script_dir=$(dirname "$script_path")
cd "${script_dir}/../certs"

rm ca.csr
rm ca.json
rm ca.pem
rm ca-key.pem
rm peer.csr
rm peer.json
rm peer.pem
rm peer-key.pem
