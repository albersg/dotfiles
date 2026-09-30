# Triage upstream — Gentleman.Dots → dotfiles (downstream)

- **Rama / worktree**: `chore/upstream-triage` en `/home/alberto/work/dotfiles-wt-upstream`
- **Base downstream**: `main` @ `28eb183`; base upstream heredada: `v2.12.2` (`UPSTREAM.md`)
- **Upstream triado**: `Gentleman-Programming/Gentleman.Dots` (PRs e issues **abiertos** a fecha de este triaje)
- **Método**: sólo lectura. `gh pr diff` / `gh pr view --json files,body` / `gh issue view`. Cada veredicto se comprobó contra el árbol local con `grep`/lectura de fichero; las pruebas se citan como `fichero:línea`. **No se ha modificado nada más que este documento. No hay commits.**

> Regla de lectura aplicada: un arreglo del upstream **sólo** nos aplica si el defecto sigue existiendo en nuestro código. Nuestro instalador divergió mucho (Go propio, TUI, trainer, paneles): la mayoría de los arreglos de `installer/` del upstream **ya están resueltos aquí por otra vía**, y varios de sus tests de regresión existen con otro nombre.

## Cómo se trió `#197` (≈2 970 líneas, 32 ficheros)

No se leyó entero. Se leyó el **cuerpo del PR** (que declara qué issues cierra y qué NO toca), la lista de ficheros del diff (`openspec/changes/archive/2026-08-29-fix-arch-installer/{proposal,design,tasks,specs}`), y se comprobó **cada defecto declarado** contra nuestro árbol. Resultado: los tres defectos que cierra (`#183`, `#193`, `#195`) y el dry-run ya están resueltos aquí, así que no hace falta leer el resto del diff.

---

## Resumen ejecutivo

- **11 filas `APPLIES`**, que corresponden a **8 frentes de trabajo distintos** (hay duplicados: `#210`⊂`#211`, `#180`+`#173`⊂`#189`, `#165`⊂`#170`).
- **Pérdidas de datos heredadas: 2 clases, ambas en el instalador y ambas versiones suaves de lo que el upstream ya arregló** (ver §5). No son borrados silenciosos irreversibles sin red: nuestro instalador sí hace backup previo, pero **sobrescribe y no reincorpora** personalizaciones.
- **Lo que NO hay que traer**: casi todo el `installer/` del upstream (`#197`, `#202`, `#185`, `#188`, `#192`), el tap (`#175`), `opencode.nix` (`#208`), la integración zsh-autocomplete (`#199`, `#205`) y el selector de leader key (`#124`). Ver §4.
- **Sin determinar**: `#181` (ANSI crudos en WezTerm+WSL2+Zellij+Nushell) y la validez del COPR de Ghostty en Fedora 44 (`#184`). Ver §6.

---

## 1. PRs abiertos

