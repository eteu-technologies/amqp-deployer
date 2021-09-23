{
  description = "amqp-deployer";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  inputs.flake-utils.url = "github:numtide/flake-utils";

  outputs = { self, nixpkgs, flake-utils }:
    let
      supportedSystems = [
        "aarch64-linux"
        "aarch64-darwin"
        "x86_64-linux"
        "x86_64-darwin"
      ];
    in
    flake-utils.lib.eachSystem supportedSystems (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      rec {
        packages.eteu-amqp-deployer = pkgs.callPackage ./default.nix { rev = if (self ? rev) then self.rev else "dirty"; };

        defaultPackage = packages.eteu-amqp-deployer;

        devShell = pkgs.mkShell {
          nativeBuildInputs = [
            pkgs.go_1_17
            pkgs.golangci-lint
            pkgs.gopls
          ];
        };
      });
}
