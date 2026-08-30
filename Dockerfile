# Cairn builds in two stages that mirror `make build`: the Vue app compiles to
# web/dist, then the Go build embeds that directory into the binary.
#
# The result runs on scratch. modernc.org/sqlite is a pure-Go port with no cgo,
# so the binary has no dynamic links and needs no base image at all -- no libc,
# no shell, nothing with a CVE feed. That is the same property that makes the
# release a single file.
#
# Both build stages pin to BUILDPLATFORM and cross-compile to TARGETARCH, so a
# multi-architecture build runs at native speed instead of under emulation.

FROM --platform=$BUILDPLATFORM node:24-alpine AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm install --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /app/web/dist ./web/dist

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
# -w -s drops DWARF and the symbol table; there is no debugger in the runtime
# image to use them.
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-w -s -X main.version=${VERSION}" -o /cairn ./cmd/cairn

# The data directory is created here so the named volume inherits an ownership
# the unprivileged runtime user can actually write to. A volume mounted over a
# path that does not exist in the image is created root-owned, and SQLite would
# fail on its first write.
RUN mkdir -p /data && chown 65532:65532 /data

FROM scratch
COPY --from=build /cairn /cairn
COPY --from=build --chown=65532:65532 /data /data

USER 65532:65532
VOLUME /data
EXPOSE 7777

LABEL org.opencontainers.image.title="Cairn" \
      org.opencontainers.image.description="An issue tracker that treats the coding agent as a first-class actor." \
      org.opencontainers.image.source="https://github.com/alperkyoruk/cairn" \
      org.opencontainers.image.licenses="Apache-2.0"

# Bound to all interfaces because the container's network namespace is the
# boundary; publish the port to 127.0.0.1 on the host and reach it through a
# reverse proxy.
ENTRYPOINT ["/cairn"]
CMD ["-db", "/data/cairn.db", "-addr", "0.0.0.0:7777"]
