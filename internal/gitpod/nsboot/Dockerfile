# slimmer than gitpod/workspace-full, includes base, Go, Docker and Tailscale.
FROM gitpod/workspace-go

# install nsboot to get automatic updates
# Currently fails with:
#
#   2ff4dc5 ...ev/foundation/internal/nsboot/gitpod /home/linuxbrew/.linuxbrew/Homebrew/Library/Taps/namespacelabs/homebrew-namespace/nsboot.rb:35: syntax error, unexpected tIDENTIFIER, expecting end
#   2ff4dc5 ...ev/foundation/internal/nsboot/gitpod ...   libexec.install "nsboot" bin.install_symlink libexec/"nsb...
USER gitpod
RUN NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
RUN /home/linuxbrew/.linuxbrew/bin/brew install namespacelabs/namespace/ns
