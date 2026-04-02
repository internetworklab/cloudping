#!/bin/bash

./buildversion.sh

docker build \
  --target prod-nginx \
  --build-arg NEXT_PUBLIC_DEFAULT_RESOLVER=172.20.0.53:53 \
  --tag cloudping-web:nginx .