| # | Qué cambia | Ficheros en upstream | ¿Existe en nuestro árbol? | ¿Tenemos el mismo defecto? (prueba) | Veredicto | Esfuerzo | Riesgo |
|---|---|---|---|---|---|---|---|
| **#212** | herdr: `prompt_new_tab_name = false` en `[ui]` para no pedir nombre de pestaña | `herdr/config.toml` | Sí → `dotfiles-herdr/config.toml` | **Sí**: no existe la clave en `dotfiles-herdr/config.toml:24-32` | `APPLIES` | XS | Bajo |
| **#211** | zsh: mover el lanzador del WM (`start_if_needed` + su llamada) **por encima** del instant prompt de p10k; añade test de orden | `GentlemanZsh/.zshrc`, `installer/internal/system/zsh_template_test.go` | Sí → `dotfiles-zsh/.zshrc` (test no existe) | **Sí**: instant prompt en `dotfiles-zsh/.zshrc:4`; `start_if_needed` definido en `:403-406` y **llamado en `:832`** | `APPLIES` | S | Medio (nuestro launcher usa `typeset -a WM_CMD` + `herdr` por defecto, no `WM_CMD="tmux"`; el parche encaja conceptualmente, no literal) |
| **#204** | herdr: activar avisos sonoros por estado de agente y documentar el reproductor | `herdr/config.toml` | Sí | **No**: ya tenemos el bloque completo `[ui.sound] enabled = true` con el mismo comentario en `dotfiles-herdr/config.toml:34-45` | `ALREADY FIXED` | — | — |
| **#203** | ghostty: claves deprecadas en 1.3 (`background-blur-radius`→`background-blur`, `gtk-tabs-location=hidden`→`window-show-tab-bar=never`) | `GentlemanGhostty/config` | Sí → `dotfiles-ghostty/config` | **Sí**: `dotfiles-ghostty/config:20` y `:29` tienen las claves viejas | `APPLIES` | XS | Bajo-medio (si el usuario tiene Ghostty <1.3 las claves nuevas no existen; asumimos ≥1.3) |
| **#202** | Arch: quitar nombres sólo-AUR (`carapace`, `zsh-theme-powerlevel10k`) de las listas de `pacman` + test que lee los literales del fuente | `installer/internal/tui/installer.go`, `arch_packages_source_test.go`, `platform_packages_test.go` | Sí (Go propio, fichero distinto) | **No**: ya hay filtro dedicado `archUnavailable = {carapace, zsh-theme-powerlevel10k}` y `archPackages()` en `installer/internal/tui/installer.go:1167-1185`, usado en `:1565`, `:1593`, `:1617`; los tests están en `installer/internal/tui/arch_packages_test.go:21-22,128-137` | `ALREADY FIXED` | — | — |
| **#201** | Linux: soporte gestionado de fuente emoji + navegación Unicode/UTF-8 en el trainer (`simulator.go`) | `AGENTS.md`, spec `.md`, `installer/e2e/test_emoji_navigation.sh`, `installer/internal/tui/trainer/simulator.go`, `simulator_unicode_test.go` | Sí (trainer propio, muy divergido) | **Parcial**: el defecto técnico existe (`isWordChar(ch byte, ...)` en `installer/internal/tui/trainer/simulator.go:908`); la parte de "fuente emoji" **choca con nuestra decisión**: `installer/internal/tui/view.go:969-970,2233-2254` evita emoji a propósito porque un terminal sin fuente emoji dibuja cajas | `PARTIAL` | M | Medio (el trainer es nuestros; portar `simulator.go` a mano puede romper nuestra máquina de buffer) |
| **#197** | Arch: hacer que la instalación funcione y **dejar de destruir datos**. Cierra `#183` (AUR), `#193` (`rm -rf` relativo al CWD), `#195` (socket en el backup); arregla `--dry-run`; añade `repostate.go` (guard de borrado), `dryrun.go`, tests | `installer/internal/tui/installer.go`, `installer/internal/system/{exec.go,repostate.go,dryrun.go,report.go}`, `openspec/…` (32 ficheros) | Sí (nombres y estructura distintos) | **No** en ninguno de sus cuatro frentes, **triado por partes**: (a) AUR → `archUnavailable`/`archPackages` (`installer.go:1167-1185`); (b) `rm -rf` relativo → el clon vive en `os.MkdirTemp` y el comentario documenta el defecto antiguo (`installer.go:151-197`; no queda ningún `rm -rf <ruta relativa>` de repo); (c) socket → `ErrNotRegularFile` + `CopyDirReport` con `skipped` (`system/exec.go:290-336,419-422,586-645`); (d) dry-run → `executeStep` corta todos los steps (`installer.go:50-60,90-102`) | `ALREADY FIXED` (por partes) | — | — |
| **#191** | Fish: `git.io/fisher` muerto → URL de GitHub + `and`; basura fuera del `PATH` (`$HOME/.config`, `/usr/local/lib/*`) y `cargo/bin` duplicado; `MANPAGER` con bat; quitar `clear`. Tmux: `M-l` lazygit + `history-limit`. Installer: `tmux new-session -A -s main` → sesión única por ventana | `GentlemanFish/fish/config.fish`, `GentlemanTmux/tmux.conf`, `installer/internal/system/exec.go`, `patch_test.go` | Sí → `dotfiles-fish/fish/config.fish`, `dotfiles-tmux/tmux.conf`, `installer/internal/system/exec.go` | **Sí, en 3 de los 4 frentes**: URL muerta en `dotfiles-fish/fish/config.fish:5` (única aparición de `git.io` en el repo); `PATH` con `$HOME/.config` y `/usr/local/lib/*` en `:29` y `:33`; `set -x PATH $HOME/.cargo/bin $PATH` duplicado en `:52`; `clear` final en `:122`; sin `MANPAGER`; `tmux.conf` sin `M-l` ni `history-limit`; `tmux new-session -A -s main` en `installer/internal/system/exec.go:1033` (y en la plantilla `config.fish:43`) | `PARTIAL` → **APPLIES** en fish y en el bloque tmux de `exec.go`; las dos líneas de `tmux.conf` son features | S | Bajo (ediciones literales, ficheros casi idénticos al upstream) |
| **#189** | nvim: retirar `gemini-cli.nvim` (descontinuado) y migrar a `antigravity-cli.nvim`; registrar el nuevo plugin en `disabled.lua`; docs; regenerar `lazy-lock.json`. Cierra `#180`, solapa `#173` | `GentlemanNvim/nvim/lua/plugins/{gemini,antigravity,disabled}.lua`, `lazy-lock.json`, `docs/ai-configuration.md` | Sí → `dotfiles-nvim/nvim/lua/plugins/…`, `dotfiles-nvim/nvim/lazy-lock.json`, `docs/ai-configuration.md` | **Sí**: `gemini.lua:1-6` sigue haciendo `require("gemini").setup()` en `config` (doble `setup()` con el `plugin/gemini.vim` del plugin — `#173`), `disabled.lua:27-29` lo desactiva, `lazy-lock.json:13` todavía tiene `gemini-cli.nvim` | `APPLIES` | M | Medio (regenerar `lazy-lock.json` a mano es frágil; `antigravity.lua` debe adaptarse a nuestras claves `<leader>a` ya usadas por `claude-code.lua:11-32`) |
| **#188** | Preservar `~/.oh-my-zsh` existente: instalar con el instalador oficial **sólo si falta**; borrar la copia vendorizada de 1 021 ficheros; `PathExists` | `installer/internal/system/exec.go`, `installer/internal/tui/installer.go`, `installation_steps_test.go`, `docs/manual-installation.md`, `GentlemanZsh/.oh-my-zsh/**` | Sí (código equivalente en Go propio) | **No**: ya instalamos omz con el instalador oficial **pinneado a un commit** sólo cuando falta (`installer.go:1300-1362`: `ohMyZshInstallerURL`, `ohMyZshInstallerRef`, `ohMyZshEntrypoint`, `shouldInstallOhMyZsh`, `installOhMyZsh`), usado en `installer.go:1990-1996`; `system/exec.go:482-487` tiene `PathExists` resolviendo symlinks | `ALREADY FIXED` | — | — |
| **#185** | Arch: instalar `carapace` desde release estático con verificación SHA-256; quitarlo de las listas Arch | `docs/manual-installation.md`, `installer/internal/tui/installer.go`, `platform_packages_test.go` | Sí, pero no `installCarapaceBinary` | **No el defecto** (`#183`): ya filtramos el nombre AUR y avisamos + ofrecemos Homebrew (`archUnavailable`/`logArchUnavailable`, `installer.go:1167-1197`). **Sí la mejora**: no descargamos el binario de release | `PARTIAL` (defecto cubierto; la descarga cómoda no) | M | Bajo si no se porta; medio si se porta (tercer binario pinneado por SHA a mantener) |
| **#170** | Soportar distros atómicas/inmutables (Silverblue, Bazzite, Kinoite): detectar `rpm-ostree`/flatpak, preferir Homebrew sobre `sudo dnf/apt`, saltar cambios en `/etc/shells`, instrucciones manuales si no hay brew/flatpak. Cierra `#165` | `installer/cmd/gentleman-installer/main.go`, `installer/internal/system/{detect.go,detect_test.go,exec.go}`, `installer/internal/tui/{installer.go,interactive.go,model.go}` | Sí (Go propio; nombres de cmd distintos: `installer/cmd/dotfiles/main.go`) | **Sí**: no hay ningún rastro de atómicas; `installer/internal/system/detect.go:11-21` sólo tiene `OSMac/OSLinux/OSArch/OSDebian/OSFedora/OSTermux/OSWSL/OSUnknown`, sin `IsAtomic`/`HasFlatpak` | `APPLIES` | L | Medio-alto (toca planificación de paquetes, ya muy reescrita aquí; riesgo de duplicar el fallback brew existente) |
| **#124** | Selector de leader key de nvim en el instalador (`<Space>`, `,`, `\`) + remapeo de `flash.nvim`. **PR en conflicto** con `main` | `installer/gentleman-installer`, `installer/go.mod`, `go.sum`, `installer/internal/tui/{installer.go,model.go,update.go,view.go}`, `leader_nav_test.go` | Los ficheros sí; el mecanismo no | **N/A**: nuestro instalador no genera opciones de nvim, copia `dotfiles-nvim/nvim` tal cual (`repoassets.go:29`); `mapleader` no se declara en nuestro árbol (sólo un comentario en `dotfiles-nvim/nvim/lua/teacher/dump.lua:48`). El "LeaderMode" de `installer/internal/tui/model.go:231-232` es el atajo vim-like de **nuestra TUI**, no el leader de nvim | `NOT APPLICABLE` (divergencia + es feature, no defecto) | — | — |

