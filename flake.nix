{
  description = "Dev environment with Tailwind and Go server";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };
    in
    {
      packages.${system} = {
        default = pkgs.buildGoModule {
          pname = "gameservers";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-ievFLcJ2jehliQCHtqVACigHmbG+zxPe3Q5WiGoR+TQ=";

          # Required for SQLite (go-sqlite3 uses CGO)
          env.CGO_ENABLED = 1;
          nativeBuildInputs = [ pkgs.pkg-config ];
          buildInputs = [ pkgs.sqlite ];

          # Skip tests (they require Docker)
          doCheck = false;

          preBuild = ''
            ${pkgs.tailwindcss}/bin/tailwindcss --content "./templates/*.html" -o static/tailwind.css -m
          '';

          meta = {
            description = "Docker-based gameserver management control panel";
            mainProgram = "gameservers";
          };
        };

        dev = pkgs.writeShellScriptBin "dev" ''
          ${pkgs.reflex}/bin/reflex -r '\.go|\.html$' -s -- sh -c '${pkgs.tailwindcss}/bin/tailwindcss --content "./templates/*.html" -o static/tailwind.css -m && ${pkgs.go}/bin/go run .'
        '';
        test = pkgs.writeShellScriptBin "test" ''
          echo "Running main application tests..."
          ${pkgs.richgo}/bin/richgo test ./database/... ./docker/... ./handlers/... ./models/... ./services/... 
        '';
        test-images = pkgs.writeShellScriptBin "test-images" ''
          echo "Running Docker image integration tests..."
          echo "Note: These tests require Docker and take several minutes to complete."
          ${pkgs.richgo}/bin/richgo test ./images/...
        '';
        test-all = pkgs.writeShellScriptBin "test-all" ''
          echo "Running all tests (application + images)..."
          echo "=== Application Tests ==="
          ${self.packages.${system}.test}/bin/test
          echo ""
          echo "=== Docker Image Tests ==="
          ${self.packages.${system}.test-images}/bin/test-images
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
        test-images = {
          type = "app";
          program = "${self.packages.${system}.test-images}/bin/test-images";
        };
        test-all = {
          type = "app";
          program = "${self.packages.${system}.test-all}/bin/test-all";
        };
      };

      devShells.${system}.default = pkgs.mkShell {
        buildInputs = [
          pkgs.tailwindcss
          pkgs.reflex
          pkgs.go
          pkgs.richgo # Nicer go tests
          pkgs.sqlite
          pkgs.nodejs # Needed by tailwind
          self.packages.${system}.dev
          self.packages.${system}.test
          self.packages.${system}.test-images
          self.packages.${system}.test-all
        ];

        shellHook = ''
          export GAMESERVER_PUBLIC_ADDRESS=$(hostname -I | awk '{print $1}')
        '';
      };
    };
}
