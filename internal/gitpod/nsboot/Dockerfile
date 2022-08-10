# slimmer than gitpod/workspace-full, includes base, Go, Docker and Tailscale.
FROM gitpod/workspace-go

USER gitpod

# install nsboot to get automatic updates
RUN NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/c017ced9ca817138cc03acabb59454a0a0ca889e/install.sh)"
RUN /home/linuxbrew/.linuxbrew/bin/brew install namespacelabs/namespace/ns

# 1. Adding the homebrew dir to the PATH so "ns" can be found.
#
# 2. Adding current $PATH to .bashrc so it survives "layer_build_plan" merging that copies only files
#    from layers but not environment variables.
RUN echo "export PATH=${PATH}:/home/linuxbrew/.linuxbrew/bin:\$PATH" > $HOME/.bashrc.d/ns-path
