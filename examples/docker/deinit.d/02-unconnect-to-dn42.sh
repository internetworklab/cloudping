#!/bin/bash

script_path=$(realpath $0)
script_dir=$(dirname $script_path)

source $script_dir/../.env

pid1=$(docker inspect bird -f {{.State.Pid}})
if [ -z "$pid1" ]; then
    echo "Error: Failed to get pid of bird container"
    exit 1
fi

echo "bird pid: $pid1"

containername="globalping-agent"

pid2=$(docker inspect $containername -f {{.State.Pid}})
if [ -z "$pid2" ]; then
    echo "Error: Failed to get pid of $containername container"
    exit 1
fi

echo "$containername pid: $pid2"

nsenter -t $pid1 -n ip l del v-gping 2>/dev/null || true
nsenter -t $pid2 -n ip l del v-bird 2>/dev/null || true
