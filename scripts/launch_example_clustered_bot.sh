#!/bin/bash

script_path=$(realpath $0)
script_dir=$(dirname $script_path)

cd $script_dir/..

# Note: our telegram bot implementation currently only
# supports webhook, so you need to have a public domain before
# you can test it.
#
# In our example, I have configured a cloudflared tunnel that
# publish localhost:8084 to https://test-bot.ping2.sh .

BOT_PUBLIC_ENDPOINT="https://test-bot.ping2.sh"

go run ./cmd/globalping bot \
  --listen-address=":8087" \
  --public-endpoint="${BOT_PUBLIC_ENDPOINT}" \
  --ping-resolver="127.0.0.53:53" \
  --tg-webhook-secret-env="TG_WS_SECRET_TEST" \
  --tg-bot-secret-env="TG_BOT_TOKEN_TEST" \
  --upstream-jwt-sec-env="UPSTREAM_JWT_TOKEN" \
  --upstream-api-prefix="http://localhost:8084"

# Where:
# --listen-address: The listen port for accepting incoming webhook requests.
# --public-endpoint: As explained above.
# --ping-resolver: The resolver where the agents send queries to
# --upstream-api-prefix: The public listen pport of the hub, see `launch_example_clustered_hub.sh` in the same file
# --tg-webhook-secret-env: Env pointing to shared secret for protecting the bot-to-telegram communication
# --tg-bot-secret-env: Provided by the @botfather
#
