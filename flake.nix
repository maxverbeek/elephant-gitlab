{
  description = "GitLab provider plugin for Elephant";

  inputs = {
    elephant.url = "github:abenz1267/elephant/v2.19.3";
    nixpkgs.follows = "elephant/nixpkgs";
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
        in
        {
          default = elephantProviders.overrideAttrs (old: {
            pname = "elephant-gitlab";

            # Inject our plugin source into the elephant source tree
            postUnpack = (old.postUnpack or "") + ''
              mkdir -p $sourceRoot/internal/providers/gitlab
              cp ${./setup.go} $sourceRoot/internal/providers/gitlab/setup.go
              cp ${./config.go} $sourceRoot/internal/providers/gitlab/config.go
              cp ${./db.go} $sourceRoot/internal/providers/gitlab/db.go
              cp ${./gitlab.go} $sourceRoot/internal/providers/gitlab/gitlab.go
              cp ${./query.go} $sourceRoot/internal/providers/gitlab/query.go
              cp ${./activate.go} $sourceRoot/internal/providers/gitlab/activate.go
              cp ${./README.md} $sourceRoot/internal/providers/gitlab/README.md
            '';

            buildPhase = ''
              runHook preBuild
              echo "Building provider: gitlab"
              go build -buildmode=plugin -trimpath -o gitlab.so ./internal/providers/gitlab
              runHook postBuild
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
