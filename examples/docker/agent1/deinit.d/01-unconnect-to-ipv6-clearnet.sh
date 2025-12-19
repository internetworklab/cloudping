#!/bin/bash

script_path=$(realpath $0)
script_dir=$(dirname $script_path)

source $script_dir/../.env

containername="globalping-agent"

pid2=$(docker inspect $containername -f {{.State.Pid}})
if [ -z "$pid2" ]; then
    echo "Error: Failed to get pid of $containername container"
    exit 1
fi

echo "$containername pid: $pid2"

nsenter -t $pid2 -n ip l del v-host 2>/dev/null || true
ip l del v-gping 2>/dev/null || true
