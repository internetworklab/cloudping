#!/bin/bash

script_path=$(realpath $0)
script_dir=$(dirname $script_path)

cd $script_dir/..
docker compose -f agent.docker-compose.yaml down