---

## 2. Issues abiertos

| # | Qué pide / qué defecto | Ficheros upstream implicados | ¿Existe en nuestro árbol? | ¿Tenemos el mismo defecto? (prueba) | Veredicto | Esfuerzo | Riesgo |
|---|---|---|---|---|---|---|---|
| **#210** | zsh: el WM nunca autostartea con el instant prompt de p10k activo (raíz de `#211`) | `GentlemanZsh/.zshrc` | Sí | **Sí, idéntico**: `dotfiles-zsh/.zshrc:4` vs `:832` | `APPLIES` (duplicado de `#211`) | S | Medio |
| **#208** | `opencode.nix`: el hook `home.activation.installOpenCode` sin guardar aborta todo el `home-manager switch` si falla la red | `opencode.nix` | **No** existe `opencode.nix` en nuestro árbol (sólo `herdr.nix` en la raíz) | **N/A**: no enviamos ese módulo | `NOT APPLICABLE` | — | — |
| **#207** | `herdr.nix`: el hook de activación sondea `command -v herdr/brew` sin prepender Homebrew al `PATH` → warning falso y rama `brew install` inalcanzable | `herdr.nix` | **Sí** → `herdr.nix` (idéntico al upstream en este punto) | **Sí**: `herdr.nix:13-20` sondea `command -v herdr`/`brew` sin `export PATH=…homebrew…` | `APPLIES` | XS | Bajo (2 líneas; `engram.nix` del upstream ya muestra el patrón) |
| **#206** | zsh: punto de extensión estable (`~/.zshrc.d/*.zsh`) que sobreviva a las actualizaciones; hoy se reemplaza `~/.zshrc` entero | `GentlemanZsh/.zshrc`, `installer/internal/tui/installer.go` | Sí | **Sí, funcionalmente**: `installer.go:1928-1932` copia `dotfiles-zsh/.zshrc` sobre `~/.zshrc`; no hay loader `.zshrc.d` en `dotfiles-zsh/.zshrc` (grep sin resultados). Mitigado: `~/.zshrc` está en `ConfigPaths` (`system/exec.go:513`) y se respalda | `APPLIES` | S | Bajo (añadir un loader al final; el respaldo ya protege) |
| **#205** | zsh: integrar la navegación del menú de `zsh-autocomplete` con fzf (widget trigger-aware) | `GentlemanZsh/.zshrc` | Sí | **No aplica tal cual**: `zsh-autocomplete` está **desactivado a propósito** por lag (`dotfiles-zsh/.zshrc:361`); las completaciones van por `fzf-tab` (`:471-512`) | `NOT APPLICABLE` (nuestra ruta ya es distinta) | — | — |
| **#200** | Linux: fuente emoji gestionada + renderizado Unicode (es el issue que implementa `#201`) | trainer, docs, e2e | Sí | Igual que `#201` | `PARTIAL` | M | Medio |
| **#199** | `zsh-autocomplete` 26.08.04 sobre zsh 5.9 dibuja filas en blanco sobre el menú | `GentlemanZsh/.zshrc` (carga del plugin) | Sí | **No**: no cargamos `zsh-autocomplete` en Linux/macOS (`dotfiles-zsh/.zshrc:361`), sólo en Termux (`:342`) | `NOT APPLICABLE` | — | — |
| **#198** | Omarchy: no generar el autostart de Herdr (los terminales independientes se reflejan entre sí); confiar en el atajo nativo `Super+Ctrl+Return` | plantillas fish/zsh/nushell del instalador | Plantillas sí; detección de Omarchy no | **Parcial**: no hay detección de Omarchy en `detect.go`; el bloque herdr de fish sigue siendo incondicional (`installer/internal/system/exec.go:1022-1028`) y nuestro `.zshrc` arranca `herdr` (`dotfiles-zsh/.zshrc:401,832`). Nuestro arreglo vecino (`#191`, sesiones tmux únicas) **no** cubre herdr | `PARTIAL` | S-M | Medio (decisión de producto: ¿queremos detección de Omarchy o basta con no arrancar herdr sin `HERDR_ENV`?) |
| **#196** | El instalador sobrescribe `~/.config/fish` con la copia estática y destruye `config.fish` sin aviso | `installer/internal/tui/installer.go:737`, `docs/manual-installation.md:382` | Sí | **Sí, versión suave**: `installer.go:1834` hace `CopyDir(dotfiles-fish/fish → ~/.config/fish)`; `CopyDir` **fusiona** (no borra `dst`-only, así que `conf.d/` y funciones del usuario sobreviven) pero `CopyFile` **sobrescribe `config.fish`**. Mitigado: `~/.config/fish` está en `ConfigPaths` (`system/exec.go:512`) y se respalda. Sigue siendo destructivo en `docs/manual-installation.md:450` (`cp -rf dotfiles-fish/fish ~/.config`) | `PARTIAL` (**APPLIES como pérdida de datos suave**) | S-M | Medio (hay que decidir: backup explícito + aviso, merge o punto de extensión) |
| **#195** | El backup aborta si el directorio contiene un socket Unix | `installer/internal/system/exec.go` | Sí | **No**: `CopyFile` devuelve `ErrNotRegularFile` y `CopyDirReport` acumula `skipped` (`system/exec.go:290-336,419-422`); `CreateBackup` lo propaga y no aborta (`:586-645`) | `ALREADY FIXED` | — | — |
| **#194** | `docker-test.sh` con `set -e` sin `pipefail` reporta éxito si falla un comando dentro de una pipe | `installer/e2e/docker-test.sh` | Sí | **No**: el script evita a propósito la pipe que enmascaraba el status (`installer/e2e/docker-test.sh:271-278`, con el comentario explicando el bug) y captura el estado con `|| run_status=$?`. Las pipes restantes (`:291`) parsean un **fichero**, no ejecutan el test | `ALREADY FIXED` (mitigado) | — | — |
| **#193** | `rm -rf Gentleman.Dots` relativo al CWD puede borrar un directorio ajeno | `installer/internal/tui/installer.go:111,1199` | Sí | **No**: el clon vive en `os.MkdirTemp` y se guarda en `m.WorkDir` (`installer.go:151-197`); se limpió la ruta relativa y el comentario `:154-157` documenta justo este defecto. No queda ningún `rm -rf` con ruta relativa de repo | `ALREADY FIXED` | — | — |
| **#192** | Arch+Fish: el instalador invoca `brew install …` cuando brew no existe → exit 127 | `installer/internal/tui/installer.go` | Sí | **No**: `planPlatformInstall` elige `pacman` en Arch y sólo cae a brew si `HasBrew` (`installer.go:1400-1430`); `runNativeWithBrewFallback` exige `hasBrew` (`:1478-1485`) | `ALREADY FIXED` | — | — |
| **#190** | Linux: (1) `--dry-run` ejecuta trabajo real; (2) Kitty oculto en la TUI Linux y no instalado; (3) paquetes Arch inválidos | `installer/cmd/…/main.go`, `installer/internal/tui/{model.go,installer.go}` | Sí | **(1) No**: `DOTFILES_DRY_RUN` + `executeStep` (`installer.go:50-60,90-102`) saltan el step completo. **(2) Sí**: sólo macOS ofrece Kitty (`model.go:382-385`) y el backend dice "Kitty is only installable on macOS" (`installer.go:847-870`). **(3) No**: filtro `archUnavailable` | `PARTIAL` → sólo (2) `APPLIES` | S | Bajo (decidir si queremos Kitty en Linux; el resto ya está) |
| **#187** | El instalador sobrescribe `~/.oh-my-zsh` con una copia estática y rompe `omz update` | `installer/internal/tui/installer.go:804`, `GentlemanZsh/.oh-my-zsh/**` | No shipamos `.oh-my-zsh`; sí el instalador | **No**: instalamos omz con el instalador oficial pinneado y sólo si falta (`installer.go:1300-1362,1990-1996`); `dotfiles-zsh/` sólo tiene `.zshrc`, `.zshenv`, `.p10k.zsh` | `ALREADY FIXED` | — | — |
| **#184** | Fedora 44: audit — COPR de Ghostty desactualizado, paquetes Fedora inexistentes (`carapace`, `starship`, `lazygit`), revisar métodos de instalación | `installer/internal/tui/installer.go`, listas Fedora | Sí | **Parcial**: los paquetes Fedora ya se filtran y avisan (`fedoraUnavailable`, `installer.go:1219-1265`). El COPR sigue siendo `dnf copr enable -y pgdev/ghostty` (`installer.go:895`) — **no verificable** sin un Fedora 44. El resto es propuesta de arquitectura | `PARTIAL` | S | Bajo (paquetes) / indeterminado (COPR) |
| **#183** | CachyOS/Arch: "Install Zsh" falla porque `carapace`/`zsh-theme-powerlevel10k` no existen en repos oficiales | `installer/internal/tui/installer.go` | Sí | **No**: filtro `archUnavailable` + aviso (`installer.go:1167-1217`) | `ALREADY FIXED` | — | — |
| **#181** | WezTerm + WSL2 + Zellij + Nushell: las secuencias ANSI salen como texto literal | `wezterm.lua`, config WSL/nushell/zellij | Parcial (no shipamos `wezterm.lua`; sí `dotfiles-wsl/wsl.conf` y `dotfiles-nushell/{env,config}.nu`) | **Sin determinar**: no encontramos ningún manejo de `TERM`/terminfo en `dotfiles-wsl/wsl.conf`, `dotfiles-nushell/env.nu` ni `config.nu`. El issue apunta a un terminfo/`TERM` incorrecto en la combinación del usuario; sin reproducir en ese stack no se puede afirmar ni negar | `INDETERMINADO` | ? | ? |
| **#180** | Migrar `gemini-cli.nvim` a `antigravity-cli` | nvim plugins | Sí | Igual que `#189` | `APPLIES` (= `#189`) | M | Medio |
| **#176** | Arch: `sudo pacman -S … zsh carapace …` corta la instalación | `installer/internal/tui/installer.go` | Sí | **No**: filtro `archUnavailable` | `ALREADY FIXED` | — | — |
| **#175** | El tap de Homebrew sirve v2.12.1 en vez de v2.12.2 | `homebrew-tap/Formula/gentleman-dots.rb` | Tenemos **nuestro** tap: `homebrew-tap/Formula/dotfiles.rb` | **N/A**: el tap es propio y descarga **nuestros** releases (`version "0.3.0"`, `github.com/albersg/dotfiles/releases/…`). El versionado upstream no nos afecta | `NOT APPLICABLE` | — | — |
| **#174** | Arch: mismatch de nombres (`carapace`, `zsh-theme-powerlevel10k` son AUR) | `installer/internal/tui/installer.go` | Sí | **No**: filtro `archUnavailable` | `ALREADY FIXED` | — | — |
| **#173** | `plugin/gemini.vim` del plugin autoejecuta `setup()` → doble ejecución y prompt bloqueante bajo lazy.nvim | `GentlemanNvim/nvim/lua/plugins/gemini.lua` | Sí | **Sí**: `dotfiles-nvim/nvim/lua/plugins/gemini.lua:3-5` llama `require("gemini").setup()` en `config`; `disabled.lua:27-29` lo desactiva, pero el spec sigue invocando setup cuando el usuario lo habilite | `APPLIES` (= `#189`) | M | Medio |
| **#165** | Soportar distros inmutables/atómicas (es el issue que implementa `#170`) | instalador | Sí | Igual que `#170` | `APPLIES` (= `#170`) | L | Medio-alto |

