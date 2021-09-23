{ lib, buildGoModule, go_1_17, runCommandNoCC, git, rev ? null }:

let
  versionInfo = src: import (runCommandNoCC "eteu-amqp-deployer-version" { } ''
    v=$(${git}/bin/git -C "${src}" rev-parse HEAD || echo "0000000000000000000000000000000000000000")
    printf '{ version = "%s"; }' "$v" > $out
  '');

  # Need to keep .git around for version string
  srcCleaner = name: type: let baseName = baseNameOf (toString name); in (baseName == ".git" || lib.cleanSourceFilter name type);

  buildGo117Module = buildGoModule.override {
    go = go_1_17;
  };
in
buildGo117Module rec {
  pname = "eteu-amqp-deployer";
  version = if (rev != null) then rev else (versionInfo src).version;

  src = lib.cleanSourceWith { filter = srcCleaner; src = ./.; };

  ldflags = [
    "-X github.com/eteu-technologies/amqp-deployer/internal/core.Version=${version}"
  ];

  doCheck = true;

  vendorSha256 = "sha256-B0/trcGe8GL84tqfOiGjLUdfeODZsOaBvgjMfAAOxTc=";
  subPackages = [ "cmd/amqp-deployer" "cmd/run-deploy" ];
}
