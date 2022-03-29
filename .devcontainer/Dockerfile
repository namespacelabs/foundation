FROM golang:1.18.0-bullseye AS go-prebuilt

# Reuse a base image made for devcontainers.
FROM mcr.microsoft.com/vscode/devcontainers/base:bullseye

ARG USERNAME=vscode
ARG GO_VERSION=1.18

RUN apt update -y
RUN apt -y install --no-install-recommends curl xz-utils git

ARG NIX_INSTALL_SCRIPT=https://nixos.org/nix/install
RUN curl -L ${NIX_INSTALL_SCRIPT} | sudo -u ${USERNAME} NIX_INSTALLER_NO_MODIFY_PROFILE=1 sh

# Configuration for Nix from the repository shared amongst developers.
RUN mkdir -p /etc/nix
COPY etc/nix.conf /etc/nix/nix.conf

# This loads the development environment when the container is started.
COPY etc/devcontainer.sh /etc/profile.d/devcontainer.sh

# install bash config
COPY etc/bash.bashrc /etc/bash.bashrc

# set env for non interactve shell to load nix
COPY etc/envrc /etc/envrc
ENV ENV="/etc/envrc" BASH_ENV="/etc/envrc"

# XXX use Nix to get Go
COPY --from=go-prebuilt /usr/local/go /usr/local/go
ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH
RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 777 "$GOPATH"

USER ${USERNAME}

# Rebuilding the container should not throw the Nix store away so we make /nix
# a volume. See `devcontainer.json` later.
VOLUME /nix