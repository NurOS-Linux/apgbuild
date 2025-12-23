{
  description = "Tulpar non-required C++ component";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };
    in
    {
      packages.${system}.default = pkgs.stdenv.mkDerivation {
        pname = "apgbuild";
        version = "0.2.1";

        src = ./.;

        nativeBuildInputs = [
          pkgs.meson
          pkgs.ninja
          pkgs.pkg-config
        ];

        buildInputs = [
          pkgs.gcc
          pkgs.openssl
          pkgs.libarchive
          pkgs.nlohmann_json
        ];

        mesonFlags = [
          "--buildtype=release"
        ];

        installPhase = ''
          mkdir -p $out/bin
          cp -r apgbuild $out/bin
        '';
      };
    };
}
