#!/bin/bash

# Note: our telegram bot implementation currently only
# supports webhook, so you need to have a public domain before
# you can test it.
#
# In our example, I have configured a cloudflared tunnel that
# publish localhost:8084 to https://test-bot.ping2.sh .

go run ./cmd/globalping bot \
 --listen-address=":8084" \
 --public-endpoint="https://test-bot.ping2.sh" \
 --tg-webhook-secret-env="TG_WS_SECRET_TEST" \
 --tg-bot-secret-env="TG_BOT_TOKEN_TEST" \
 --ping-resolver="172.20.0.53:53"
