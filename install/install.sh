#!/bin/sh
# Copyright 2022 Namespace Labs Inc; All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.

set -e

VERSION=0.0.73

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

  if [ -z "$version" ]; then
    version="$VERSION"
  fi

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

  case $(uname -m) in
    x86_64) architecture="amd64" ;;
    arm64) architecture="arm64" ;;
    arm)    dpkg --print-architecture | grep -q "arm64" && architecture="arm64" || architecture="arm" ;;
  esac

  if [ -z $architecture ]; then
    echo "Unsupported platform architecture. Available only on amd64 and arm64 currently."
    exit 1
  fi

  echo "Detected ${architecture} as the platform architecture"

  ns_root="${NS_ROOT:-$HOME/.ns}"
  bin_dir="$ns_root/bin"
  temp_tar="$(mktemp)"

  if [ ! -d "$bin_dir" ]; then
    $sh_c "mkdir -p ${bin_dir}"
  fi

  download_uri="https://get.namespace.so/packages/nsboot/v${version}/nsboot_${version}_${os}_${architecture}.tar.gz"

  echo "Downloading and installing Namespace from ${download_uri}"

  $sh_c "curl --fail --location --progress-bar --user-agent install.sh --output ${temp_tar} ${download_uri}"

  $sh_c "tar -xzf ${temp_tar} -C ${bin_dir}"

  $sh_c "chmod +x ${bin_dir}/nsboot"

  $sh_c "ln -sf ./nsboot ${bin_dir}/ns"

  $sh_c "rm ${temp_tar}"

  echo
  echo "Namespace was successfully installed to ${bin_dir}/ns"
  echo

  if ! $dry_run; then
    if command -v ns >/dev/null; then
      echo "Check out our examples at https://namespace.so/docs#examples to get started."
      echo
    else
      case $SHELL in
	      /bin/zsh) shell_profile=".zshrc" ;;
	      *) shell_profile=".bashrc" ;;
      esac
      echo "Manually add the directory to your \$HOME/$shell_profile (or similar)"
	    echo "  export NS_ROOT=\"$ns_root\""
	    echo "  export PATH=\"\$NS_ROOT/bin:\$PATH\""
            echo
            echo "Check out our examples at https://namespace.so/docs#examples to get started."
            echo
	  fi
  fi
}

do_install "$@"