---

## 3. Orden de implementación recomendado (lotes revisables)

Cada lote es un PR revisable y autocontenido. Dentro del lote, el orden es el de la tabla.

### Lote 0 — Integridad de datos y arranque roto (primero, por impacto)
1. **`#196` fish: no destruir `config.fish`** — pérdida de datos real y silenciosa (hoy sólo mitigada por backup). Documentar en `docs/manual-installation.md:450`.
2. **`#206` zsh: punto de extensión `~/.zshrc.d`** — misma clase de pérdida: las personalizaciones se respaldan pero **no se recargan**. `#206` y `#211` tocan el mismo `.zshrc`.
3. **`#211`/`#210` zsh: lanzador del WM por encima del instant prompt** — bug funcional que deja al usuario con un shell pelado; va aquí porque toca el mismo fichero que `#206` (hacer los dos en el mismo PR o en dos PR encadenados).

*Por qué primero*: son las dos únicas pérdidas de datos heredadas y el único arreglo que repara una funcionalidad que hoy está rota de forma determinista en una configuración soportada (zsh + p10k).

### Lote 1 — Seguridad del instalador y paquetes
4. **`#190`(2) Kitty en Linux** (decidir: habilitarlo o documentar que no) — resto de `#190` ya está.
5. **`#198` Omarchy + herdr** (decisión de producto; si se aplica, se toca el mismo bloque que `#191`).
6. **`#184` COPR de Ghostty en Fedora 44** — sólo si se confirma en un Fedora 44.
   > `#197`, `#202`, `#185`, `#188`, `#192`, `#193`, `#195`, `#194` **no entran**: ya resueltos. No reabrir.

