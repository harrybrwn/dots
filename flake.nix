{
  description = "A CLI for managing your home directory dot files.";

  inputs.nixpkgs.url = "nixpkgs/nixos-22.11";

  outputs = { self, nixpkgs }:
    let
      lastModifiedDate = self.lastModifiedDate or self.lastModified or "19700101";
      # lastModifiedDate = builtins.currentTime;
      #version = builtins.substring 0 8 lastModifiedDate;
      version = "0.0.1";
      commit = if (self ? rev) then self.rev else "dirty";
      supportedSystems = [ "x86_64-linux" "x86_64-darwin" "aarch64-linux" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
      nixpkgsFor = forAllSystems (system: import nixpkgs { inherit system; });
    in
    {

      # Provide some binary packages for selected system types.
      packages = forAllSystems (system:
        let pkgs = nixpkgsFor.${system};
        in
        {
          dots = pkgs.buildGoModule {
            pname = "dots";
            subPackages = [ "./cmd/dots" ];
            inherit version;
            src = ./.;
            ldflags = [
              "-s"
              "-w"
              "-X"
              "github.com/harrybrwn/dots/cli.completions=false"
              "-X"
              "github.com/harrybrwn/dots/cli.Version=v${version}"
              "-X"
              "github.com/harrybrwn/dots/cli.Commit=${commit}"
              "-X"
              "github.com/harrybrwn/dots/cli.Hash=${commit}"
              "-X"
              "github.com/harrybrwn/dots/cli.Date=${lastModifiedDate}"
            ];
            #vendorSha256 = pkgs.lib.fakeSha256;
            vendorSha256 = "sha256-VQ70WpzZhpr+3XwtZykdCvN82Oe5QnxbdnDSOlKSZoc=";
            nativeBuildInputs = [ pkgs.git ];
            doCheck = false; # disable tests on build
            preBuild = "go generate ./...";
          };
        });

      # Add dependencies that are only needed for development
      devShells = forAllSystems (system:
        let pkgs = nixpkgsFor.${system};
        in
        {
          default = pkgs.mkShell {
            buildInputs = with pkgs; [
              go_1_20
              gopls
              gotools
              go-tools
              golangci-lint
            ];
          };
        });

      # The default package for 'nix build'. This makes sense if the
      # flake provides only one package or there is a clear "main"
      # package.
      defaultPackage = forAllSystems (system: self.packages.${system}.dots);
    };
}
