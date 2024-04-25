# Base image for build
ARG base_image_version=1.22
# We are doing cross compilation, this speeds things up
# https://www.docker.com/blog/faster-multi-platform-builds-dockerfile-cross-compilation-guide/
FROM --platform=$BUILDPLATFORM golang:${base_image_version} as builder

# Switch workdir
WORKDIR /opt/apm-perf

# Use dedicated layer for Go dependency, cached until they changes
COPY go.mod go.sum ./
RUN go mod download

# Copy files
COPY . .

# Build 
ARG TARGETOS TARGETARCH
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH
ENV CGO_ENABLED=0

# Leverage mounts to cache relevant Go paths
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    make build

# Base image for final image
FROM cgr.dev/chainguard/static

# Arguments
ARG commit_sha
ARG current_time
ARG image_id
ARG project_url

# Labels
LABEL \
  org.opencontainers.image.created="${current_time}" \
  org.opencontainers.image.authors="amannocci <adrien.mannocci@elastic.co>" \
  org.opencontainers.image.url="${project_url}" \
  org.opencontainers.image.documentation="${project_url}" \
  org.opencontainers.image.source="${project_url}" \
  org.opencontainers.image.version="${image_id}" \
  org.opencontainers.image.revision="${commit_sha}" \
  org.opencontainers.image.vendor="Elastic" \
  org.opencontainers.image.licenses="Elastic License 2.0"

# Switch workdir
WORKDIR /opt/apm-perf

# Copy files for apmsoak
COPY --from=builder /opt/apm-perf/dist/apmsoak /usr/bin/apmsoak
COPY ./internal/loadgen/events ./events
COPY ./cmd/apmsoak/scenarios.yml /opt/apm-perf/scenarios.yml

# Copy files for apmbench
COPY --from=builder /opt/apm-perf/dist/apmbench /usr/bin/apmbench

# Default to apmsoak, override to use apmbench
CMD [ "/usr/bin/apmsoak" ]
