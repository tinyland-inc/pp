{
  description = "prompt-pulse: Shell monitoring dashboard with Claude API, cloud billing, and infrastructure tracking";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.11";
    flake-parts.url = "github:hercules-ci/flake-parts";
  };

  outputs = inputs@{ flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" "x86_64-darwin" ];

      perSystem = { pkgs, self', system, ... }: {
        packages.default = ((pkgs.buildGoModule.override { go = pkgs.go_1_24; }) {
          pname = "prompt-pulse";
          version = "0.2.0";
          src = ./.;
          vendorHash = null;

          ldflags = [
            "-s" "-w"
            "-X main.version=0.2.0"
          ] ++ pkgs.lib.optionals (inputs.self ? rev) [
            "-X main.commit=${inputs.self.rev}"
          ];

          meta = with pkgs.lib; {
            description = "Shell monitoring dashboard with Claude API, cloud billing, and infrastructure tracking";
            homepage = "https://gitlab.com/tinyland/projects/prompt-pulse";
            license = licenses.mit;
            platforms = platforms.unix;
            mainProgram = "prompt-pulse";
          };
        }).overrideAttrs (old: {
          # Disable tests - visual regression tests require golden files not in Nix sandbox
          doCheck = false;
        });

        devShells.default = pkgs.mkShell {
          inputsFrom = [ self'.packages.default ];
          packages = with pkgs; [ go_1_24 gopls golangci-lint ];
        };
      };

      flake = {
        overlays.default = final: prev: {
          prompt-pulse = inputs.self.packages.${final.system}.default;
        };
      };
    };
}
