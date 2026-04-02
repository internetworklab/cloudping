# Introduction

This is a **demonstration deployment** of the project. It walks through setting up a Cloudflare Tunnel to expose the services to the internet. The goal is to provide a hands-on, step-by-step example of how to wire up Cloudflare Tunnel with DNS — **not** a production-ready setup.

Demo page: [demo.ping2.sh](https://demo.ping2.sh/)

Demo bot: [@cloudping_test_bot](http://t.me/cloudping_test_bot)

Follow the steps below to get the demo running, and see the [Cleaning Up](#cleaning-up) section when you're done.

# How to Setup

All environment variables should be defined in a `.env` file in the same directory as the `docker-compose.yaml`. See `.env.example` for reference.

The setup scripts live in `setup.d/` and **must be run in order**.

## Step 1: Create the Cloudflare Tunnel

Run `setup.d/00-create-tunnel.sh` with the following:

**API Token Permissions (at least one required):**

- `Cloudflare One Connectors Write`
- `Cloudflare One Connector: cloudflared Write`
- `Cloudflare Tunnel Write`

More on: https://developers.cloudflare.com/api/resources/zero_trust/subresources/tunnels/subresources/cloudflared/methods/create/

**Required ENVs (in `.env`):**

| Variable         | Description                          |
|------------------|--------------------------------------|
| `CF_ACCOUNT_ID`  | Your Cloudflare account ID           |
| `CF_API_TOKEN`   | Cloudflare API token with tunnel permissions |
| `TUNNEL_NAME`    | Name for the new tunnel              |

**Example:**

```bash
./setup.d/00-create-tunnel.sh
```

This outputs a tunnel UUID and generates `cfd_tunnel.json` and `cfd_credentials.json`.

## Step 2: Create the Cloudflare DNS Records

Run `setup.d/01-create-cfd-dns.sh` with the following:

**API Token Permissions (at least one required):**

- `DNS Write`

More on: https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/create

**Required ENVs (in `.env`):**

| Variable           | Description                                      |
|--------------------|--------------------------------------------------|
| `CF_API_TOKEN`     | Cloudflare API token with DNS edit permissions    |
| `CF_ZONE_ID`       | Cloudflare zone ID for your domain                |
| `MAIN_DOMAIN`      | The domain name for the web app CNAME record (e.g. `app.example.com`) |
| `BOT_DOMAIN`       | The domain name for the Telegram bot webhook CNAME record (e.g. `bot.example.com`) |

> **Note:** The tunnel UUID is read automatically from `./cfd_credentials.json` (generated in Step 1). No need to set it manually.

**Example:**

```bash
./setup.d/01-create-cfd-dns.sh
```

This creates **two** proxied CNAME records:
- `MAIN_DOMAIN` → `<TUNNEL_UUID>.cfargotunnel.com`
- `BOT_DOMAIN` → `<TUNNEL_UUID>.cfargotunnel.com`

The DNS record IDs are appended to `./created_dns_resource_ids.txt`, which is used later by the cleanup scripts.

## Step 3: Generate Self-Signed Certificates

Run `setup.d/02-create-self-sign-certs.sh` to generate the mTLS certificates required by the hub and agent services:

```bash
./setup.d/02-create-self-sign-certs.sh
```

This generates certificates inside the `certs/` directory using templates (`ca.json.template`, `peer.json.template`) and helper scripts (`gen-ca.sh`, `gen-cert-pair.sh`) located there.

> **Note:** This script reads `MAIN_DOMAIN` from `.env` to include it in the certificate's SAN (Subject Alternative Name).

## Step 4: Configure `cloudflared.conf.yaml`

Now that you have the tunnel UUID from Step 1, update `cloudflared.conf.yaml` to match your setup:

| Field              | Description                                                                 |
|--------------------|-----------------------------------------------------------------------------|
| `tunnel`           | Replace with the tunnel UUID obtained from Step 1                           |
| `credentials-file` | Path to `cfd_credentials.json` inside the container (default: `/app/cfd_credentials.json`) |
| `ingress`          | Update the `hostname` and `service` rules to match your domains and backend services |

The config includes two ingress rules — one for the web frontend and one for the Telegram bot webhook:

```yaml
ingress:
  - hostname: <MAIN_DOMAIN>
    service: http://cloudping-demo-web:8080
  - hostname: <BOT_DOMAIN>
    service: http://cloudping-demo-bot:8093
  - service: http_status:404
```

## Step 5: Start the Services

```bash
docker compose up -d
```

This starts all services defined in `docker-compose.yaml`, including the Cloudflare Tunnel (`cloudflared`), web frontend (`web`), Telegram bot (`bot`), measurement hub (`hub`), and a probe agent (`agent1`).

# Cleaning Up

When you're done with the demo, run the cleanup scripts in `cleanup.d/` to tear down the resources created during setup. **The scripts must be run in order** — DNS records should be removed before the tunnel is deleted.

```bash
# Step 1: Delete the transient X.509 certs and key
./cleanup.d/00-cleanup-temp-certs.sh

# Step 2: Delete the DNS record(s)
./cleanup.d/01-cleanup-cfd-dns-rr.sh

# Step 3: Delete the Cloudflare Tunnel
./cleanup.d/02-cleanup-cfd-tunnel.sh
```

> **Note:** `01-cleanup-cfd-dns-rr.sh` reads DNS record IDs from `./created_dns_resource_ids.txt` (generated during [Step 2](#step-2-create-the-cloudflare-dns-records)). If this file is missing, the script will exit gracefully with nothing to clean up.

# See Also

1. https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/get-started/create-remote-tunnel-api/
2. https://developers.cloudflare.com/api/resources/zero_trust/subresources/tunnels/subresources/cloudflared/methods/create/
3. https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/do-more-with-tunnels/local-management/configuration-file/
