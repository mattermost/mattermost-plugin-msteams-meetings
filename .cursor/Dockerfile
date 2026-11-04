FROM ubuntu:24.04

ARG GO_VERSION=1.24.6
ARG NODE_VERSION=24.13.1
# Keep NODE_VERSION in sync with .nvmrc.

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates curl git gnupg make sudo xz-utils build-essential \
        iptables fuse-overlayfs \
    && rm -rf /var/lib/apt/lists/*

# Docker Engine and Compose, following Cursor's guidance for running Docker in a cloud agent.
RUN install -m 0755 -d /etc/apt/keyrings \
    && curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu noble stable" > /etc/apt/sources.list.d/docker.list \
    && apt-get update && apt-get install -y --no-install-recommends docker-ce docker-ce-cli containerd.io docker-compose-plugin \
    && rm -rf /var/lib/apt/lists/* \
    && update-alternatives --set iptables /usr/sbin/iptables-legacy \
    && update-alternatives --set ip6tables /usr/sbin/ip6tables-legacy \
    && mkdir -p /etc/docker \
    && echo '{"storage-driver": "fuse-overlayfs"}' > /etc/docker/daemon.json

RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-$(dpkg --print-architecture).tar.gz" | tar -C /usr/local -xz
RUN curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-$(dpkg --print-architecture | sed 's/amd64/x64/').tar.xz" | tar -C /usr/local --strip-components=1 -xJ
ENV PATH="/usr/local/go/bin:/root/go/bin:${PATH}"

# Playwright's browser dependencies are installed with the browser in environment.json.
RUN (id -u ubuntu >/dev/null 2>&1 || useradd -m -s /bin/bash ubuntu) \
    && usermod -aG docker,sudo ubuntu \
    && echo "ubuntu ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/ubuntu

USER ubuntu
ENV PATH="/usr/local/go/bin:/home/ubuntu/go/bin:${PATH}"
WORKDIR /home/ubuntu
