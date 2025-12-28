# My GlobalPing

This is my own little toy project, and especially, it has nothing to do with the more famous and official [globalping.io](https://globalping.io). I wrote this code mainly for personal interest. To me, it's fun to experiment with networking technologies like IP, ICMP or BGP. It's quite meaningful to deal with raw ICMP/IP packets and that is exactly the practice for which I built this project for.

## So, What is My GlobalPing then?

Just as My Traceroute (MTR) combines ping and traceroute features and provides a CLI user interface for continuously displaying network conditions refreshed in real-time, so does My GlobalPing. My GlobalPing also combines ping and traceroute, and provides a web-based UI for continuously displaying network path tracing refreshed in real-time.

In addition, My GlobalPing supports the Internet (also referred as 'Clearnet', in DN42) as well as DN42, at no hassle. It enables you to have a clear picture of how packets traverse the BGP forwarding path. It's a handy learning tool to help you understand networking and routing better.

You can quickly get hands-on with it to see how it works by visiting my own deployed instance of My GlobalPing at the top right of the page, or just click [here](https://globalping.netneighbor.me).

## Features

- Web-based UI for displaying real-time refreshing Ping or Traceroute results, with multiple origin nodes and multiple targets simultaneously.
- Basic rDNS and IPInfoLite-like results for both Clearnet and DN42, such as country, ASN, and AS Name.
- API-first design with RESTful API endpoints and plain-text JSON line stream outputs. Components of our system can be debugged with simple HTTP clients such as curl.

## API Design

The agents respond to HTTP requests that have a path prefixed as `/simpleping`, and the hub responds to HTTP requests that have a path prefixed as `/ping`. Both HTTP request methods are GET, and port numbers are determined by command-line arguments. Parameters are encoded as URL search params.

Refer to [pkg/pinger/request.go](pkg/pinger/request.go) for what parameters are supported, and refer to [pkg/pinger/ping.go](pkg/pinger/ping.go) for the effects of the parameters.

Both `/simpleping` and `/ping` return a stream of JSON lines, so the line feed character can be used as the delimiter.

When sending requests to the hub, targets are encoded in `--url-query targets=` and separated by commas. When sending requests to the agent, only one target is supported at a time, and should be encoded in `--url-query destination`. The `--url-query` option is a syntax sugar provided by curl for easily encoding URL search params.

A client certificate pair is required for calling the agent's API endpoint, which is protected. Every request sent to it is authenticated via mTLS. Just refer to `bin/globalping agent --help` or `bin/globalping hub --help` for how to configure the certificates.

The APIs of the system are not intended to be called directly by end users; only developers should do that.

## Clustering

My GlobalPing system is designed to be distributed. There is a hub and many agents. The hub and agents communicate through mTLS-protected channels. An agent doesn't talk to other agents but only to the hub, and the hub only talks to agents. There is only one hub in a cluster.

Take a look at [docs/how-to-join.md](docs/how-to-join.md) for how to join a new agent to a cluster. It's no more complicated than just advertising itself to the hub.
