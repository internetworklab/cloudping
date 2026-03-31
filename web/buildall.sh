#!/bin/bash

./buildversion.sh

docker build \
  --push \
  --target prod-nginx \
  --tag ghcr.io/internetworklab/cloudping-web:latest .
