{
  description = "kcore Crossplane provider — dev env (kind, kubectl, helm, Argo CD, buf, Go)";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" "x86_64-darwin" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      devShells = forAllSystems (pkgs:
        {
          default = pkgs.mkShell {
            name = "kcore-crossplane";
            packages = with pkgs; [
              go
              gnumake
              git
              buf
              protobuf
              kind
              kubectl
              kubernetes-helm
              argocd
              yq-go
              jq
            ];
            shellHook = ''
              export PATH="$PWD/.nix-go/bin:$PATH"
              mkdir -p .nix-go/bin
              # Crossplane CLI (crank); optional if not packaged consistently
              if ! command -v crossplane >/dev/null 2>&1; then
                echo "Tip: install Crossplane CLI once with: go install github.com/crossplane/crossplane/cmd/crank@latest && ln -sf \"\$(go env GOPATH)/bin/crank\" .nix-go/bin/crossplane"
              fi
            '';
          };
        });
    };
}
