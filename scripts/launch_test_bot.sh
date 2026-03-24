#!/bin/bash

go run ./cmd/globalping bot \
 --listen-address=":8084" \
 --public-endpoint="https://test-bot.ping2.sh" \
 --tg-webhook-secret-env="TG_WS_SECRET_TEST" \
 --tg-bot-secret-env="TG_BOT_TOKEN_TEST" \
 --ping-resolver="127.0.0.53:53"
