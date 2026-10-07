# dotfiles

> Un gestor de dotfiles y instalador TUI para un entorno de desarrollo de terminal
> completo. Detecta tu sistema, pregunta qué shell, terminal, multiplexor y editor
> querés, y después instala los paquetes y escribe la configuración por vos.

📄 Leer en: [English](README.md) | **Español**

## Tabla de Contenidos

- [¿Qué es esto?](#qué-es-esto)
- [Inicio rápido](#inicio-rápido)
- [Plataformas soportadas](#plataformas-soportadas)
- [Funciones del instalador](#funciones-del-instalador)
- [Entrenador de Maestría en Vim](#-entrenador-de-maestría-en-vim)
- [Documentación](#documentación)
- [Resumen de herramientas](#resumen-de-herramientas)
- [Soporte](#soporte)
- [Licencia](#licencia)
- [Contribuidores](#contribuidores)

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
| **Homebrew** | Se instala automáticamente cuando falta, en macOS y Linux, excepto en Fedora y Arch, que conservan sus gestores nativos (`dnf` y `pacman`), y en Termux |

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

El TUI te guía en la selección de tus herramientas preferidas y se encarga de toda la configuración automáticamente.

Durante la selección del multiplexor, elegí **Tmux**, **Zellij**, **Herdr** o **None**. Fish, Zsh y Nushell quedan configurados para iniciar el multiplexor elegido en shells interactivos nuevos, evitando sesiones anidadas.

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
`alacritty`, `wezterm`, `ghostty` o `none`, más `kitty` solo en macOS. `--wm` acepta
`tmux`, `zellij`, `herdr` o `none`. `--nvim` y `--font` son opcionales.

La interfaz también se puede ajustar. La animación, el mouse y el sprite son a la vez
un flag y una variable de entorno, para que un script pueda fijarlos: `--no-anim`
(`DOTFILES_ANIM=0`) detiene el movimiento, `--no-mouse` (`DOTFILES_MOUSE=0`) le devuelve
la selección y el scroll a la terminal, y `--no-sprite` (`DOTFILES_SPRITE=0`) dibuja el
gato ASCII en lugar de la criatura sombreada. La salida sincronizada no tiene flag y se
desactiva solo con `DOTFILES_SYNC=0`. La animación se apaga sola cuando la salida no es
una terminal o cuando `TERM=dumb`; el reporte del mouse y la salida sincronizada son
compuertas separadas, ambas apagadas cuando no hay terminal, y el mouse además se apaga
en Termux (`DOTFILES_MOUSE=1` anula ese valor por defecto).

Ejecutá `dotfiles --help` para ver la lista completa, incluidos `--dry-run`,
`--backup=false` y la variable de entorno `DOTFILES_VERBOSE=1`.

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
| Linux (Arch)          | x86_64                | Descarga directa            | pacman             |
| Windows               | WSL                   | Descarga directa (ver docs) | Homebrew           |
| Android               | Termux (ARM64)        | Compilación local           | pkg                |

---

## Funciones del instalador

Además de la instalación en sí, el TUI trae dos funciones a las que podés volver: una
sección de **Utilidades** para trabajos chicos y reversibles, y un **Entrenador de
Maestría en Vim** interactivo. La [Guía del instalador TUI](docs/tui-installer.md) cubre
ambas en detalle.

### Utilidades

La sección **Utilidades** reúne los trabajos chicos que no forman parte de una
instalación. Abrila desde la fila **Utilities** del menú principal, o presioná `u`.

**Cambiar el tema del escritorio.** La primera utilidad cambia el tema claro/oscuro del
sistema a través de la herramienta propia del escritorio — `gsettings` en GNOME,
`plasma-apply-colorscheme` en KDE Plasma, `defaults` en macOS — y solo ofrece un
escritorio cuya configuración puede leer exactamente como la puede escribir. Antes de
cambiar nada registra la configuración anterior en `$XDG_STATE_HOME/dotfiles/theme.json`
(o `~/.local/state/dotfiles/theme.json`), para que **Deshacer el último cambio de tema**
pueda restaurarla; una configuración que no puede restaurar con seguridad queda intacta y
se muestra el motivo. A un host sin escritorio — un servidor, Termux, una terminal pelada
— se le dice, en lugar de ofrecerle un cambio que fallaría. `--dry-run` omite la utilidad,
igual que cada paso de instalación. Consultá la
[Guía del instalador TUI](docs/tui-installer.md#utilities) para ver los archivos exactos
que toca.

**Cambiar el tema propio de los dotfiles.** Los dotfiles traen una biblioteca de paletas
— seis temas completos que cubren cada herramienta que el cambio pinta — y la paleta de
cada tema se define una sola vez en [`themes/`](themes/) en lugar de escribirse a mano en
cada configuración. Un tema es un archivo ahí, así que agregar uno agrega una fila a la
lista de temas, que abre la fila **Change the dotfiles theme** de la sección Utilidades. El
cambio aplica un tema completo (una definición con todos los roles canónicos y los miembros
`[syntax]` que lee la vista previa: **dotfiles**, **Catppuccin Mocha**, **Catppuccin
Latte**, **Kanagawa**, **Everforest** y **Rosé Pine**) a las configuraciones que este
repositorio posee para las doce herramientas, registra los bytes exactos que reemplazó en
el mismo `theme.json`, y puede restaurarlos con una fila de deshacer dedicada. bat elige su
tema por el nombre del archivo de tema personalizado, así que el cambio exporta ese nombre
y no el que lleva escrito dentro del archivo. Cada uno de los seis pinta cada herramienta
que el cambio nombra — Alacritty, Kitty, WezTerm, Ghostty, Starship, el editor de línea de
zsh, el prompt de p10k, Herdr, fish, bat, Neovim y tmux — y un guard lo mantiene así: un
tema que no pudiera pintar una la nombraría en su fila en lugar de dejarla en silencio en
la paleta anterior, y un tema al que le faltan roles, o la sintaxis que la vista previa
necesita para mostrarlo, se informa como parcial y nunca se ofrece. Donde una herramienta lee roles que la paleta no trae (los roles del prompt de
Starship, los dieciocho de fish, los ámbitos de bat, las opciones de estilo de tmux, los
grupos de resaltado de Neovim) los valores se derivan de la paleta del propio tema
mediante el mapeo fijo registrado en [`themes/README.md`](themes/README.md), así que un
color derivado es uno que el tema ya contiene. Las definiciones se leen de un checkout en
disco, nunca del binario: primero `$DOTFILES_DIR`, después el clon que hace esta
ejecución, después el directorio de trabajo y sus padres, y después `~/dotfiles` y
`~/.dotfiles`. Iniciar el instalador desde dentro del checkout muestra las filas de
inmediato; cuando ninguno de esos tiene definiciones, la sección dice que el cambio no
está disponible acá en lugar de dibujar una fila que falla. Mové el cursor sobre una fila
de tema y **todo el instalador se repinta con los colores de ese tema** — una vista previa
en vivo construida con la misma definición que escribe la aplicación, rotulada
`Preview (nothing applied)` — y al salir de la fila vuelve el aspecto por defecto; no se
escribe nada mientras el cursor se mueve. Solo reescribe archivos que llevan el marcador
de propiedad `dotfiles-managed-config:`; un archivo tuyo queda exactamente como está. Un
archivo gestionado anterior al marcador se adopta primero: cuando todo su contenido prueba
que es nuestro (es lo que el repositorio trae, o lleva un marcador de bloque generado)
solo se agrega esa línea de marcador, y cuando solo una región es nuestra — el archivo se
desvió pero todavía lleva los anclajes que el generador conoce — **solo se reescriben los
bytes entre esos anclajes y el resto del archivo queda intacto**; en cualquier caso
Deshacer restaura el original byte a byte, y un archivo que no prueba nada de eso se
rechaza con un mensaje que nombra las pruebas que intentó y el camino a seguir. La fila
**Refresh outdated theme files** trae un archivo viejo al día como un cambio nombrado y
preservado — nombra cada archivo que tocaría y dónde se preserva uno no propio (en
`~/.zshrc.d/` o `~/.config/fish/dotfiles.d/`, o junto a sí mismo como
`<path>.bak-dotfiles-<timestamp>`) antes de escribir nada — registra los bytes previos
para que Deshacer restituya cada archivo, y omite un archivo que no reconoce mientras
sigue con el resto. `--dry-run` también lo omite. Consultá
[`themes/README.md`](themes/README.md).

**Ajustar los recursos de WSL.** En un host WSL, **Adjust the WSL resources** abre la
memoria, los procesadores y el swap que puede usar la VM de WSL 2. La pantalla imprime la
RAM real y la cantidad de procesadores lógicos del host Windows, recomienda valores a
partir de ellos — la mitad de la RAM del host redondeada hacia abajo a 512 MB (nunca tanta
que Windows se quede con menos de 2 GiB), todos los CPU lógicos, y un cuarto de esa
memoria para swap — y te deja mover cada valor con **←/→** o volver todo a la recomendación
con **`r`**. Son los mismos valores y el mismo escritor que usa el paso de instalación, no
un segundo cálculo, y solo se ofrece donde hay un `.wslconfig` para editar: en cualquier
otro caso la sección dice por qué. Una escritura solo toca las claves que gestiona
[`dotfiles-wsl/.wslconfig.tmpl`](dotfiles-wsl/.wslconfig.tmpl) — el resto de tu archivo,
sus comentarios y tus propias claves se conservan exactamente como están — y el archivo
anterior se copia junto a sí mismo como `.wslconfig.bak-dotfiles-<timestamp>` primero. WSL
lee el archivo cuando arranca la VM, así que la pantalla dice que hay que ejecutar
`wsl --shutdown` en Windows para aplicar el cambio y deliberadamente no lo ejecuta;
`--dry-run` omite la escritura.

---

## 🎮 Entrenador de Maestría en Vim

¡Aprendé Vim de forma divertida! El instalador incluye un entrenador interactivo estilo RPG con:

| Módulo | Teclas cubiertas |
|--------|------------------|
| 🏃 Movimientos horizontales | `w, W, e, E, b, B, f, F, t, T, ;, ,, 0, $, ^` |
| 📐 Movimientos verticales | `j, k, gg, G, {, }, H, M, L, ctrl+d/u/f/b` |
| 🎯 Objetos de texto | `viw, vaw, vi", va", vi{, diw, daw, ci", di{, yiw, yi"` |
| 🔁 Cambiar y repetir | `d, c, dd, D, C, x, *, #, n, N, gn, cgn, dgn, .` |
| 🔄 Sustitución | `r, R, s, S, ~, gu, gU, J, :s, :%s, flags (g, c, i)` |
| 🔍 Regex y vimgrep | `/, ?, n, N, *, #, \\v, :vimgrep, :copen, :cnext` |
| 🎪 Macros | `qa, q, @a, @@, :normal, :g/pattern/` |
| 📝 Edición y deshacer | `i, a, I, A, o, O, <Esc>, u, Ctrl-r, dd, yy, p, P, >>, <<, x, D, %, marks` |
| 📋 Registros e indentación | `yy, yiw, y$, yw, yj, p, P, "a-"z, "0, dd, x, D, >>, <<` |

Cada módulo tiene entre 19 y 24 lecciones progresivas (con un mínimo de 15), modo práctica con selección inteligente de ejercicios, combates contra jefes y seguimiento de XP.

Podés iniciarlo desde el menú principal: **Vim Mastery Trainer**

---

## Documentación

| Documento | Descripción |
|-----------|-------------|
| [Guía del instalador TUI](docs/tui-installer.md) | Funciones interactivas, utilidades, temas, navegación, backup y restore |
| [Instalación manual](docs/manual-installation.md) | Configuración paso a paso para todas las plataformas |
| [Procedimientos de rollback](docs/ROLLBACK.md) | Restaurar configuraciones desde un backup y deshacer una instalación |
| [Keymaps de Neovim](docs/neovim-keymaps.md) | Referencia completa de atajos |
| [Configuración de IA](docs/ai-configuration.md) | Claude Code, OpenCode, Copilot y más |
| [Especificación del entrenador Vim](docs/vim-trainer-spec.md) | Detalles técnicos del entrenador |
| [Referencia de herramientas](docs/tools.md) | Descripciones de cada herramienta que configura el instalador |
| [Testing con Docker](docs/docker-testing.md) | Tests E2E con contenedores |
| [Contribuir](docs/contributing.md) | Setup de desarrollo, sistema de skills y releases |

### Antes de hacer push

Hay dos chequeos locales, y responden preguntas distintas.

- **`make check`** es el bucle interno. Corre `gofmt`, `go vet` y los tests de los paquetes que cambia esta rama, con la caché de tests de Go activada. Corrélo después de cada edición: es el chequeo que te dice rápido si lo que acabás de escribir sigue compilando y pasando.
- **`make preflight`** es la barrera completa. Corre acá, en el orden de CI, lo que CI reportaría después de un ciclo completo — `gofmt`, `go vet`, `go build` y el smoke test de `--help`, toda la suite de tests de Go, `shellcheck`, la auditoría de branding y un escaneo de gitleaks de los commits que agrega tu rama — e imprime el job de CI que refleja cada paso. Corrélo una vez, antes de hacer push. CI igual corre la matriz completa en el push; `make preflight` es lo que evita gastar un ciclo de CI en una falla que una corrida local habría atrapado.

Ambos viven en [scripts/preflight.sh](scripts/preflight.sh). `make preflight` se detiene en la primera falla con el comando que falló nombrado y sale con código distinto de cero, y nombra los jobs que no puede correr localmente (la matriz E2E de Docker, Termux y el toolchain de macOS). Necesita `go`, `gofmt`, `git`, `ripgrep`, `shellcheck` y `gitleaks`; `brew bundle` instala los últimos tres. `make check` solo necesita `go`, `gofmt` y `git`.

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

---

## Licencia

Licencia MIT — libre de usar, modificar y compartir.

**¡Feliz coding!** 🧰

---

## Contribuidores

¡Gracias a todos los que contribuyeron a dotfiles!

[![Contributors](https://contrib.rocks/image?repo=albersg/dotfiles)](https://github.com/albersg/dotfiles/graphs/contributors)
