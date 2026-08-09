{
  description = "GitLab provider plugin for Elephant";

  inputs = {
    # Track the branch, not a tag: elephant's Go plugin ABI must match its
    # toolchain exactly, and nixconfig overrides this input to follow the
    # branch anyway. Pinning here only made CI test a build we never run.
    elephant.url = "github:abenz1267/elephant";
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
              cp -r ${./src} $sourceRoot/internal/providers/gitlab
              chmod -R u+w $sourceRoot/internal/providers/gitlab
            '';

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
