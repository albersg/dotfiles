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
| T1 | Corregir el contenido obsoleto (IA, undotree, telescope, mouse de Tmux) y su guard | S3 | inline | `fix(installer): correct stale learn content` |
| T2 | Hub de Learn About Tools y navegación desde el menú principal | S2, S4 | inline | `feat(installer): reach every learn page from the main menu` |
| T3 | Páginas de Font y Prompts | S4, S5 | inline | `feat(installer): add font and prompt pages` |
| T4 | Páginas de Shell tooling, Plugins y Machine toolset | S4, S5 | inline | `feat(installer): document shell tooling, plugins and the machine toolset` |
| T5 | Keymaps Reference: Kitty, WezTerm y Alacritty | S5, S6 | inline | `feat(installer): add the terminal keymap references` |
| T6 | Docs al día (`tui-installer.md`, `tools.md`) | S7 | inline | `docs(installer): document the completed learn section` |
| T7 | Suite completa y cierre | S8 | inline | -- |

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

**L5.** Reconocimiento que acota S6: `alacritty.toml` no tiene sección `[keyboard]` ni `[[keyboard.bindings]]`
y `.wezterm.lua` no asigna `config.keys`; `dotfiles-kitty/kitty.conf:68-80` declara los once `map`.
```
