# dotfiles

📄 Leer en: [English](README.md) | **Español**

## Tabla de Contenidos

- [¿Qué es esto?](#qué-es-esto)
- [Inicio rápido](#inicio-rápido)
- [Plataformas soportadas](#plataformas-soportadas)
- [Entrenador de Maestría en Vim](#-entrenador-de-maestría-en-vim)
- [Documentación](#documentación)
- [Resumen de herramientas](#resumen-de-herramientas)
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

### Requisitos

| Requisito | Detalles |
|-------------|---------|
| **Sistema operativo** | macOS 10.15+; Linux (Ubuntu 20.04+, Debian, Fedora/RHEL, Arch); WSL2; o Termux |
| **Git y curl** | El instalador clona este repositorio mientras se ejecuta |
| **Internet** | Para clonar el repositorio y descargar paquetes |
| **Homebrew** | Se instala automáticamente cuando falta, en macOS y Linux, excepto en Fedora y Termux |

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

El archivo descargado no se coloca en el `PATH`, por lo que se ejecuta como `./dotfiles`
desde el directorio donde se descargó, y cada versión publicada incluye además un archivo
`SHA256SUMS` para verificar la descarga.

### Opción 3: Termux (Android)

Termux requiere compilar localmente: Android no tiene Homebrew y no hay un binario publicado para él. El instalador reconoce Termux, instala sus paquetes con `pkg` en lugar de un gestor de paquetes que no existe allí, y escribe la Nerd Font en `~/.termux/font.ttf` en vez de un directorio de fuentes de escritorio. Hay que clonar el repositorio, compilar el instalador con Go y ejecutarlo desde el checkout. Termux es la plataforma menos ejercitada de las tres y todavía no tiene una guía paso a paso.

---

El TUI te deja elegir **Tmux**, **Zellij**, **Herdr** o **None** como multiplexor. Fish, Zsh y Nushell quedan configurados para iniciar el multiplexor elegido en shells interactivos nuevos, evitando sesiones anidadas.

> **Usuarios de Tmux:** Después de la instalación, abrí tmux y presioná `prefix + I` (I mayúscula) para instalar los plugins con TPM. Esto asegura que el tema y los plugins carguen correctamente.

> **Usuarios de Windows:** Primero tenés que configurar WSL. Consultá la [Guía de instalación manual](docs/manual-installation.md#windows-wsl).

### Qué hace la instalación

El instalador es un TUI interactivo. Detecta el sistema y las herramientas que ya están
presentes, pregunta qué shell, terminal, multiplexor y editor se desean, y luego instala
los paquetes y escribe la configuración.

Antes de reemplazar cualquier cosa, copia la configuración que ya está en su lugar a
`~/.dotfiles-backup-<timestamp>/`. Esas copias de seguridad son copias simples de los
archivos previos, así que devolverlas es copiar. Consultar [Procedimientos de rollback](docs/ROLLBACK.md).

### Probarlo sin cambiar nada

```bash
dotfiles --dry-run    # informa qué se instalaría, sin cambiar nada
dotfiles -t           # se ejecuta contra un HOME desechable, sin tocar el real
```

### Instalación no interactiva

```bash
dotfiles --non-interactive --shell=zsh --wm=herdr --nvim
```

`--shell` es obligatorio y acepta `fish`, `zsh` o `nushell`. `--terminal` acepta
`alacritty`, `wezterm`, `kitty`, `ghostty` o `none`. `--wm` acepta `tmux`, `zellij`,
`herdr` o `none`. `--nvim` y `--font` son opcionales. `dotfiles --help` lista el resto,
incluidos `--backup=false` y la variable de entorno `DOTFILES_VERBOSE=1`.

### Después de instalar

Hay que abrir un shell nuevo. El instalador escribe la configuración del shell elegido,
pero no puede recargar el shell desde el que se ejecutó.

Para actualizar más adelante, hay que instalar el instalador más reciente y ejecutarlo de
nuevo: `brew upgrade dotfiles` en la vía de Homebrew, o descargar el binario actual en
caso contrario. Una ejecución posterior vuelve a clonar este repositorio, así que toma
las configuraciones actuales.

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
| [Procedimientos de rollback](docs/ROLLBACK.md)                | Restaurar configuraciones desde un backup y deshacer una instalación           |
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

## Soporte

- **Issues**: [GitHub Issues](https://github.com/albersg/dotfiles/issues)

> Esta es una distribución downstream de [dotfiles](https://github.com/albersg/dotfiles). Consultá [UPSTREAM.md](UPSTREAM.md) para enlaces de la comunidad upstream y atribución.

---

## Licencia

Licencia MIT — libre de usar, modificar y compartir.

**¡Feliz coding!** 🧰

---

## Contribuidores

¡Gracias a todos los que contribuyeron a dotfiles!

[![Contributors](https://contrib.rocks/image?repo=albersg/dotfiles)](https://github.com/albersg/dotfiles/graphs/contributors)