### Lote 2 — Contenido de shells y terminales (rápido, bajo riesgo)
7. **`#191` fish** (`config.fish`: URL de fisher, `PATH`, `MANPAGER`, `clear`) + **`#191` tmux** (`tmux.conf`: `M-l`, `history-limit`) + **`#191` instalador** (`exec.go:1033` y el mismo cambio en la plantilla `config.fish:43`).
8. **`#203` ghostty 1.3** (`dotfiles-ghostty/config:20,29`).
9. **`#212` herdr** (`prompt_new_tab_name = false`).

### Lote 3 — herdr/nix
10. **`#207` `herdr.nix` PATH en activación** (2 líneas; puede ir con el lote 2 si se prefiere).

### Lote 4 — nvim
11. **`#189`/`#180`/`#173` migración gemini → antigravity** (`gemini.lua`, `antigravity.lua`, `disabled.lua`, `lazy-lock.json`, `docs/ai-configuration.md`).

### Lote 5 — Features grandes, sólo si se prioriza
12. **`#170`/`#165` distros atómicas** (L; alto riesgo de colisión con nuestro `planPlatformInstall`).
13. **`#201`/`#200` Unicode en el trainer** (sólo la parte de rune-safety; la parte de fuente emoji contradice nuestra decisión de no usar emoji).

---

## 4. Lo que NO hay que traer, y por qué

| Elemento | Razón |
|---|---|
| `#197`, `#202`, `#185`, `#192` (paquetes/borrado Arch) | Nuestro instalador ya resuelve el defecto por otra vía: `archUnavailable`/`archPackages`, clon en `MkdirTemp`, `planPlatformInstall` con `HasBrew`. Portarlos duplicaría lógica y chocaría con `installer.go`. |
| `#188`, `#187` (oh-my-zsh) | Ya no vendorizamos `.oh-my-zsh` y lo instalamos con el instalador oficial pinneado (`installer.go:1300-1362`). El enfoque upstream (versión sin pin) **no** es mejor para nosotros: perderíamos el pin a commit. |
| `#195`, `#193`, `#194` | Ya arreglados (socket, `rm -rf` relativo, pipe enmascarada). |
| `#208` (`opencode.nix`) | No enviamos ese módulo. |
| `#199`, `#205` (`zsh-autocomplete`) | Desactivamos `zsh-autocomplete` a propósito (`dotfiles-zsh/.zshrc:361`); las completaciones son de `fzf-tab`. Portar el widget trigger-aware sería trabajo para un plugin que no cargamos. |
| `#124` (selector de leader key) | PR en conflicto y feature, no defecto; nuestro instalador copia `dotfiles-nvim/nvim` estático y no genera opciones. Además, grantear branding/UX: nuestra TUI tiene su propio "leader mode" que no es el de nvim. |
| `#175` (tap v2.12.1) | Nuestro tap (`homebrew-tap/Formula/dotfiles.rb`) publica nuestros releases; el versionado upstream es irrelevante. |
| `#204` (sonidos herdr) | Ya presente **verbatim** en `dotfiles-herdr/config.toml:34-45`. |
| `#201` "fuente emoji gestionada" | Contradice la decisión documentada en `installer/internal/tui/view.go:969-970,2233-2254` (nada de emoji porque un terminal sin fuente emoji dibuja cajas). Traer la fuente emoji resolvería un problema que decidimos no tener. |
| `GentlemanZsh/.oh-my-zsh/**`, `GentlemanZsh/.zshrc` "tal cual" | Branding/divergencia: nuestro `.zshrc` es 43 KB con paleta, caché de carapace, fzf-tab, `WM_CMD` como array, rama WSL y rama Termux que el upstream no tiene. Los parches se adaptan, no se copian. |

---

## 5. Pérdidas de datos / borrado peligroso (marcados como tales)

Estos van **primero** aunque su arreglo esté en el instalador, porque su fallo es irreversible en la práctica.

| Marca | Filas | Defecto | Evidencia | Estado |
|---|---|---|---|---|
| **PÉRDIDA DE DATOS** | `#196` | `~/.config/fish/config.fish` del usuario se sobrescribe sin merge ni aviso; sólo se salva si el usuario aceptó el backup | `installer.go:1834`; `ConfigPaths()` incluye `fish` (`system/exec.go:512`); doc destructiva en `docs/manual-installation.md:450` | Heredado (suave: hay backup) |
| **PÉRDIDA DE DATOS** | `#206` | `~/.zshrc` se reemplaza entero cada instalación; las personalizaciones del usuario quedan inertes aunque el fichero respaldado exista | `installer.go:1928`; respaldo en `system/exec.go:513` | Heredado (suave: hay backup, no hay recarga) |
| — (ya seguros) | `#187`, `#193`, `#195`, `#194` | El upstream los describe como pérdida de datos/borrado peligroso; **aquí no existen** | Ver §1/§2 | Cerrados |

No se ha encontrado ningún `rm -rf` relativo al CWD en nuestro árbol (sólo quedan `rm -rf "$ALACRITTY_DIR"` y `rm -rf "$WY_DIR"` en `installer/internal/tui/interactive.go:211,531`, con la variable definida a partir de `$HOME`, no del CWD).

---

## 6. Parches para cada `APPLIES` (referencia exacta + riesgo de portado a mano)

### A. `#212` — herdr: no pedir nombre de pestaña
- **Upstream**: `herdr/config.toml`, añade `prompt_new_tab_name = false` en `[ui]` (6 líneas con comentario).
- **Nuestro destino**: `dotfiles-herdr/config.toml`, dentro de `[ui]` (línea ~24-32, junto a `accent`).
- **Riesgo del portado**: **bajo**. Fichero distinto al upstream (nuestro tema `vesper`, sinónimos de `[ui]`), pero la clave es independiente. Añadir la clave + comentario equivalente.

