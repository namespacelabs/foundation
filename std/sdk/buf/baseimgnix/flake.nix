{
  inputs = {
    flake-utils.url = "github:numtide/flake-utils";
    nixpkgs.url = "nixpkgs/nixos-21.11";
  };
  outputs = {
    self,
    nixpkgs,
    flake-utils,
  }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = nixpkgs.legacyPackages.${system};
      grpc-gateway-src = pkgs.fetchFromGitHub {
        owner = "grpc-ecosystem";
        repo = "grpc-gateway";
        rev = "fa67768a440ff01c662ed62e92a157ae2621d158";
        sha256 = "sha256-Zz0LaQN4g7v97R2uyu03BCf6t2fqFnnfjFlju4XyB8g=";
      };
      # See https://ryantm.github.io/nixpkgs/languages-frameworks/go/
      grpc-gateway = pkgs.buildGoModule {
        name = "grpc-gateway";
        src = grpc-gateway-src;
        vendorSha256 = "sha256-2K5mRrjGLta2N63hAwsBZXME8vSxIoXWubJe2eZRWP0=";
        subPackages = ["protoc-gen-grpc-gateway" "protoc-gen-openapiv2"];
      };
      yarnDeps = pkgs.mkYarnPackage {
        src = ./.;
        packageJSON = ./package.json;
        yarnLock = ./yarn.lock;
        yarnNix = ./yarn.nix;
      };
    in rec {
      defaultPackage = pkgs.dockerTools.buildImage {
        name = "baseimg2";
        contents = [
          pkgs.buf
          pkgs.protoc-gen-go
          pkgs.protoc-gen-go-grpc
          grpc-gateway
          pkgs.protobuf
          yarnDeps
        ];
      };
    });
}
