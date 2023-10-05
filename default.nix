{ pkgs ? import <nixpkgs> { } }:
pkgs.buildGoModule {
  pname = "cflivestatus";
  version = "0.1.0";

  src = ./.;

  vendorHash = null;

  CGO_ENABLED = 0;
}