### B. `#211` + `#210` — zsh: lanzador del WM por encima del instant prompt
- **Upstream**: `GentlemanZsh/.zshrc` mueve el bloque `WM_VAR`/`WM_CMD`/`function start_if_needed` y la llamada `start_if_needed` por encima del bloque `if [[ -r … p10k-instant-prompt … ]]`; añade `installer/internal/system/zsh_template_test.go`.
- **Nuestro destino**: `dotfiles-zsh/.zshrc`.
  - Mover a **después de la línea 1** (`export ZSH=…`) y **antes de la línea 4** (instant prompt) el bloque que hoy está en `:397-406` (`WM_VAR="$HERDR_ENV"`, `typeset -a WM_CMD`, `WM_CMD=(herdr)`, `function start_if_needed`) **más** la llamada de `:832`.
  - Ojo: nuestra función usa `command -v "${WM_CMD[1]}"` y `exec "${WM_CMD[@]}"`, no `WM_CMD="tmux"`.
- **Test a portar**: `installer/internal/system/zsh_template_test.go` (nuevo) — adaptando la ruta a `dotfiles-zsh/.zshrc` y el banner de `PatchZshForWM` (`exec.go:824-884`).
- **Riesgo del portado**: **medio**. El `git mv` no encaja literal: nuestro fichero tiene 850+ líneas, bloque `WM_CMD` como array, y `PatchZshForWM` reescribe las líneas `WM_VAR=…` y la condición de `start_if_needed` in situ, así que hay que comprobar que el parcheo sigue funcionando tras mover el bloque (el test del PR cubre exactamente eso).

### C. `#203` — ghostty 1.3
- **Upstream**: `GentlemanGhostty/config`, 2 sustituciones.
- **Nuestro destino**: `dotfiles-ghostty/config:20` `background-blur-radius` → `background-blur`; `:29` `gtk-tabs-location = hidden` → `window-show-tab-bar = never`.
- **Riesgo del portado**: **bajo-medio**. Nuestro `config` deriva del upstream pero con extras; las dos líneas están intactas. Decidir el suelo de versión de Ghostty (si se soporta <1.3, la clave nueva no existe y Ghostty avisa).

### D. `#191` — fish / tmux / instalador
- **Upstream (`GentlemanFish/fish/config.fish`)**: `curl -sL https://raw.githubusercontent.com/jorgebucaran/fisher/main/functions/fisher.fish | source` + `and fisher install …`; quitar `$HOME/.config` y `/usr/local/lib/*` del `PATH` (macOS y Linux); borrar `set -x PATH $HOME/.cargo/bin $PATH`; añadir `set -gx MANPAGER "sh -c 'col -bx | bat -l man -p'"`; quitar `clear` final.
- **Nuestro destino**: `dotfiles-fish/fish/config.fish:5-6` (URL), `:29` y `:33` (PATH), `:52` (cargo dup), `:122` (`clear`), añadir `MANPAGER` tras `:78`.
- **Upstream (`GentlemanTmux/tmux.conf`)**: `bind-key -n M-l display-popup … "lazygit"` y `set -g history-limit 100000`.
- **Nuestro destino**: `dotfiles-tmux/tmux.conf:33-34` (tras el popup `M-g`) y `:78` (antes de TPM).
- **Upstream (`installer/internal/system/exec.go`)**: en `fishMultiplexerBlock` default, `tmux new-session -A -s main` → `tmux new-session -s "term-$(date +%s)-$(random)"`.
- **Nuestro destino**: `installer/internal/system/exec.go:1033` (mismo texto). **Y la plantilla** `dotfiles-fish/fish/config.fish:43` tiene el mismo `-A -s main`; si no se cambia, el bloque de la plantilla (para quien no pasa por el parcheo) sigue espejando. Portar también `installer/internal/system/patch_test.go` (añadir `wantNotContain`).
- **Riesgo del portado**: **bajo**. Los tres ficheros son casi idénticos al upstream.

### E. `#207` — herdr.nix PATH en activación
- **Upstream**: `herdr.nix`, prepender Homebrew (`/opt/homebrew/bin` en Darwin, `/home/linuxbrew/.linuxbrew/bin` en Linux) **antes** de los `command -v`.
- **Nuestro destino**: `herdr.nix:10-20`.
- **Riesgo del portado**: **bajo**. Fichero idéntico al upstream en el bloque afectado.

### F. `#206` — punto de extensión zsh
- **Upstream**: hilo de issue (sin PR); propone cargar `~/.zshrc.d/*.zsh` en orden léxico cerca del final de `.zshrc`, antes del arranque del multiplexer.
- **Nuestro destino**: `dotfiles-zsh/.zshrc`, justo antes de la sección del WM (que tras el lote anterior quedará arriba; entonces, al final del fichero, después de la última sección de usuario y antes de nada que dependa de ella).
- **Riesgo del portado**: **bajo**; decidir la ubicación exacta y documentarlo en `docs/manual-installation.md`.

### G. `#189` + `#180` + `#173` — gemini → antigravity (nvim)
- **Upstream**: `GentlemanNvim/nvim/lua/plugins/antigravity.lua` (nuevo, 32 líneas), borra `gemini.lua`, añade la entrada en `disabled.lua`, regenera `lazy-lock.json`, actualiza `docs/ai-configuration.md`.
- **Nuestro destino**: `dotfiles-nvim/nvim/lua/plugins/antigravity.lua` (nuevo), borrar `dotfiles-nvim/nvim/lua/plugins/gemini.lua`, añadir a `dotfiles-nvim/nvim/lua/plugins/disabled.lua` (junto a la entrada de `gemini-cli.nvim` en `:26-29`), regenerar `dotfiles-nvim/nvim/lazy-lock.json` (hoy tiene `gemini-cli.nvim` en `:13`), actualizar `docs/ai-configuration.md`.
- **Ajuste obligatorio**: el PR usa `<leader>a*`; en nuestro árbol esas claves ya las ocupa `claude-code.lua:11-32`. Mantener `enabled = false` y no declarar `keys` globales en conflicto (o documentar que sólo uno de los plugins AI se habilita a la vez, como ya hace nuestro `disabled.lua`).
- **Riesgo del portado**: **medio**. `lazy-lock.json` no debe editarse a mano: hay que resolverlo con `lazy.nvim` (o fijar el commit del plugin nuevo) y **no** arrastrar el resto de bumps del upstream (su `lazy-lock.json` incluye ~70 actualizaciones ajenas al cambio).

### H. `#170` + `#165` — distros atómicas
- **Upstream**: `detect.go` añade `IsAtomic`/`HasFlatpak` + `checkAtomic`/`checkFlatpak` (+ tests); `exec.go` añade `RunRpmOstree`/`RunFlatpak`; `installer.go`, `interactive.go`, `model.go` y `main.go` ramifican según atómica.
- **Nuestro destino**: `installer/internal/system/detect.go` (añadir campos y detección, hoy `:11-21`), `installer/internal/system/exec.go`, `installer/internal/tui/installer.go` (`planPlatformInstall` en `:1400-1430` es el punto natural para "brew-first y sin sudo") y `installer/cmd/dotfiles/main.go`.
- **Riesgo del portado**: **alto**. Nuestro `planPlatformInstall`/`archPackages`/`fedoraPackages` ya es una reescritura distinta; injertar la rama atómica sin romper el filtrado por disponibilidad de paquetes requiere diseñarlo aquí, no portar el diff.

