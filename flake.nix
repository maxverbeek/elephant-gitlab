{
  description = "GitLab provider plugin for Elephant";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    elephant.url = "github:abenz1267/elephant";
    elephant.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs =
    { self, nixpkgs, elephant, ... }:
    let
      systems = [ "x86_64-linux" ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
      nixpkgsFor = forAllSystems (system: import nixpkgs { inherit system; });
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = nixpkgsFor.${system};
          elephantProviders = elephant.packages.${system}.elephant-providers;

          # Inject our plugin source into the elephant source tree, and its
          # extra dependency (upstream's go.mod does not have godbus)
          inject = ''
            cp -r ${./src} $sourceRoot/internal/providers/gitlab
            chmod -R u+w $sourceRoot/internal/providers/gitlab
            chmod u+w $sourceRoot/go.mod $sourceRoot/go.sum
            grep godbus ${./go.sum} >> $sourceRoot/go.sum
            (cd $sourceRoot && go mod edit -require=github.com/godbus/dbus/v5@v5.2.2)
          '';
        in
        {
          default = elephantProviders.overrideAttrs (old: {
            pname = "elephant-gitlab";

            postUnpack = (old.postUnpack or "") + inject;

            vendorHash = "sha256-t8Lsa6/K2AJCZEmi6ngQ4DAIN5RcGBtgLGW/Z/9e0kw=";

            passthru = old.passthru // {
              overrideModAttrs = pkgs.lib.composeExtensions old.passthru.overrideModAttrs (
                _: prev: { postUnpack = (prev.postUnpack or "") + inject; }
              );
            };

            buildPhase = ''
              runHook preBuild
              echo "Building provider: gitlab"
              go build -buildmode=plugin -trimpath -o gitlab.so ./internal/providers/gitlab
              runHook postBuild
            '';

            # Upstream's checkPhase relies on go-hooks that aren't in scope here
            # (it fails with "getGoDirs: command not found"), so run our tests directly.
            checkPhase = ''
              runHook preCheck
              go test ./internal/providers/gitlab
              runHook postCheck
            '';

            installPhase = ''
              runHook preInstall
              mkdir -p $out/lib/elephant/providers
              cp gitlab.so $out/lib/elephant/providers/
              runHook postInstall
            '';
          });
        }
      );

      # Package build has doCheck = true, so this runs the Go tests too.
      checks = forAllSystems (system: { default = self.packages.${system}.default; });

      devShells = forAllSystems (
        system:
        let
          pkgs = nixpkgsFor.${system};
        in
        {
          default = pkgs.mkShell {
            name = "devshell";
            packages = with pkgs; [
              gcc
              pkg-config
            ];
          };
        }
      );
    };
}
