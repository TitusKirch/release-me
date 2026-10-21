# golang:1.27.3-bookworm, matches the go version pinned in mise.toml
FROM golang@sha256:972b4110ba0f47539a975c31e52c64765e1ff19f2d74469ae4a595c6676e80a5 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/

ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.Version=${VERSION}" -o /release-me ./cmd/release-me

# gcr.io/distroless/static:nonroot, runs as the image's built in "nonroot"
# user (65532:65532), so no USER instruction is needed here.
FROM gcr.io/distroless/static@sha256:d62067879de890a1def9da0e7766b7d20810a5e5555cef8506b752437d7a3e88

COPY --from=builder /release-me /release-me

ENTRYPOINT ["/release-me"]
