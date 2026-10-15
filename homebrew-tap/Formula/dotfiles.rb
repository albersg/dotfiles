class Dotfiles < Formula
  desc "Interactive TUI installer for dotfiles development environment"
  homepage "https://github.com/albersg/dotfiles"
  version "0.7.11"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/albersg/dotfiles/releases/download/v#{version}/dotfiles-darwin-arm64"
      sha256 "32ecb6d897ec3c1bc4678b5b56bcbe03055941b6480c06a90e6e98a2f18d4648"
    end
    on_intel do
      url "https://github.com/albersg/dotfiles/releases/download/v#{version}/dotfiles-darwin-amd64"
      sha256 "3c160b64624180dcf53b0e49fadf12cd5316c7a0f8f53fed7195db3f6a71701d"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/albersg/dotfiles/releases/download/v#{version}/dotfiles-linux-arm64"
      sha256 "0e5f284cc0c3645dc8b2cb1db073346051e9e558b4f8cad887c3cc9f061f3a22"
    end
    on_intel do
      url "https://github.com/albersg/dotfiles/releases/download/v#{version}/dotfiles-linux-amd64"
      sha256 "47816b11d54d0d5428e9e6864a006aa63a6661ff350164e8d70e6554761badf9"
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
