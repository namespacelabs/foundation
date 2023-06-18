{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
    ...
  }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = nixpkgs.legacyPackages.${system};
    in {
      devShell = pkgs.mkShell {
        buildInputs = with pkgs;
          [
            clang-tools # For clang-format.
            go_1_20
            gopls
            go-outline
            go-tools
            upx
            nodejs
            yarn
            git

            crane


            goreleaser

            kubectl
            awscli2
            aws-iam-authenticator
            eksctl
            google-cloud-sdk
            postgresql
          ];
      };
    });
}
