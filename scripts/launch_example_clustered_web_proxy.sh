#!/bin/bash

script_path=$(realpath $0)
script_dir=$(dirname $script_path)

cd $script_dir/..

go run ./cmd/globalping web \
  --jwt-auth-secret-from-env="JWT_SECRET"
