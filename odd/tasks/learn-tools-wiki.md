# Learn About Tools: completing and correcting the installer's wiki

The installer's in-interface wiki -- Learn About Tools, Keymaps Reference, LazyVim Guide -- ships
four terminals, three shells, three multiplexers and Neovim, and nothing else. The font the run
installs, the prompts, the shell tooling, the plugin sets and the machine toolset have no page;
the shells, multiplexers and Neovim pages are unreachable from the main menu; the Keymaps
Reference has no table for the terminals the repository configures except Ghostty; and one keymap
group documents plugins this configuration ships disabled. This front completes and corrects it.

## Specs

**S1. Alcance.** Verbatim: *"Revisa todo el learn about tools, toda la wiki que tenemos dentro del
installer, mira a ver si falta informacion de algo, de alguna parte, o lo que sea, y compeltalo"*

**S2. Alcance aprobado.** Verbatim (la opción que el humano eligió): *"C) Todo, incluida paridad de
keymaps (puntos 1-5)"*, cuya descripción es: *"B + tablas de keymaps de WezTerm, Alacritty y Kitty,
que hoy se configuran sin referencia en el instalador."* La paridad de keymaps se entrega dentro
del límite de S5: se documenta lo que existe, nunca se inventa una fila.

**S3. Ninguna página describe una configuración que no existe.** `docs/ai-configuration.md:36`
verbatim: *"Any documentation or keymap table that describes Copilot completions, an OpenCode keymap
group, or a Claude Code default describes a configuration that does not exist."* Concretamente:
- La categoría `AI Assistants (Copilot + OpenCode)` de `GetNvimKeymaps()` describe `copilot.lua` y
  `opencode.nvim`, que `dotfiles-nvim/nvim/lua/plugins/disabled.lua` deshabilita. Debe nombrar el
  asistente activo y su binding real, y decir que los demás están deshabilitados y son opcionales.
- `GetNvimInfo()` no puede anunciar `AI integration (Copilot, Avante)`: `avante.nvim` está
  deshabilitado en el mismo fichero.
- `<leader>su - Undotree` solo se publica si el plugin existe en la configuración; hoy no aparece
  ningún plugin de undotree en `dotfiles-nvim/nvim/lua/`.
- Las pistas que dicen `telescope` se nombran como lo que la configuración usa (snacks).
- Tmux: *"No native mouse support (needs config)"* se corrige a lo que es cierto (la opción `mouse`
  existe y viene apagada por defecto).

