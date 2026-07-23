ARG GO_VERSION=1.26
ARG BUSYBOX_VERSION=1.38

FROM --platform=$BUILDPLATFORM docker.io/library/golang:$GO_VERSION-trixie AS build

ARG TARGETARCH
ARG TARGETOS
# hadolint ignore=DL3008
RUN set -eux; \
    apt-get update; \
    apt-get install --no-install-recommends -y ca-certificates; \
    apt-get clean; \
    rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY Makefile go.mod go.sum ./
RUN make download
COPY . .
RUN make build NATIVE_GOOS="$TARGETOS" NATIVE_GOARCH="$TARGETARCH"


FROM docker.io/library/busybox:$BUSYBOX_VERSION-musl
COPY --from=build /src/build/regdoc /bin/regdoc
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/bin/regdoc"]
CMD ["version"]
