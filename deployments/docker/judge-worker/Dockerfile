# Multi-stage Dockerfile for Judge Worker with Dual-Layer Sandbox
FROM ubuntu:22.04 AS judge-base

# Avoid prompts from apt
ENV DEBIAN_FRONTEND=noninteractive

# Install system dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    gcc \
    g++ \
    openjdk-17-jdk \
    python3 \
    python3-pip \
    nodejs \
    npm \
    golang-go \
    rustc \
    isolate \
    libcap-dev \
    libseccomp-dev \
    cgroup-tools \
    quota \
    curl \
    wget \
    git \
    ca-certificates \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Install additional compilers and runtimes
RUN pip3 install --no-cache-dir pypy3

# Set up Go environment
ENV GO_VERSION=1.21.0
ENV GOPATH=/go
ENV PATH=$GOPATH/bin:/usr/local/go/bin:$PATH

# Create directories for the sandbox
RUN mkdir -p /var/local/lib/isolate \
    && mkdir -p /opt/judge \
    && mkdir -p /tmp/judge-work

# Configure isolate sandbox
RUN echo "# Isolate configuration for judge worker" > /usr/local/etc/isolate.conf \
    && echo "box_count = 256" >> /usr/local/etc/isolate.conf \
    && echo "cg_enable = true" >> /usr/local/etc/isolate.conf \
    && echo "cg_memory_limit = 512M" >> /usr/local/etc/isolate.conf \
    && echo "cg_timing = true" >> /usr/local/etc/isolate.conf

# Create judge user with limited privileges
RUN groupadd -r judge && useradd -r -g judge -d /opt/judge -s /bin/bash judge

# Set up cgroups for resource limiting
RUN mkdir -p /sys/fs/cgroup/memory/judge \
    && mkdir -p /sys/fs/cgroup/cpuacct/judge \
    && mkdir -p /sys/fs/cgroup/pids/judge

# Build stage for Go application
FROM judge-base AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the judge worker
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo \
    -ldflags '-extldflags "-static"' \
    -o judge-worker ./cmd/judge-worker/

# Production stage
FROM judge-base AS production

# Copy the built binary
COPY --from=builder /app/judge-worker /opt/judge/

# Copy configuration files
COPY deployments/docker/judge-worker/entrypoint.sh /opt/judge/
COPY deployments/docker/judge-worker/seccomp-profile.json /opt/judge/
COPY deployments/docker/judge-worker/isolate.conf /usr/local/etc/

# Set permissions
RUN chmod +x /opt/judge/entrypoint.sh \
    && chmod +x /opt/judge/judge-worker \
    && chown -R judge:judge /opt/judge

# Create volume mount points
VOLUME ["/var/local/lib/isolate", "/tmp/judge-work"]

# Expose health check port
EXPOSE 8081

# Set up environment
ENV JUDGE_WORK_DIR=/tmp/judge-work
ENV ISOLATE_CONFIG=/usr/local/etc/isolate.conf
ENV SECCOMP_PROFILE=/opt/judge/seccomp-profile.json

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8081/health || exit 1

# Use judge user
USER judge

# Set working directory
WORKDIR /opt/judge

# Entry point
ENTRYPOINT ["/opt/judge/entrypoint.sh"]
CMD ["/opt/judge/judge-worker"]