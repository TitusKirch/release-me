# golang:1.27.3-bookworm, matches the go version pinned in mise.toml
FROM --platform=$BUILDPLATFORM golang@sha256:972b4110ba0f47539a975c31e52c64765e1ff19f2d74469ae4a595c6676e80a5 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/

ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags "-s -w -X main.Version=${VERSION}" -o /release-me ./cmd/release-me

# Alpine 3.23.8. The CLI invokes git, including HTTPS and SSH transport.
FROM alpine@sha256:f3d55900449227ae9e237ede521e7b665d53a253b42065ba90b9db9275e198ad

RUN apk add --no-cache git=2.52.0-r0 ca-certificates=20261020-r0 openssh-client-default=10.2_p1-r1 \
    && addgroup -g 65532 release-me \
    && adduser -D -u 65532 -G release-me release-me

ARG VERSION=dev
ARG REVISION=unknown
ARG SOURCE=https://github.com/ostendorf-cloud/release-me
LABEL org.opencontainers.image.source=${SOURCE} \
      org.opencontainers.image.revision=${REVISION} \
      org.opencontainers.image.version=${VERSION}

COPY --from=builder /release-me /release-me
COPY LICENSE NOTICE /usr/share/licenses/release-me/

USER 65532:65532
WORKDIR /repo
ENTRYPOINT ["/release-me"]
