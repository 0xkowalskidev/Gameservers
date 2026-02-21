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

        # Docker image build/push scripts
        build-image = pkgs.writeShellScriptBin "build-image" ''
          GAME="$1"
          REGISTRY="registry.0xkowalski.dev/gameservers"

          if [ -z "$GAME" ]; then
            echo "Usage: build-image <game>"
            echo "Available games:"
            ls -d images/*/ 2>/dev/null | xargs -n1 basename | sed 's/^/  /'
            exit 1
          fi

          if [ ! -d "images/$GAME" ]; then
            echo "Error: images/$GAME not found"
            exit 1
          fi

          # Calculate checksum of image directory
          CHECKSUM=$(find "images/$GAME" -type f | sort | xargs ${pkgs.coreutils}/bin/sha256sum | ${pkgs.coreutils}/bin/sha256sum | cut -d' ' -f1)

          echo "Building $REGISTRY/$GAME:latest (checksum: $CHECKSUM)"
          ${pkgs.docker}/bin/docker build "images/$GAME" \
            --tag "$REGISTRY/$GAME:latest" \
            --label "gameservers.checksum=$CHECKSUM"
        '';

        push-image = pkgs.writeShellScriptBin "push-image" ''
          GAME="$1"
          REGISTRY="registry.0xkowalski.dev/gameservers"

          if [ -z "$GAME" ]; then
            echo "Usage: push-image <game>"
            exit 1
          fi

          echo "Pushing $REGISTRY/$GAME:latest"
          ${pkgs.docker}/bin/docker push "$REGISTRY/$GAME:latest"
        '';

        build-images = pkgs.writeShellScriptBin "build-images" ''
          REGISTRY="registry.0xkowalski.dev/gameservers"
          GAMES=$(ls -d images/*/ 2>/dev/null | xargs -n1 basename)

          for GAME in $GAMES; do
            CHECKSUM=$(find "images/$GAME" -type f | sort | xargs ${pkgs.coreutils}/bin/sha256sum | ${pkgs.coreutils}/bin/sha256sum | cut -d' ' -f1)
            EXISTING=$(${pkgs.docker}/bin/docker inspect "$REGISTRY/$GAME:latest" 2>/dev/null | ${pkgs.jq}/bin/jq -r '.[0].Config.Labels["gameservers.checksum"] // ""')

            if [ "$CHECKSUM" = "$EXISTING" ]; then
              echo "Skipping $GAME (unchanged)"
            else
              echo "Building $GAME..."
              ${pkgs.docker}/bin/docker build "images/$GAME" \
                --tag "$REGISTRY/$GAME:latest" \
                --label "gameservers.checksum=$CHECKSUM"
            fi
          done
        '';

        push-images = pkgs.writeShellScriptBin "push-images" ''
          REGISTRY="registry.0xkowalski.dev/gameservers"
          GAMES=$(ls -d images/*/ 2>/dev/null | xargs -n1 basename)

          for GAME in $GAMES; do
            if ${pkgs.docker}/bin/docker image inspect "$REGISTRY/$GAME:latest" >/dev/null 2>&1; then
              echo "Pushing $GAME..."
              ${pkgs.docker}/bin/docker push "$REGISTRY/$GAME:latest"
            fi
          done
        '';

        build-push-images = pkgs.writeShellScriptBin "build-push-images" ''
          ${self.packages.${system}.build-images}/bin/build-images
          ${self.packages.${system}.push-images}/bin/push-images
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
        build-image = {
          type = "app";
          program = "${self.packages.${system}.build-image}/bin/build-image";
        };
        push-image = {
          type = "app";
          program = "${self.packages.${system}.push-image}/bin/push-image";
        };
        build-images = {
          type = "app";
          program = "${self.packages.${system}.build-images}/bin/build-images";
        };
        push-images = {
          type = "app";
          program = "${self.packages.${system}.push-images}/bin/push-images";
        };
        build-push-images = {
          type = "app";
          program = "${self.packages.${system}.build-push-images}/bin/build-push-images";
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
          pkgs.jq # For parsing docker inspect output
          self.packages.${system}.dev
          self.packages.${system}.test
          self.packages.${system}.test-images
          self.packages.${system}.test-all
          self.packages.${system}.build-image
          self.packages.${system}.push-image
          self.packages.${system}.build-images
          self.packages.${system}.push-images
          self.packages.${system}.build-push-images
        ];

        shellHook = ''
          export GAMESERVER_PUBLIC_ADDRESS=$(hostname -I | awk '{print $1}')
        '';
      };
    };
}
