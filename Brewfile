# Machine toolset.
#
# This is the declarative inventory of the tools installed on this machine, on
# top of what the installer already provisions. The installer installs what the
# configuration needs to work; this file records everything else the machine
# carries, so a new machine can be brought up to the same state.
#
# Apply it with:
#
#   brew bundle --file=Brewfile
#
# Regenerate it from the machine with:
#
#   brew bundle dump --force --file=Brewfile
#
# That command records the machine as it is, not this list. It reinstates the
# tools deliberately left out, so re-curate before committing the result.
#
# brew bundle is idempotent: anything the installer already installed is left
# untouched, so entries appearing in both places are harmless and are kept here
# on purpose, to make this file a complete description of the machine.
#
# Entries that only exist on one platform are guarded with OS.linux? / OS.mac?.

# --- Shells, prompt and completions ------------------------------------------
brew "zsh"
brew "bash"
brew "nushell"
brew "starship"
brew "powerlevel10k"
brew "carapace"
brew "atuin"
brew "zoxide"
brew "direnv"
brew "fnm"
brew "zsh-autosuggestions"
brew "zsh-syntax-highlighting"
brew "zsh-completions"
brew "fzf-tab"
brew "zsh-autocomplete"
brew "thefuck"
brew "navi"

# --- Modern Unix replacements ------------------------------------------------
brew "eza"
brew "bat"
brew "fd"
brew "ripgrep"
brew "fzf"
brew "jq"
brew "git-delta"
brew "yazi"
brew "glow"
brew "fx"
brew "nb"
brew "dust"
brew "duf"
brew "btop"

# --- Git and development -----------------------------------------------------
brew "git"
brew "gh"
brew "lazygit"
brew "gitleaks"
brew "neovim"
brew "tree-sitter"
brew "go"
brew "shellcheck"
brew "grpcurl"
brew "xh"
brew "trippy"
brew "strace"
brew "coreutils"
brew "gcc"
brew "curl"

# --- Secrets -------------------------------------------------------------------
# SOPS encrypts only the values of a file and age holds the key, so an encrypted
# .env stays readable as a list of keys and can be committed. Chosen over pass and
# a GPG keyring because the shell helpers call it on every project directory and
# GPG needs an agent and a pinentry prompt, which is fragile under WSL.
brew "sops"
brew "age"

# --- Containers and Kubernetes -----------------------------------------------
brew "lazydocker"
brew "k9s"

# --- Terminals and multiplexers ----------------------------------------------
brew "herdr"

# --- Personal organiser and media --------------------------------------------
brew "calcurse"
brew "spotify_player" if OS.linux?

# --- Language toolchains -----------------------------------------------------
# The Go entries below are third-party module paths, and a module path is the
# only way to name them: they are not in homebrew/core and not on npm. That is a
# dependency, not branding; this repository attributes nothing to that project.
go "github.com/gentleman-programming/gentle-ai/v3/cmd/gentle-ai"
go "github.com/gentleman-programming/gentle-ai/cmd/gentle-ai"
npm "@google/gemini-cli"
npm "@openai/codex"
npm "azure-functions-core-tools"
npm "http-server"
npm "vercel"
npm "wscat"
uv "harlequin[mysql]"

# --- Editor extensions -------------------------------------------------------
vscode "anthropic.claude-code"
vscode "github.copilot"
vscode "github.copilot-chat"
vscode "ms-python.debugpy"
vscode "ms-python.python"
vscode "ms-python.vscode-pylance"
vscode "ms-python.vscode-python-envs"
vscode "openai.chatgpt"

# --- Windows host applications -----------------------------------------------
# Applied through winget from inside WSL, so a fresh Windows + WSL machine can be
# brought up from this same file.
winget "7zip.7zip"
winget "DBeaver.DBeaver.Community"
winget "Debian.Debian"
winget "EclipseAdoptium.Temurin.21.JDK"
winget "Git.Git"
winget "Google.Chrome"
winget "KeePassXCTeam.KeePassXC"
winget "Microsoft.365Copilot"
winget "Microsoft.Office"
winget "Microsoft.Outlook"
winget "Microsoft.PowerBI"
winget "Microsoft.PurviewInformationProtection"
winget "Microsoft.Teams"
winget "Microsoft.Teams.Classic"
winget "Microsoft.VisualStudioCode"
winget "Microsoft.WindowsApp"
winget "Microsoft.WindowsTerminal"
winget "PortSwigger.BurpSuite.Community"
winget "Python.Launcher"
winget "VideoLAN.VLC"
