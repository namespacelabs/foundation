#!/bin/sh
# Copyright 2022 Namespace Labs Inc; All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.

set -eu

tool_name="nsc"
docker_cred_helper_name="docker-credential-nsc"
bazel_cred_helper_name="bazel-credential-nsc"

is_wsl() {
	case "$(uname -r)" in
	*microsoft* ) true ;; # WSL 2
	*Microsoft* ) true ;; # WSL 1
	* ) false;;
	esac
}

is_darwin() {
	case "$(uname -s)" in
	*darwin* ) true ;;
	*Darwin* ) true ;;
	* ) false;;
	esac
}

do_install() {
  dry_run=false

  while [ $# -gt 0 ]; do
    case "$1" in
	  --dry_run)
	    dry_run=true
	    ;;

      -v)
        version="$2"
        ;;

      --version)
        version="$2"
        ;;

      --*)
        echo "Illegal option $1"
        ;;
    esac
    shift $(( $# > 0 ? 1 : 0 ))
  done

  sh_c="sh -c"
  if $dry_run; then
    sh_c="echo"
  fi

  echo "Executing Namespace's installation script..."

  if is_darwin; then
    os="darwin"
  elif [ "$(expr substr $(uname -s) 1 5)" = "Linux" ]; then
    os="linux"
  elif is_wsl; then
    os="linux"
  else
    echo "Unsupported host operating system. Available only for Mac OS X, GNU/Linux and the Windows Subsystem for Linux (WSL)."
    exit 1
  fi

  echo "Detected ${os} as the host operating system"

  architecture=''
  case $(uname -m) in
    x86_64) architecture="amd64" ;;
    arm64|aarch64) architecture="arm64" ;;
    arm)    dpkg --print-architecture | grep -q "arm64" && architecture="arm64" || architecture="arm" ;;
  esac

  if [ -z $architecture ]; then
    echo "Unsupported platform architecture. Available only on amd64 and arm64 currently."
    exit 1
  fi

  echo "Detected ${architecture} as the platform architecture"

  # Determine bin_dir
  case "$os" in
    darwin) bin_dir="$HOME/Library/Application Support/ns/bin" ;;
    linux) bin_dir="$HOME/.ns/bin" ;;
  esac

  if [ ! -z "${NS_ROOT:-}" ]; then
    bin_dir="$NS_ROOT/bin"
  elif [ ! -z "${NS_INSTALL_DIR:-}" ]; then
    bin_dir="$NS_INSTALL_DIR"
  fi

  # If no version specified, query the latest version
  if [ -z "${version:-}" ]; then
    echo "Querying latest version..."
    api_os=$(echo "$os" | tr '[:lower:]' '[:upper:]')
    api_arch=$(echo "$architecture" | tr '[:lower:]' '[:upper:]')

    version_response=$(curl --fail --silent --location \
      -X POST \
      -H "Content-Type: application/json" \
      -d "{\"${tool_name}\":{}}" \
      "https://get.namespace.so/nsl.versions.VersionsService/GetLatest")

    version=$(echo "$version_response" | sed -n 's/.*"version":"\([^"]*\)".*/\1/p')
    # Strip leading "v" if present
    version=$(echo "$version" | sed 's/^v//')

    if [ -z "$version" ]; then
      echo "Failed to query latest version"
      exit 1
    fi

    echo "Latest version: ${version}"
  fi

  download_uri="https://get.namespace.so/packages/${tool_name}/v${version}/${tool_name}_${version}_${os}_${architecture}.tar.gz"

  echo "Downloading and installing Namespace ${version} from ${download_uri}"

  ci_header=""
  if [ ! -z "${CI:-}" ]; then
    ci_header="-H 'CI: ${CI}'"
  fi

  TEMP_DIR=$(mktemp -d)
  trap "rm -rf $TEMP_DIR" EXIT

  cd ${TEMP_DIR}
  $sh_c "curl $ci_header --fail --location --progress-bar --user-agent install_nsc.sh \"${download_uri}\" | tar -xz"

  $sh_c "chmod +x ${tool_name}"
  $sh_c "chmod +x ${docker_cred_helper_name}"
  $sh_c "chmod +x ${bazel_cred_helper_name}"

  # Version comparison: check if version >= 0.0.462 (supports "nsc install")
  version_major=$(echo "$version" | cut -d. -f1)
  version_minor=$(echo "$version" | cut -d. -f2)
  version_patch=$(echo "$version" | cut -d. -f3)

  supports_install=false
  if [ "$version_major" -gt 0 ] 2>/dev/null; then
    supports_install=true
  elif [ "$version_major" -eq 0 ] && [ "$version_minor" -gt 0 ] 2>/dev/null; then
    supports_install=true
  elif [ "$version_major" -eq 0 ] && [ "$version_minor" -eq 0 ] && [ "$version_patch" -ge 462 ] 2>/dev/null; then
    supports_install=true
  fi

  if $supports_install; then
    echo "Installing Namespace ${version} using install command"
    install_args="--dir=\"${bin_dir}\""

    $sh_c "./${docker_cred_helper_name} install $install_args"
    $sh_c "./${bazel_cred_helper_name} install $install_args"
    $sh_c "./${tool_name} install $install_args"
  else
    echo "Installing Namespace ${version} to ${bin_dir}"
    $sh_c "mkdir -p \"${bin_dir}\""
    $sh_c "cp ${tool_name} \"${bin_dir}/\""
    echo "✓ Installed ${tool_name} to ${bin_dir}/${tool_name}"
    $sh_c "cp ${docker_cred_helper_name} \"${bin_dir}/\""
    echo "✓ Installed ${docker_cred_helper_name} to ${bin_dir}/${docker_cred_helper_name}"
    $sh_c "cp ${bazel_cred_helper_name} \"${bin_dir}/\""
    echo "✓ Installed ${bazel_cred_helper_name} to ${bin_dir}/${bazel_cred_helper_name}"
  fi

  echo "Installation complete"
}

do_install "$@"
