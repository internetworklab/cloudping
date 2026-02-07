#!/bin/bash

./buildversion.sh

docker build \
  --push \
  --tag ghcr.io/internetworklab/cloudping-web:latest .
