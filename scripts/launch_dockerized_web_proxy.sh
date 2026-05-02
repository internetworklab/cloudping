#!/bin/bash

script_path=$(realpath $0)
script_dir=$(dirname $script_path)

cd $script_dir/..

docker run \
  --name web-proxy-dev \
  --env-file ./.env \
  --add-host "host.docker.internal:host-gateway" \
  -v go-dev-vol:/go \
  -v ./:/root/projects/globalping \
  -v ./.env:/root/projects/globalping/.env \
  -w /root/projects/globalping \
  -p 8092:8092 \
  --rm \
  -it \
  docker.io/library/golang:latest \
    go run ./cmd/globalping web \
      --listen-address=":8092" \
      --jwt-auth-secret-from-env="JWT_SECRET" \
      --github-oauth-client-id-from-env="GITHUB_OAUTH_CLIENT_ID" \
      --github-oauth-app-secret-from-env="GITHUB_OAUTH_APP_SECRET" \
      --github-oauth-redir-url="http://localhost:8092/login/as/github/auth" \
      --google-oauth-client-id-from-env="GOOGLE_OAUTH_CLIENT_ID" \
      --google-oauth-client-secret-from-env="GOOGLE_OAUTH_CLIENT_SECRET" \
      --google-oauth-redir-url="http://localhost:8092/login/as/google/auth" \
      --iedon-oauth-client-id-from-env="IEDON_OAUTH_CLIENT_ID" \
      --iedon-oauth-client-secret-from-env="IEDON_OAUTH_CLIENT_SECRET" \
      --iedon-oauth-redir-url="http://localhost:8092/login/as/iedon/auth" \
      --kioubit-oauth-client-id-from-env="KIOUBIT_OAUTH_CLIENT_ID" \
      --kioubit-oauth-client-secret-from-env="KIOUBIT_OAUTH_CLIENT_SECRET" \
      --kioubit-oauth-redir-url="http://localhost:8092/login/as/kioubit/auth" \
      --entra-id-tenant-id-from-env="ENTRA_ID_TENANT_ID" \
      --entra-id-client-id-from-env="ENTRA_ID_CLIENT_ID" \
      --entra-id-client-secret-from-env="ENTRA_ID_CLIENT_SECRET" \
      --entra-id-redir-url="http://localhost:8092/login/as/entra/auth" \
      --default-backend="http://host.docker.internal:45844" \
      --add-white-list-path="/_next/" \
      --add-white-list-path="/login/as/" \
      --add-white-list-path="/login/" \
      --add-white-list-path="/login"
