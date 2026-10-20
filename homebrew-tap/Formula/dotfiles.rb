class Dotfiles < Formula
  desc "Interactive TUI installer for dotfiles development environment"
  homepage "https://github.com/albersg/dotfiles"
  version "0.12.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/albersg/dotfiles/releases/download/v#{version}/dotfiles-darwin-arm64"
      sha256 "c3b14c8d125b86a9d7a055f356bc09455f38eed3c652095540dc36056e2f1559"
    end
    on_intel do
      url "https://github.com/albersg/dotfiles/releases/download/v#{version}/dotfiles-darwin-amd64"
      sha256 "bd52cb491a0f1249a9e0880a121fa01f8ab944de368262a320c4140fc0fe336a"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/albersg/dotfiles/releases/download/v#{version}/dotfiles-linux-arm64"
      sha256 "0a0b8da368b0e5d27d2ca5feb7ab04a3031541940ba966615361b2266da208e5"
    end
    on_intel do
      url "https://github.com/albersg/dotfiles/releases/download/v#{version}/dotfiles-linux-amd64"
      sha256 "b498705771d5fd824d6849fc6d3ecabe96c4f0647cff75253d3086f90717f36b"
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
