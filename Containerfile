# Base image for build
ARG base_image_version=1.20.7
FROM golang:${base_image_version} as builder

# Switch workdir
WORKDIR /opt/apm-perf

# Copy files
COPY . .

# Build 
RUN \
  go mod download \
  && make build

# Base image for build
FROM debian:bookworm

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

# Add minimal stuff
RUN \
  apt-get update > /dev/null \
  && apt-get upgrade -y \
  && apt-get install -y --no-install-recommends \
    "apt-utils=*" \
    "ca-certificates=*" \
    "curl=*" \
    "gnupg=*" \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/*

# Copy files for apmsoak
COPY --from=builder /opt/apm-perf/dist/apmsoak /usr/bin/apmsoak
COPY ./internal/loadgen/events ./events
COPY ./cmd/apmsoak/scenarios.yml /opt/apm-perf/scenarios.yml

# Copy files for apmbench
COPY --from=builder /opt/apm-perf/dist/apmbench /usr/bin/apmbench

# Default to apmsoak, override to use apmbench
CMD [ "/usr/bin/apmsoak" ]
