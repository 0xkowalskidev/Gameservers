{
  description = "Dev environment with Tailwind and Go server";

  inputs = { nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable"; };

  outputs = { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };
    in {
      packages.${system} = {
        dev = pkgs.writeShellScriptBin "dev" ''
          ${pkgs.reflex}/bin/reflex -r '\.go|\.html$' -s -- sh -c '${pkgs.tailwindcss}/bin/tailwindcss --content "./templates/*.html" -o static/tailwind.css -m && ${pkgs.go}/bin/go run .'
        '';
        test = pkgs.writeShellScriptBin "test" ''
          ${pkgs.richgo}/bin/richgo test ./... -v
        '';
      };

      apps.${system} = {
        dev = {
          type = "app";
          program = "${self.packages.${system}.dev}/bin/dev";
        };
        test = {
          type = "app";
          program = "${self.packages.${system}.test}/bin/test";
        };
      };

      devShells.${system}.default = pkgs.mkShell {
        buildInputs = [
          pkgs.tailwindcss
          pkgs.reflex
          pkgs.go
          pkgs.richgo # Nicer go tests
          pkgs.sqlite
          pkgs.nodejs # Needed by tailwind supposedly
          self.packages.${system}.dev
          self.packages.${system}.test
        ];
      };
    };
}

