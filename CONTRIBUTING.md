# Contributing

## Getting Started

1. Read [UPSTREAM.md](UPSTREAM.md) and [DOWNSTREAM.md](DOWNSTREAM.md) to understand the project structure
2. See [docs/GIT-BRANCHING.md](docs/GIT-BRANCHING.md) for branch strategy
3. Set up the development environment:

```bash
git clone https://github.com/albersg/dotfiles.git
cd dotfiles
cd installer && go build ./cmd/dotfiles
```

## Making Changes

### What to Change Here

- Documentation improvements
- Branding updates
- CI/CD pipeline changes
- Profiles and configurations specific to this downstream
- Bug fixes that don't exist upstream

### What to Contribute Upstream

- Installer bugs or improvements
- New features (shell support, platform support, tools)
- Trainer improvements
- General bug fixes

Before starting work, check if the change would benefit upstream. If yes, consider contributing to [Gentleman.Dots](https://github.com/Gentleman-Programming/Gentleman.Dots) first, then sync back.

## Pull Request Checklist

- [ ] Branch from `main` with conventional name (`feat/`, `fix/`, `docs/`, etc.)
- [ ] Changes include tests where applicable
- [ ] `cd installer && go build ./cmd/dotfiles && go vet ./...` passes
- [ ] Documentation updated if behavior changed
- [ ] Branding audit passes (no residual `Gentleman` outside attribution)
- [ ] Conventional commit format

## Code Style

- Go: Follow standard Go conventions (`gofmt`, `go vet`)
- Shell: POSIX-friendly Bash, pass `shellcheck`
- YAML: 2-space indentation

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
