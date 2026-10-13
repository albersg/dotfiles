class Dotfiles < Formula
  desc "Interactive TUI installer for dotfiles development environment"
  homepage "https://github.com/albersg/dotfiles"
  version "0.7.6"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/albersg/dotfiles/releases/download/v#{version}/dotfiles-darwin-arm64"
      sha256 "5b3dbbe1c561b157159841262fda57bc0a7cd8d6534370a57adfaa71d7d5b9e2"
    end
    on_intel do
      url "https://github.com/albersg/dotfiles/releases/download/v#{version}/dotfiles-darwin-amd64"
      sha256 "cdf621e0768e5bc94e5b2e5f2f09127e4d3e0aa2e015b6de4807fa66ed8e5d98"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/albersg/dotfiles/releases/download/v#{version}/dotfiles-linux-arm64"
      sha256 "a3697eb2ab4b3732f5a8d7c0beff6cd473a52dfa04846de28d80e1460888ef51"
    end
    on_intel do
      url "https://github.com/albersg/dotfiles/releases/download/v#{version}/dotfiles-linux-amd64"
      sha256 "d2687ba1932369faa8b8cb1ab682c941d70e63373b1a1e53fe0777ef2daa44d9"
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
