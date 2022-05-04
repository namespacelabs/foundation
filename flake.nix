{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs";
    flake-utils.url = "github:numtide/flake-utils";
    alejandra.url = "github:kamadorueda/alejandra/1.1.0";
    alejandra.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = {
    self,
    nixpkgs,
    alejandra,
    flake-utils,
  }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = nixpkgs.legacyPackages.${system};
    in {
      devShell = pkgs.mkShell {
        buildInputs = with pkgs;
          [
            clang-tools # For clang-format.
            go_1_18
            upx
            nodejs-16_x
            yarn
            docker-client # Required by `fn prepare`.

            pre-commit
            golangci-lint

            goreleaser

            kubectl
            awscli2
            aws-iam-authenticator
            eksctl
            google-cloud-sdk
          ]
          ++ [
            alejandra.defaultPackage.${system} # For nix file formatting. Bit meta.
          ];
      };
    });
}
