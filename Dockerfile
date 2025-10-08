# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.25

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS builder
WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY . .

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/trustd /src

FROM alpine AS final

COPY --from=builder /out/trustd /trustd

USER 65532:65532
ENTRYPOINT ["/trustd"]

