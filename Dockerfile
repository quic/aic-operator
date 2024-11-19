# Build the manager binary
FROM registry.access.redhat.com/ubi9/go-toolset:1.21 AS builder
ARG TARGETOS
ARG TARGETARCH

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY internal/controller/ internal/controller/
COPY internal/kmmmodule/ internal/kmmmodule/

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager cmd/main.go

FROM registry.access.redhat.com/ubi9/ubi-minimal:9.4
WORKDIR /
COPY --from=builder /opt/app-root/src/manager .

# Include license and location information
LABEL org.opencontainers.image.source https://github.com/quic/cloud-ai-containers
COPY --chmod=755 manual_install/src_img_build/motd.sh /etc
COPY manual_install/src_img_build/CONTAINER_LICENSE.txt /usr/share/doc
RUN ln -s /usr/share/doc/CONTAINER_LICENSE.txt / && \
    ln -s /usr/share/doc/CONTAINER_LICENSE.txt /root && \
# Different systems use different system wide non-login shell config files
    if [ -e /etc/bash.bashrc ]; then \
        echo '[ ! -z "$TERM" -a -r /etc/motd.sh ] && /etc/motd.sh' >> /etc/bash.bashrc ; \
    elif [ -e /etc/bashrc ]; then \
        echo '[ ! -z "$TERM" -a -r /etc/motd.sh ] && /etc/motd.sh' >> /etc/bashrc ; \
    else \
        echo "Unable to find system bashrc" ; \
        exit 1 ; \
    fi

USER 65532:65532
ENTRYPOINT ["/manager"]