**S4. Todo lo que el instalador pregunta o provisiona tiene página, y se alcanza desde el menú
principal.** El hub de Learn About Tools lista, y cada fila abre una página: Terminals, Shells,
Multiplexers, Neovim; y después Font, Prompts, Shell tooling, Plugins y Machine toolset. El nombre
del menú y su texto no cambian; `docs/tui-installer.md:86` (*"Explore terminals, shells, and
multiplexers"*) deja de prometer algo que la ruta no daba.

**S5. El contenido es veraz y comprobable.** Cada fila se apoya en un fichero de este repositorio
o en la documentación del propio tool, y el comentario de la tabla nombra la fuente -- como
`keymaps_herdr.go` cita `https://herdr.dev/docs/keyboard/`. Si la fuente no existe, la fila no se
escribe.

**S6. Paridad de keymaps de terminales.** `dotfiles-kitty/kitty.conf` declara 11 `map` (cmd+1..9,
ctrl+shift+c, ctrl+shift+v) y esas filas entran en la referencia. `.wezterm.lua` y `alacritty.toml`
no declaran ningún binding: su entrada dice que la configuración no declara ninguno y no inventa
filas. La referencia se amplía sin quitar las cinco tablas actuales.

**S7. Docs al día.** `docs/tui-installer.md` recibe la sección Learn Mode completada y la lista del
menú principal corregida (la fila se llama "Keymaps Reference", no "Neovim Keymaps"). Si una
página nueva contradice o amplía `docs/tools.md`, se enlaza, sin duplicar la tabla.

**S8. Rojo antes, verde después.** Cada spec con su test: contenido obsoleto (S3), alcance del hub
(S4), paridad de las tablas nuevas (S5, S6) y los guards del repositorio al día
(`TestEveryScreenIsCoveredByAFrameGuard`, `TestEveryScreenFitsEveryTerminalSize`,
`TestGetCurrentOptionsForAllScreens`, `TestGetScreenTitleForAllScreens`, `TestEscapeKeyBehavior`,
`panels_test.go`). No se relaja ningún guard para que pase.

## Tasks

| ID | Tarea | Specs | Ruta | Commit |
|----|-------|-------|------|--------|
| T1 | Corregir el contenido obsoleto (IA, undotree, telescope, mouse de Tmux) y su guard | S3 | inline | `3ea7bf2` |
| T2 | Hub de Learn About Tools y navegación desde el menú principal | S2, S4 | inline | `420c4a1` |
| T3 | Páginas de Font y Prompts | S4, S5 | inline | `9c58ee6` |
| T4 | Páginas de Shell tooling, Plugins y Machine toolset | S4, S5 | inline | `9c58ee6` |
| T5 | Keymaps Reference: Kitty, WezTerm y Alacritty | S5, S6 | inline | `9944a9e` |
| T6 | Docs al día (`tui-installer.md`, `tools.md`) | S7 | inline | `b0627f7` |
| T7 | Suite completa y cierre | S8 | inline | `3f0a29b`, `167e430` + la puerta |

## Log

**L1.** Petición original, verbatim: *"Revisa todo el learn about tools, toda la wiki que tenemos
dentro del installer, mira a ver si falta informacion de algo, de alguna parte, o lo que sea, y
compeltalo"*

**L2.** Corrección operativa del humano, verbatim: *"hay otros lllm tocando los dotfiles, cuidado"*.
Estado medido antes de empezar: `git worktree list` muestra dos frentes ajenos en el mismo clon,
`my-things/dotfiles-utilities` (`feat/utilities-refresh`, sin cambios sin commitear) y
`my-things/dotfiles-vim-trainer` (`feat/vim-trainer-redesign`, un test sin trackear en
`installer/internal/tui/trainer/`), más `odd/tasks/start-installation-flow.md` sin trackear en el
worktree principal. Por eso este frente vive en su propio worktree,
`my-things/dotfiles-learn-tools`, rama `feat/learn-tools-wiki`.

**L3.** Auditoría que originó el frente, con la evidencia de cada punto:
1. `update.go:1766` abre `ScreenLearnTerminals` en seco para la fila "Learn About Tools" del menú
   principal; `ScreenLearnShells`, `ScreenLearnWM` y `ScreenLearnNvim` solo se alcanzan dentro del
   asistente, y `docs/tui-installer.md:86` promete "terminals, shells, and multiplexers".
2. `tools_info.go` no contiene nada de la fuente, los prompts, el shell tooling, los plugin sets ni
   el machine toolset; `docs/tools.md` sí los documenta.
3. `tools_info.go:396` y siguientes: categoría de IA obsoleta, `GetNvimInfo()` con Avante,
   `<leader>su` de undotree, pistas con telescope, afirmación sobre el ratón de Tmux.
4. La referencia de keymaps ofrece cinco tools (`panels.go:866-873`) y ninguna tabla para WezTerm,
   Alacritty ni Kitty.
5. `docs/tui-installer.md:87` sigue llamando "Neovim Keymaps" a la fila "Keymaps Reference".

**L4.** Decisión del humano (verbatim): *"C) Todo, incluida paridad de keymaps (puntos 1-5)"*.

**L6.** T3 y T4 se entregaron en un solo commit: sus cambios viven en los mismos archivos
(`tools_info.go`, `view.go`, `update.go`, `model.go`, `learn_menu.go`, `teatest_test.go`,
`comprehensive_test.go`), así que la frontera entre "páginas de Font y Prompts" y "páginas de
referencia" no se puede cortar por archivo sin partir hunks a mano. Evidencia de la suite del
paquete que obligó a un arreglo: `TestEveryScreenFitsEveryTerminalSize` falló con "the guard
enumerates 66 screens, want the measured 62", porque `measuredScreens` está pinneada a propósito y
no siguió a las pantallas nuevas; ahora vale 69 (57 estados del installer + los 6 que abre la
sección de utilidades + los 6 del trainer) y su comentario dice de dónde sale cada grupo.

También en T4: `markdownTableRowsAfter` ahora salta la primera fila de la sección como encabezado
—sea cual sea su nombre— en vez de reconocer "Keys" y "Plugin", porque la tabla de shell tooling
titula su primera columna "Tool"; y el guard del toolset trata una directiva marcada como dropeada
que el Brewfile sí declara como el caso que la página documenta, no como un desajuste: el
comportamiento real de cada fila lo verifica `filterBrewfile`.

**L7.** T5. La referencia de keymaps ofrecía cinco tools; ahora ofrece ocho. Kitty entra con las
once líneas `map` de `dotfiles-kitty/kitty.conf:68-80` (nueve saltos de pestaña y las dos de
portapapeles) y las otras dos entran con la respuesta exacta: ni `.wezterm.lua` ni `alacritty.toml`
declaran binding alguno, así que sus páginas dicen eso y listan los ajustes que los archivos sí
traen, bajo encabezados que se los prometen de verdad (`Setting | Section | What this configuration
sets`). Los guards leen el archivo: la tabla de Kitty se compara en las dos direcciones contra sus
`map`, y las otras dos fallan si el archivo llega a declarar un binding y si una fila nombra un ajuste
que el archivo no trae. El handler del menú pasó de rutear por índice de fila a rutear por etiqueta:
con el `switch m.Cursor` anterior, insertar una fila arriba mandaba una tool a la página de otra.

Dos renombres, porque el mecanismo dejó de ser sólo del learn: `LearnTableScroll` →
`ReferenceTableScroll` y `handleLearnTableKeys` → `handleReferenceTableKeys`. Y `keymapSources()`
sumó Kitty: el panel cuenta tablas de bindings, y los dos que no declaran ninguno no tienen tabla
que contar, cosa que su comentario ahora dice en vez de afirmar que son "the five tools".
`measuredScreens` pasó de 69 a 73.

**L8.** T6 y T7. Los dos documentos quedaron al día: `docs/tui-installer.md` nombra el hub y cada
página que alcanza, la referencia de keymaps tiene sección propia con las ocho tools y el archivo que
copia cada tabla, y la página de Neovim ya no lista los plugins de IA como vivos; `docs/tools.md` dice
dónde aparece su contenido dentro de la interfaz y que la tabla de shell tooling se copia fila por
fila. Verificación de cierre: `bash scripts/preflight.sh` (la puerta completa, sin caché y en el
orden de CI) más la suite del módulo y los guards dirigidos de cada tarea.

**L9.** T7 encontró dos defectos de guard, no de producto (el hub ya resolvía la fila por etiqueta en
`handleLearnHubKeys`):
1. `TestEveryLearnHubRowOpensItsPageAndComesBack` pulsaba Enter en el índice de la fila dentro de
   `learnToolRows()`, pero el menú del hub inserta una regla entre sus dos secciones, así que desde la
   primera fila de la segunda sección el índice apuntaba a la fila siguiente. Ahora la fila se busca
   por su etiqueta en `GetCurrentOptions()`, que es lo que hace el propio handler.
2. `Screen.String()` era una rebanada indexada por el valor de la constante: una pantalla insertada
   arriba renombraba a su vecina y las más nuevas no tenían nombre, justo en los mensajes que debían
   nombrarlas (el fallo se leía "opens Unknown, want Unknown"). Ahora es un mapa con una entrada por
   constante, y `TestEveryDeclaredScreenHasAName` falla por nombre si se añade una constante sin
   entrada. Arreglo en `3f0a29b`. No se relajó ningún guard.

**L10.** T7, veredicto por paso de la puerta (`scripts/preflight.sh`): `gofmt -l`, `go vet`, `go build
./cmd/dotfiles`, `shellcheck --severity=warning`, la auditoría de marca y `gitleaks` sobre los ocho
commits de la rama pasan. `go test ./...` pasa salvo cuatro tests del instalador de Ghostty
(`installer_test.go`), y eso es del host, no de este frente: `/usr/bin/ghostty` existe en esta
máquina desde abril de 2026, el paso responde "✓ Ghostty already installed" y los tests miden la
descarga que el host ya no necesita. La prueba de que es previo y ajeno: en un worktree sobre el
commit base `4ba4332` (antes de cualquier cambio de este frente) los mismos cuatro tests fallan con
la misma salida. En CI, sin Ghostty instalado, no aparecen. Además, el guard del compañero pinnaba
896 estados de frame y con las nueve pantallas nuevas pasó a 1072: la constante se subió con el
comentario que dice de dónde sale, y ese arreglo va en este commit.

Confirmación medida después de ese arreglo: `go test ./... -count=1` deja `internal/tui/trainer`,
`cmd/dotfiles`, `internal/release` e `internal/system` en verde y sólo esos cuatro fallos del host; el
guard del compañero pasa con 1072 estados. Queda una decisión del humano: si esos cuatro tests deben
volverse independientes del host (hoy stubean `curl` pero no la pregunta "¿ya está instalado?"), es
un frente aparte, fuera del alcance de esta wiki.

**L11.** Entrega. La rama cumplía la política del repo (`docs/GIT-BRANCHING.md`: `main` protegida,
todas las PRs van a `main`, nombre `feat/<descripción>`): 0 commits por detrás de `origin/main`
(base `4ba4332`) y 10 por delante. Push no forzado de `feat/learn-tools-wiki` y PR **#336** contra
`main`. `CONTRIBUTING.md` no pide issue previo y `.github/` no trae plantilla de PR, así que se sigue
su checklist: rama convencional, tests donde aplican, `go build` + `go vet`, documentación al día,
auditoría de marca y commits convencionales. En el cuerpo de la PR se declara el fallo previo y
dependiente del host (los cuatro tests de Ghostty), con la prueba en `4ba4332` y la nota de que en CI
pasan.
```
