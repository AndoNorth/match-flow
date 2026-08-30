{
  description = "MatchFlow development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    git-hooks.url = "github:cachix/git-hooks.nix";
  };

  outputs = { self, nixpkgs, flake-utils, git-hooks, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };

        # Lint + security checks only - no architecture/boundary rules here.
        # Rendered into .pre-commit-config.yaml on `nix develop`.
        preCommit = git-hooks.lib.${system}.run {
          src = ./.;
          hooks = {
            # Go
            gofmt.enable = true;
            golines = {
              enable = true;
              name = "golines";
              # Auto-fix, like gofmt: golines only rewraps long lines, it
              # doesn't reorder or restructure code, so there's no
              # betteralign-style risk to unattended -w.
              entry = "${pkgs.golines}/bin/golines -w --max-len=120 --tab-len=4 --shorten-comments --reformat-tags --chain-split-dots";
              language = "system";
              pass_filenames = true;
              files = "\\.go$";
            };
            golangci-lint = {
              enable = true;
              entry = "${pkgs.golangci-lint}/bin/golangci-lint run";
              pass_filenames = false;
            };
            govulncheck = {
              enable = true;
              name = "govulncheck";
              entry = "${pkgs.govulncheck}/bin/govulncheck ./...";
              language = "system";
              pass_filenames = false;
              files = "\\.go$";
            };
            betteralign = {
              enable = true;
              name = "betteralign";
              # Check only, no -apply: auto-reordering struct fields can
              # silently break positional (unkeyed) struct literals, so
              # this flags misalignment for a human to fix, not a hook
              # that rewrites field order unattended.
              entry = "${pkgs.betteralign}/bin/betteralign ./...";
              language = "system";
              pass_filenames = false;
              files = "\\.go$";
            };
            gotest = {
              enable = true;
              name = "go test";
              entry = "${pkgs.go}/bin/go test ./...";
              language = "system";
              pass_filenames = false;
              files = "\\.go$";
            };

            # TypeScript / frontend
            #
            # Runs from inside frontend/, not repo root: biome scans for a
            # root config from its cwd, and running it from the repo root
            # over frontend/biome.json produces a "nested root
            # configuration" error instead of picking that file up as the
            # root config for its own directory.
            biome-check = {
              enable = true;
              name = "biome check";
              entry = builtins.toString (pkgs.writeShellScript "biome-check-hook" ''
                cd frontend && ${pkgs.biome}/bin/biome check --write .
              '');
              language = "system";
              pass_filenames = false;
              files = "^frontend/.*\\.(js|jsx|ts|tsx|json)$";
            };

            # Secret scanning, repo-wide
            gitleaks = {
              enable = true;
              name = "gitleaks";
              entry = "${pkgs.gitleaks}/bin/gitleaks protect --staged --no-banner";
              language = "system";
              pass_filenames = false;
            };
          };
        };
      in
      {
        checks.pre-commit = preCommit;

        devShells.default = pkgs.mkShell {
          inherit (preCommit) shellHook;

          packages = with pkgs; [
            # Go toolchain
            go
            gopls
            golangci-lint
            govulncheck
            betteralign
            golines
            air

            # Frontend toolchain
            nodejs_22
            pnpm
            biome

            # Infra / security
            #
            # Docker itself (the `docker` CLI and its `compose`/daemon)
            # is NOT provided here - it's a host-level prerequisite
            # (Docker Desktop, Colima, native dockerd) the shell can't
            # supply or manage. Nix can only put binaries on PATH, not
            # run a system service. `make check-docker` verifies it's
            # present and reachable before `dev-infra`/`dev-k8s` use it.
            redis
            gitleaks

            # Local K8s dev loop - Kind also needs the host's Docker to
            # run cluster nodes as containers, same prerequisite as above.
            kind
            tilt
            kubectl
            kubernetes-helm
          ];
        };
      });
}
