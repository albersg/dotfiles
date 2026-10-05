class Dotfiles < Formula
  desc "Interactive TUI installer for dotfiles development environment"
  homepage "https://github.com/albersg/dotfiles"
  version "0.5.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/albersg/dotfiles/releases/download/v#{version}/dotfiles-darwin-arm64"
      sha256 "ae186293ffcc442233479a06a0c428171c620ce6466fdcdcaafea7ad8ae6e3f4"
    end
    on_intel do
      url "https://github.com/albersg/dotfiles/releases/download/v#{version}/dotfiles-darwin-amd64"
      sha256 "04430746d097e2e2ad13ee9acf895bba765827d3be3681d97c288bdd159f5946"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/albersg/dotfiles/releases/download/v#{version}/dotfiles-linux-arm64"
      sha256 "b0bf8ba1a04f5f33e59bda2d349bc3019202a0b7be7f1ac18a77246eebe8a12f"
    end
    on_intel do
      url "https://github.com/albersg/dotfiles/releases/download/v#{version}/dotfiles-linux-amd64"
      sha256 "3241633f2cf706f2b2b7617dae0cebc3aa17e96d59cbc75adbb338974a458254"
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
