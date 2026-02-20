# GoFetch Installers

This document describes how the one-line installers work and how to publish releases.

## Release artifacts

Each release should upload platform-specific binaries with these names:

- gofetch-linux-amd64
- gofetch-linux-arm64
- gofetch-darwin-amd64
- gofetch-darwin-arm64
- gofetch-windows-amd64.exe

## Build commands

Linux:

GOOS=linux GOARCH=amd64 go build -o gofetch-linux-amd64 ./cmd/gofetch
GOOS=linux GOARCH=arm64 go build -o gofetch-linux-arm64 ./cmd/gofetch

macOS:

GOOS=darwin GOARCH=amd64 go build -o gofetch-darwin-amd64 ./cmd/gofetch
GOOS=darwin GOARCH=arm64 go build -o gofetch-darwin-arm64 ./cmd/gofetch

Windows:

GOOS=windows GOARCH=amd64 go build -o gofetch-windows-amd64.exe ./cmd/gofetch

## Installers

Linux/macOS:

curl -fsSL https://raw.githubusercontent.com/AppajiDheeraj/GoFetch/main/install.sh | sh

Windows (PowerShell):

irm https://raw.githubusercontent.com/AppajiDheeraj/GoFetch/main/install.ps1 | iex

## Version pinning

You can pin a specific release with GOFETCH_VERSION:

GOFETCH_VERSION=v1.2.3 curl -fsSL https://raw.githubusercontent.com/AppajiDheeraj/GoFetch/main/install.sh | sh

$env:GOFETCH_VERSION = "v1.2.3"
irm https://raw.githubusercontent.com/AppajiDheeraj/GoFetch/main/install.ps1 | iex

## GitHub Actions release workflow

The release workflow builds and uploads binaries whenever you push a tag like v1.2.3.

Create a tag and push it:

git tag v1.2.3
git push origin v1.2.3

The workflow is located at .github/workflows/release.yml.
