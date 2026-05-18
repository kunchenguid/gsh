{
  description = "A battery-included, POSIX-compatible, generative shell";
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { nixpkgs, ... }:
  let
    version = "1.11.0"; # x-release-please-version
    forAllSystems = f:
      nixpkgs.lib.genAttrs
        [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ]
        (hostSystem: f nixpkgs.legacyPackages.${hostSystem});
  in {
    packages = forAllSystems (pkgs: {
      default = pkgs.buildGoModule {
        pname = "gsh";
        inherit version;
        src = ./.;
        # Run `nix build` with lib.fakeHash to get the correct hash
        vendorHash = "sha256-Ov9D1D7lrS2JmreSJlxwVVsWCdQK0qoun9aCYXwYvL4=";

        subPackages = [ "cmd/gsh" ];

        ldflags = [
          "-X main.BUILD_VERSION=${version}"
        ];

        nativeBuildInputs = with pkgs; [
          which
        ];

        # Skip tests that require network access or violate
        # the filesystem sandboxing.
        doCheck = false;
      };
    });
  };
}