### I. `#190`(2) — Kitty en Linux
- **Upstream**: no hay PR propio; el issue pide alinear TUI/CLI/docs/backend.
- **Nuestro destino**: `installer/internal/tui/model.go:381-385` (lista de terminales) y `installer/internal/tui/installer.go:847-870` (instalación y mensaje "sólo macOS").
- **Riesgo del portado**: **bajo**, pero es una **decisión**: si se habilita en Linux hay que instalar el binario (apt/pacman/dnf/brew) o documentar que no se instala.

### J. `#201`/`#200` — Unicode en el trainer (sólo rune-safety)
- **Upstream**: `installer/internal/tui/trainer/simulator.go` (`isWordChar` byte→rune, `runeAtBytePos`, `isEmojiRune`, `findNextEmoji`, ajustar `moveWordForward/Backward/EndOfWord`) + `simulator_unicode_test.go`.
- **Nuestro destino**: `installer/internal/tui/trainer/simulator.go:556-700,908`.
- **Riesgo del portado**: **medio-alto**. Nuestro trainer tiene otra máquina de buffer (ver `odd/tasks/vim-trainer-buffer-engine.md`); portar el diff del upstream puede no compilar. Mejor tratarlo como trabajo propio con test que reproduzca primero un fallo real con UTF-8 en un ejercicio.
- **No traer**: la parte de fuente emoji (`AGENTS.md`, spec, `test_emoji_navigation.sh`) por la decisión de diseño ya documentada en `view.go`.

### K. `#198` — Omarchy + herdr (si se decide arreglar)
- **Upstream**: sin PR; el issue pide detectar Omarchy y no generar el autostart de herdr.
- **Nuestro destino**: `installer/internal/system/exec.go:1022-1028` (bloque herdr de fish) y la rama equivalente de zsh en `dotfiles-zsh/.zshrc:401,832`.
- **Riesgo del portado**: **medio**; requiere decisión de producto primero.

---

## 7. No determinado / pendiente de comprobación

1. **`#181`** (ANSI literales en WezTerm+WSL2+Zellij+Nushell): no shipamos `wezterm.lua` y no hay manejo de `TERM`/terminfo en `dotfiles-wsl/wsl.conf` ni en `dotfiles-nushell/env.nu`/`config.nu`. No se puede afirmar ni negar sin reproducir en ese stack. **Acción**: pedir al reportero `echo $TERM`, `infocmp $TERM | head` y si ocurre con nushell sin zellij.
2. **`#184`** COPR de Ghostty en Fedora 44 (`installer.go:895` usa `pgdev/ghostty`): no verificable aquí. **Acción**: validar en `fedora:44` o contrastar con la doc oficial de Ghostty.
3. **`#201`** ¿algún ejercicio real de nuestro trainer contiene texto multibyte que hoy se navegue mal? No comprobado; haría falta un test RED antes de tocar `simulator.go`.

---

## 8. Informe final

- **`APPLIES`: 11 filas / 8 frentes** → `#211`+`#210` (zsh p10k), `#212` (herdr), `#203` (ghostty), `#189`+`#180`+`#173` (nvim), `#170`+`#165` (atómicas), `#207` (herdr.nix), `#206` (extensión zsh), y los `APPLIES` embebidos en los `PARTIAL`: `#191` (fish/tmux/exec.go), `#190`(2) (Kitty en Linux), `#196` (fish no destructivo), `#198` (Omarchy, opcional).
- **Pérdidas de datos heredadas: 2** → `#196` (se sobrescribe `~/.config/fish/config.fish`) y `#206` (se reemplaza `~/.zshrc` entero sin recargar las personalizaciones). Ambas con backup previo, ninguna con merge ni punto de extensión. Prioridad 1.
- **Orden propuesto**: Lote 0 (datos + arranque zsh) → Lote 1 (seguridad instalador: Kitty/Omarchy/COPR) → Lote 2 (fish/tmux/ghostty/herdr, todo XS-S) → Lote 3 (herdr.nix) → Lote 4 (nvim gemini→antigravity) → Lote 5 (atómicas y Unicode trainer, opcionales).
- **Lo que no hay que traer**: todo el `installer/` ya resuelto (`#197`, `#202`, `#185`, `#188`, `#192`, `#193`, `#194`, `#195`), `opencode.nix` (`#208`), zsh-autocomplete (`#199`, `#205`), leader key (`#124`), tap (`#175`), sonidos herdr (`#204`) y la fuente emoji (`#201`).
- **Lo que no pude determinar**: `#181` (ANSI en el stack WezTerm/WSL2/Zellij/Nushell), la validez del COPR `pgdev/ghostty` en Fedora 44 (`#184`) y si algún ejercicio real del trainer se ve afectado por la navegación byte a byte (`#201`).

---

## 9. Implementación (segunda fase, misma rama, sin commits)

Tras el triaje se implementaron los `APPLIES` que viven en ficheros de contenido, **en un solo escritor** (este worktree). Todo queda en el árbol de trabajo, sin `git add` ni commit. Lo que cae dentro de `installer/` o `.github/` **no se tocó** y se lista en §9.2.

### 9.1 Lotes implementados

