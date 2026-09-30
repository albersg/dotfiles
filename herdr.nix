{ lib, ... }:

{
  # Herdr — agent multiplexer that lives in your terminal (https://herdr.dev)
  # Binary installed via Homebrew (homebrew-core). Runtime config is copied as a
  # regular writable file instead of being linked into the Nix store.

  # Auto-install herdr on home-manager activation if it is missing.
  # Guarded so a missing/failed brew never breaks the activation (same approach as engram.nix).
  # Home Manager activation runs with a minimal PATH that excludes Homebrew, so
  # Homebrew is prepended before the probes: otherwise both `command -v herdr`
  # and `command -v brew` fail on a machine where both are installed, the
  # warning below is false, and the install branch is unreachable.
  home.activation.installHerdr = lib.hm.dag.entryAfter [ "linkGeneration" ] ''
    echo "🔧 Setting up Herdr..."

    if [ "$(uname)" = "Darwin" ]; then
      export PATH="/opt/homebrew/bin:/usr/local/bin:$PATH"
    else
      export PATH="/home/linuxbrew/.linuxbrew/bin:$PATH"
    fi

    if command -v herdr &>/dev/null; then
      echo "✅ Herdr already installed"
    elif command -v brew &>/dev/null; then
      echo "🚀 Installing Herdr via Homebrew..."
      brew install herdr || echo "❌ Herdr installation failed (run 'brew install herdr' manually)"
    else
      echo "⚠️  Homebrew not found — install Herdr manually: brew install herdr"
    fi

    echo "📝 Copying Herdr config..."
    mkdir -p "$HOME/.config/herdr"
    # Keep the repo file as the source of truth, but install it as a regular
    # writable file so Herdr does not read through a Nix store symlink.
    cp "${./herdr/config.toml}" "$HOME/.config/herdr/config.toml"
    chmod u+w "$HOME/.config/herdr/config.toml"
  '';
}
