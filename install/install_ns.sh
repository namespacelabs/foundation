#!/bin/sh
# Copyright 2022 Namespace Labs Inc; All rights reserved.
# Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
# available at http://github.com/namespacelabs/foundation

set -e

VERSION="0.0.47"

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

  echo "Executing the installation script for the Namespace CLI"

  if is_darwin; then
    os="darwin"
  elif [ "$(expr substr $(uname -s) 1 5)" = "Linux" ]; then
    os="linux"     
  elif is_wsl; then
    os="linux"
  else
    echo "Unsupported host operating system for the Namespace CLI. Available only for Mac OS X, GNU/Linux and the Windows Subsystem for Linux (WSL)."
    exit 1
  fi

  echo "Detected ${os} as the host operating system"

  case $(uname -m) in
    x86_64) architecture="amd64" ;;
    arm)    dpkg --print-architecture | grep -q "arm64" && architecture="arm64" || architecture="arm" ;;
  esac

  if [ -z $architecture ]; then
    echo "Unsupported platform architecture for the Namespace CLI. Available only on amd64 and arm64 currently."
    exit 1
  fi

  echo "Detected ${architecture} as the platform architecture"

  ns_install="${NS_INSTALL:-$HOME/.ns}"
  bin_dir="$ns_install/bin"
  exe="$bin_dir/ns"

  if [ ! -d "$bin_dir" ]; then
    $sh_c "mkdir -p ${bin_dir}"
  fi

  download_uri="https://ns-releases.s3.us-east-2.amazonaws.com/ns/v${version}/ns_${version}_${os}_${architecture}.tar.gz"

  echo "Downloading and installing the Namespace CLI from ${download_uri}"

  $sh_c "curl --fail --location --progress-bar --output ${exe}.tar.gz ${download_uri}"

  $sh_c "tar -xzf ${exe}.tar.gz -C ${bin_dir}"

  $sh_c "chmod +x ${exe}"

  $sh_c "rm ${exe}.tar.gz"

  echo "Namespace CLI was installed successfully to $exe"

  if ! $dry_run; then 
    if command -v ns >/dev/null; then
      echo "Run 'ns create starter' to get started"
    else
      case $SHELL in
	      /bin/zsh) shell_profile=".zshrc" ;;
	      *) shell_profile=".bashrc" ;;
      esac
      echo "Manually add the directory to your \$HOME/$shell_profile (or similar)"
	    echo "  export NS_INSTALL=\"$ns_install\""
	    echo "  export PATH=\"\$NS_INSTALL/bin:\$PATH\""
	    echo "Run '$exe create starter' to get started"
	  fi
  fi 
}

do_install "$@"