| Lote | Upstream | Ficheros tocados | Qué se hizo |
|---|---|---|---|
| **A — zsh** | `#211`, `#210`, `#206` | `dotfiles-zsh/.zshrc`, `docs/manual-installation.md` | El bloque `WM_VAR`/`WM_CMD`/`start_if_needed` **y su llamada** se movieron por encima del instant prompt de p10k; se conservó la forma exacta de las líneas que `PatchZshForWM` reescribe, y se añadió un `export PATH` mínimo antes del launcher (el bloque de PATH del fichero aún no ha corrido ahí). Se añadió el punto de extensión `~/.zshrc.d/*.zsh` al final, con glob `(N.)`. Documentado en `docs/manual-installation.md`. |
| **B — fish/tmux** | `#191` | `dotfiles-fish/fish/config.fish`, `dotfiles-tmux/tmux.conf` | Fisher desde la URL de GitHub (`git.io` está muerto) con `and`; fuera `$HOME/.config` y `/usr/local/lib/*` del `PATH` y el `cargo/bin` duplicado; `MANPAGER` con bat; fuera el `clear` final. tmux: popup `M-l` para lazygit y `history-limit 100000`. Además, el bloque tmux de la plantilla de fish pasó a sesión única por ventana. |
| **C — herdr/ghostty** | `#212`, `#203`, `#207` | `dotfiles-herdr/config.toml`, `dotfiles-ghostty/config`, `herdr.nix` | `prompt_new_tab_name = false`; `background-blur` y `window-show-tab-bar = never`; `herdr.nix` prepara Homebrew en el `PATH` antes de sondear `herdr`/`brew`. |
| **D — nvim** | `#189`, `#180`, `#173` | `dotfiles-nvim/nvim/lua/plugins/antigravity.lua` (nuevo), `.../gemini.lua` (borrado), `.../disabled.lua`, `.../lazy-lock.json`, `.../teacher/plugins.lua`, `docs/ai-configuration.md`, `docs/neovim-keymaps.md` | `antigravity-cli.nvim` sustituye a `gemini-cli.nvim` (que autoejecutaba `setup()`), desactivado como los demás plugins de IA; lock actualizado con el commit del PR; docs alineadas. Se eliminó la fila de Gemini CLI de `docs/ai-configuration.md` porque **nuestro instalador no instala ese CLI** (sólo Claude Code y OpenCode: `installer.go:2544,2555`). |

### 9.2 Pendiente por estar en `installer/` (no tocado)

| Upstream | Fichero | Parche exacto pendiente |
|---|---|---|
| `#196` (pérdida de datos) | `installer/internal/tui/installer.go:1834` | No sobrescribir `config.fish` sin backup/aviso: respaldar antes de `CopyDir(repoAssetFish, ~/.config/fish)` o copiar sólo lo ausente. Ya existe `ConfigPaths()` con `fish`, así que se puede reutilizar `CreateBackup`. |
| `#196` (doc) | hecho | El aviso en `docs/manual-installation.md` sí se añadió (es contenido). |
| `#191` (espejo tmux) | `installer/internal/system/exec.go:1033` (bloque `default` de `fishMultiplexerBlock`) | `tmux new-session -A -s main` → `tmux new-session -s "term-$(date +%s)-$(random)"`, más `wantNotContain` en `installer/internal/system/patch_test.go`. **Sin esto, el parche pisa la plantilla ya corregida** y los usuarios instalados siguen espejando sesiones. |
| `#211` (test) | `installer/internal/system/zsh_template_test.go` (nuevo) | Portar el test del PR adaptando la ruta a `../../../dotfiles-zsh/.zshrc`. Sin él no hay guard de regresión del orden. |
| `#190`(2) | `installer/internal/tui/model.go:382-385`, `installer/internal/tui/installer.go:847-870` | Decidir si Kitty se habilita en Linux (lista de la TUI + instalación real). |
| `#198` | `installer/internal/system/exec.go:1022-1028` | Detección de Omarchy antes de generar el bloque herdr. |
| `#201`/`#200` | `installer/internal/tui/trainer/simulator.go:908` | `isWordChar` byte → rune y helpers de frontera UTF-8. |
| `#170`/`#165` | `installer/internal/system/detect.go:11-21` y `installer/internal/tui/installer.go:1400-1430` | `IsAtomic`/`HasFlatpak` + rama brew-first sin `sudo`. |

### 9.3 Verificación ejecutada

| Comprobación | Resultado |
|---|---|
| `zsh -n dotfiles-zsh/.zshrc` | OK |
| `go test ./... -count=1` (4 paquetes) | 3721 passed |
| `python3 -c json.load(lazy-lock.json)` | 67 entradas, `antigravity-cli.nvim` presente, `gemini-cli.nvim` ausente |
| `nvim --headless -l` sobre `antigravity.lua`, `disabled.lua`, `teacher/plugins.lua` | sintaxis OK |
| Simulación fiel de `PatchZshForWM` (Python) sobre el `.zshrc` real, para `herdr`/`tmux`/`zellij`/`none` | El launcher queda siempre por encima del instant prompt; `none` elimina `WM_VAR`, `WM_CMD`, la función y la llamada |
| Simulación de `PatchFishForWM` sobre el `config.fish` real | El bloque se detecta y sustituye en los cuatro casos |
| `grep` de restos de `gemini-cli` en `dotfiles-nvim/` y `docs/` | Sólo el comentario que explica la migración |
| `fish -n` | **No ejecutado**: fish no está instalado en este host |

### 9.4 Autocrítica

1. **El `export PATH` temprano del `.zshrc` es una desviación deliberada del PR `#211`.** Upstream no lo lleva. Sin él, mover el launcher arriba lo deja buscando el WM antes de que el PATH del fichero se aplique, y un `zellij`/`herdr` de `~/.cargo/bin` o `~/.local/bin` no se encontraría. Lo justifico, pero un revisor puede preferir otra forma (p. ej. resolver la ruta del binario dentro del bloque).
2. **El caso `~/.zshrc.d` no se carga cuando el WM hace `exec`**, porque el `exec` reemplaza el proceso. En la práctica el shell interior de tmux/zellij/herdr vuelve a leer el `.zshrc` completo y sí lo carga, pero conviene tenerlo presente: el punto de extensión sirve para sesiones normales, no para el shell exterior que se convierte en multiplexer.
3. **`fish -n` quedó sin ejecutar** por no haber fish en este host. La sintaxis usada (`and`, `$(...)` en dobles comillas, `set -gx`) es estándar de fish, pero la verificación no es equivalente a ejecutarla.
4. **`lazy-lock.json` se editó a mano** con el commit del PR. Es válido y `lazy.nvim` lo acepta, pero el flujo canónico es `:Lazy sync`/`restore`; si el commit upstream de `antigravity-cli.nvim` ya no existiera, habría que regenerarlo. Además el instalador trata ese fichero como *user-owned* (`installer.go:2394-2397`), así que sólo afecta a instalaciones nuevas.
5. **No toqué `CHANGELOG.md`** a propósito: otras ramas en vuelo (trainer/panel/mascot/ci) pueden editarlo y añadir entradas aquí generaría conflictos; el orquestador puede redactarlas al commitear cada lote.
6. **`#202`/`#185` se marcaron `ALREADY FIXED`/`PARTIAL` sin ejecutar `pacman`**: la evidencia es el código (`archUnavailable`/`archPackages` + tests que ya lo fijan) y el hecho de que los literales AUR no llegan a la lista. No es una verificación en un Arch real.
7. **No leí `#197` entero**, por diseño (instrucción explícita); puede haber detalles internos de sus tests que no evalué, aunque ninguno de sus cuatro defectos declarados existe aquí.

