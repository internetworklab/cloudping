FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS builder-basis
ARG TARGETOS
ARG TARGETARCH

WORKDIR /app/globalping

COPY go.mod go.mod
COPY go.sum go.sum

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go mod download

FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS builder
ARG TARGETOS
ARG TARGETARCH

COPY --from=builder-basis /go/pkg /go/pkg

WORKDIR /app/globalping

COPY . .

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o bin/globalping ./cmd/globalping

FROM debian:bookworm

RUN \
  apt-get update -y && apt-get install -y ca-certificates 

COPY --from=builder /app/globalping/bin/globalping /usr/local/bin/globalping
