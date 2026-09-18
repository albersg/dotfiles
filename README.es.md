# dotfiles

📄 Leer en: [English](README.md) | **Español**

## Tabla de Contenidos

- [¿Qué es esto?](#qué-es-esto)
- [Inicio rápido](#inicio-rápido)
- [Plataformas soportadas](#plataformas-soportadas)
- [Entrenador de Maestría en Vim](#-entrenador-de-maestría-en-vim)
- [Documentación](#documentación)
- [Resumen de herramientas](#resumen-de-herramientas)
- [Bleeding Edge](#bleeding-edge)
- [Soporte](#soporte)

---

## Vista previa

### Instalador TUI

<img width="1424" height="1536" alt="Instalador TUI" src="https://github.com/user-attachments/assets/1db56d3b-a8c0-4885-82aa-c5ec04af4ac0" />

### Demostración

<img width="3840" height="2160" alt="Showcase del entorno de desarrollo" src="https://github.com/user-attachments/assets/fff14c05-9676-4e04-b05e-dab5e3cf300a" />

---

## ¿Qué es esto?

Una configuración completa de entorno de desarrollo que incluye:

- **Neovim** con LSP, autocompletado e integración con IA
- **Shells**: Fish, Zsh, Nushell
- **Multiplexores de terminal**: Tmux, Zellij, Herdr
- **Emuladores de terminal**: Alacritty, WezTerm, Kitty, Ghostty
- **Herramientas CLI de IA**: Instaladores de Claude Code y OpenCode CLI

---

## Inicio rápido

### Opción 1: Homebrew (Recomendado)

```bash
brew install albersg/tap/dotfiles
dotfiles
```

### Opción 2: Descarga directa

```bash
# macOS Apple Silicon
curl -fsSL https://github.com/albersg/dotfiles/releases/latest/download/dotfiles-darwin-arm64 -o dotfiles

# macOS Intel
curl -fsSL https://github.com/albersg/dotfiles/releases/latest/download/dotfiles-darwin-amd64 -o dotfiles

# Linux x86_64
curl -fsSL https://github.com/albersg/dotfiles/releases/latest/download/dotfiles-linux-amd64 -o dotfiles

# Linux ARM64 (Raspberry Pi, etc.)
curl -fsSL https://github.com/albersg/dotfiles/releases/latest/download/dotfiles-linux-arm64 -o dotfiles

# Luego ejecutar
chmod +x dotfiles
./dotfiles
```

### Opción 3: Termux (Android)

Termux requiere compilar localmente. Consultá la [Guía de instalación en Termux](docs/manual-installation.md#termux) para las instrucciones completas.

---

El TUI te deja elegir **Tmux**, **Zellij**, **Herdr** o **None** como multiplexor. Fish, Zsh y Nushell quedan configurados para iniciar el multiplexor elegido en shells interactivos nuevos, evitando sesiones anidadas.

> **Usuarios de Tmux:** Después de la instalación, abrí tmux y presioná `prefix + I` (I mayúscula) para instalar los plugins con TPM. Esto asegura que el tema y los plugins carguen correctamente.

> **Usuarios de Windows:** Primero tenés que configurar WSL. Consultá la [Guía de instalación manual](docs/manual-installation.md#windows-wsl).

---

## Plataformas soportadas

| Plataforma            | Arquitectura          | Método de instalación       | Gestor de paquetes |
| --------------------- | --------------------- | --------------------------- | ------------------ |
| macOS                 | Apple Silicon (ARM64) | Homebrew, descarga directa  | Homebrew           |
| macOS                 | Intel (x86_64)        | Homebrew, descarga directa  | Homebrew           |
| Linux (Ubuntu/Debian) | x86_64, ARM64         | Homebrew, descarga directa  | Homebrew           |
| Linux (Fedora/RHEL)   | x86_64, ARM64         | Descarga directa            | dnf                |
| Linux (Arch)          | x86_64                | Homebrew, descarga directa  | Homebrew           |
| Windows               | WSL                   | Descarga directa (ver docs) | Homebrew           |
| Android               | Termux (ARM64)        | Compilación local           | pkg                |

---

## 🎮 Entrenador de Maestría en Vim

¡Aprendé Vim de forma divertida! El instalador incluye un entrenador interactivo estilo RPG con:

| Módulo                   | Teclas cubiertas                          |
| ------------------------ | ----------------------------------------- |
| 🔤 Movimiento horizontal | `w`, `e`, `b`, `f`, `t`, `0`, `$`, `^`   |
| ↕️ Movimiento vertical   | `j`, `k`, `G`, `gg`, `{`, `}`            |
| 📦 Objetos de texto      | `iw`, `aw`, `i"`, `a(`, `it`, `at`       |
| ✂️ Cambiar y repetir     | `d`, `c`, `dd`, `cc`, `D`, `C`, `x`      |
| 🔄 Sustitución           | `r`, `R`, `s`, `S`, `~`, `gu`, `gU`, `J` |
| 🎬 Macros y registros    | `qa`, `@a`, `@@`, `"ay`, `"+p`           |
| 🔍 Regex / Búsqueda      | `/`, `?`, `n`, `N`, `*`, `#`, `\v`       |

Cada módulo incluye 15 lecciones progresivas, modo práctica con selección inteligente de ejercicios, jefes finales y seguimiento de XP.

Podés iniciarlo desde el menú principal: **Vim Mastery Trainer**

---

## Documentación

| Documento                                                     | Descripción                                                                    |
| ------------------------------------------------------------- | ------------------------------------------------------------------------------ |
| [Guía del instalador TUI](docs/tui-installer.md)              | Funciones interactivas, navegación, backup y restore                           |
| [Instalación manual](docs/manual-installation.md)             | Configuración paso a paso para todas las plataformas                           |
| [Keymaps de Neovim](docs/neovim-keymaps.md)                   | Referencia completa de atajos                                                  |
| [Configuración de IA](docs/ai-configuration.md)               | Claude Code, OpenCode, Copilot y más                                           |
| [Especificación del entrenador Vim](docs/vim-trainer-spec.md) | Detalles técnicos del entrenador                                               |
| [Testing con Docker](docs/docker-testing.md)                  | Tests E2E con contenedores                                                     |
| [Contribuir](docs/contributing.md)                            | Setup de desarrollo, sistema de skills y releases                              |

---

## Resumen de herramientas

- **Emuladores de terminal**: Ghostty, Kitty, WezTerm, Alacritty
- **Shells**: Nushell, Fish, Zsh (+ Powerlevel10k)
- **Multiplexores**: Tmux, Zellij, Herdr
- **Editor**: Neovim (LazyVim con LSP, completado e IA)
- **Prompt**: Starship

> Consultá la [Referencia de herramientas](docs/tools.md) para descripciones detalladas de cada herramienta.

---

## Bleeding Edge

¿Querés las últimas funcionalidades experimentales de mi workflow diario (solo macOS)?

Mirá la rama [`nix-migration`](https://https://github.com/albersg/dotfiles/tree/nix-migration).

Contiene configuraciones de vanguardia que luego pasan a `main` cuando se estabilizan.

---

## Soporte

- **Issues**: [GitHub Issues](https://github.com/albersg/dotfiles/issues)

> Esta es una distribución downstream de [dotfiles](https://github.com/albersg/dotfiles). Consultá [UPSTREAM.md](UPSTREAM.md) para enlaces de la comunidad upstream y atribución.

---

## Licencia

Licencia MIT — libre de usar, modificar y compartir.

**¡Feliz coding!** 🎩

---

## Contribuidores

¡Gracias a todos los que contribuyeron a dotfiles!

[![Contributors](https://contrib.rocks/image?repo=albersg/dotfiles)](https://github.com/albersg/dotfiles/graphs/contributors)
