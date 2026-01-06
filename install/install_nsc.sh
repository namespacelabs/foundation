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

  TEMP_DIR=$(mktemp -d)
  trap "rm -rf $TEMP_DIR" EXIT

  download_uri="https://get.namespace.so/packages/${tool_name}/latest?arch=${architecture}&os=${os}"
  if [ ! -z "${version:-}" ]; then
    download_uri="https://get.namespace.so/packages/${tool_name}/v${version}/${tool_name}_${version}_${os}_${architecture}.tar.gz"
  fi

  echo "Downloading and installing Namespace from ${download_uri}"

  ci_header=""
  if [ ! -z "${CI:-}" ]; then
    ci_header="-H 'CI: ${CI}'"
  fi

  cd ${TEMP_DIR}
  $sh_c "curl $ci_header --fail --location --progress-bar --user-agent install_nsc.sh \"${download_uri}\" | tar -xz"

  $sh_c "chmod +x ${tool_name}"
  $sh_c "chmod +x ${docker_cred_helper_name}"
  $sh_c "chmod +x ${bazel_cred_helper_name}"

  # Run install commands to handle installation
  echo "Installing..."
  $sh_c "./${tool_name} install"
  $sh_c "./${docker_cred_helper_name} install"
  $sh_c "./${bazel_cred_helper_name} install"

  # Clean up the temporary binary
  $sh_c "rm ${tool_name}"
  $sh_c "rm ${docker_cred_helper_name}"
  $sh_c "rm ${bazel_cred_helper_name}"

  echo "Installation complete"
}

do_install "$@"
