{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = {
    nixpkgs,
    flake-utils,
    ...
  }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = nixpkgs.legacyPackages.${system};
    in {
      devShell = pkgs.mkShell {
        buildInputs = with pkgs; [
          go_1_21
          gopls
          go-outline
          go-tools
          upx
          yarn
          pre-commit

          goreleaser
          gh

          eksctl
          postgresql

          git
          nodejs
          crane
          kubectl
          awscli2
          jq
          aws-iam-authenticator
          google-cloud-sdk

          (writeShellScriptBin "clang-format" ''
            ${
              lib.getExe' clang-tools "clang-format"
            } --style="${
              (builtins.fromJSON (lib.readFile ./.vscode/settings.json))."clang-format.style"
            }" $@
          '')
          clang-tools # everything except clang-format
        ];
      };
    });
}
