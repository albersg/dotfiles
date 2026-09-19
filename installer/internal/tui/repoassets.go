package tui

// Repository-relative sources copied by the installer.
//
// Every path here is resolved against the checkout created by the clone step.
// Renaming a directory in the repository therefore breaks the install silently
// unless this table is updated too, which is what happened when the upstream
// Herdr and Tmux directories were renamed during the downstream rebrand while
// the installer kept the old paths. TestRepoAssetsExist fails whenever an entry
// stops existing in the repository.
const (
	repoAssetAlacritty   = "alacritty.toml"
	repoAssetWezterm     = ".wezterm.lua"
	repoAssetKitty       = "dotfiles-kitty"
	repoAssetGhostty     = "dotfiles-ghostty"
	repoAssetStarship    = "starship.toml"
	repoAssetFish        = "dotfiles-fish/fish"
	repoAssetZshEnv      = "dotfiles-zsh/.zshenv"
	repoAssetZshrc       = "dotfiles-zsh/.zshrc"
	repoAssetP10k        = "dotfiles-zsh/.p10k.zsh"
	repoAssetBatTheme    = "dotfiles-bat/themes/dotfiles.tmTheme"
	repoAssetBashEnvJSON = "bash-env-json"
	repoAssetBashEnvNu   = "bash-env.nu"
	repoAssetNushell     = "dotfiles-nushell"
	repoAssetTmuxPlugins = "dotfiles-tmux/plugins"
	repoAssetTmuxConf    = "dotfiles-tmux/tmux.conf"
	repoAssetZellij      = "dotfiles-zellij/zellij"
	repoAssetHerdrConfig = "dotfiles-herdr/config.toml"
	repoAssetNvim        = "dotfiles-nvim/nvim"
	repoAssetWSLConfig   = "dotfiles-wsl/.wslconfig"
	repoAssetWSLConf     = "dotfiles-wsl/wsl.conf"
	repoAssetGitconfig   = ".gitconfig"
	// Includes the personal identity, because .gitconfig includes it through
	// includeIf and a missing file would leave that block pointing at nothing.
	repoAssetGitconfigPersonal = "gitconfig-personal"
)

// repoAssets enumerates repoAsset* constants so the existence test covers every
// source the installer copies. Add new entries here next to the constant.
var repoAssets = []string{
	repoAssetAlacritty,
	repoAssetWezterm,
	repoAssetKitty,
	repoAssetGhostty,
	repoAssetStarship,
	repoAssetFish,
	repoAssetZshEnv,
	repoAssetZshrc,
	repoAssetP10k,
	repoAssetBatTheme,
	repoAssetBashEnvJSON,
	repoAssetBashEnvNu,
	repoAssetNushell,
	repoAssetTmuxConf,
	repoAssetZellij,
	repoAssetHerdrConfig,
	repoAssetNvim,
	repoAssetWSLConfig,
	repoAssetWSLConf,
	repoAssetGitconfig,
	repoAssetGitconfigPersonal,
}

// optionalRepoAssets are sources the installer tolerates missing. They are kept
// apart from repoAssets so the existence test only enforces what must ship.
var optionalRepoAssets = []string{
	repoAssetTmuxPlugins,
}
