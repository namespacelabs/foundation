#!/bin/bash

# Hack! Move ns into the a dir that's in $PATH
sudo mv /ns /usr/local/bin/ns

# Hack! Reset Gitpod base image path (e.g. to run go)
export PATH=/workspace/go/bin:/home/gitpod/go/bin:/home/gitpod/go-packages/bin:/home/gitpod/.local/bin:/usr/games:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin