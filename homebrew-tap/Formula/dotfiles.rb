class Dotfiles < Formula
  desc "Interactive TUI installer for dotfiles development environment"
  homepage "https://github.com/albersg/dotfiles"
  version "0.7.2"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/albersg/dotfiles/releases/download/v#{version}/dotfiles-darwin-arm64"
      sha256 "fff238bfc54e0821ac072ac934b4417597909978025a5ef8b48e475f05a44280"
    end
    on_intel do
      url "https://github.com/albersg/dotfiles/releases/download/v#{version}/dotfiles-darwin-amd64"
      sha256 "0d4e82d8f8f09548385561a6768fa57fd5387814d774c0a4975d975952658417"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/albersg/dotfiles/releases/download/v#{version}/dotfiles-linux-arm64"
      sha256 "3d355b773b44b5f7503ca7e9dca5e4b872964a3f26e76339031b6b27ee5c1d56"
    end
    on_intel do
      url "https://github.com/albersg/dotfiles/releases/download/v#{version}/dotfiles-linux-amd64"
      sha256 "50743d756b7354a49df265f019ffcd0242ec8b12900014b4a49f225e4b8d69fd"
    end
  end

  def install
    if OS.mac? && Hardware::CPU.arm?
      bin.install "dotfiles-darwin-arm64" => "dotfiles"
    elsif OS.mac? && Hardware::CPU.intel?
      bin.install "dotfiles-darwin-amd64" => "dotfiles"
    elsif OS.linux? && Hardware::CPU.arm?
      bin.install "dotfiles-linux-arm64" => "dotfiles"
    elsif OS.linux? && Hardware::CPU.intel?
      bin.install "dotfiles-linux-amd64" => "dotfiles"
    end
  end

  test do
    system "#{bin}/dotfiles", "--help"
  end
end
