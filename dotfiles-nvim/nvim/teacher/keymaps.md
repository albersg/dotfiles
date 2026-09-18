# Guía de keymaps — configuración Neovim/LazyVim del usuario

## 1. Cómo leer esta guía
- `<leader>` es la tecla **Espacio** (`<Space>`). Cuando veas `<leader>ff`, pulsa Espacio y luego `f` y `f`.
- Las tablas son datos reales extraídos de la configuración activa. Lo que no esté aquí, no existe (o es un default de Vim que puedes comprobar con `:map` o `:verbose map <tecla>`).
- Al final del prompt del profesor hay una sección **«Mapeos activos en esta sesión»** generada en vivo. Si contradice esta guía, prevalece la sección en vivo.

## 2. Líder y grupos de which-key
Grupos bajo `<leader>` registrados en la config (con which-key, pulsa el prefijo y espera ~300 ms para ver el menú):

| Prefijo | Grupo | Uso principal |
|---|---|---|
| `<leader><Tab>` | +tabs | pestañas (nueva, cerrar, siguiente...) |
| `<leader>b` | +buffer | operaciones con buffers |
| `<leader>c` | +code | LSP y código (`<leader>cm` Mason, `<leader>cs` esquema de símbolos) |
| `<leader>d` | +debug | depuración con nvim-dap (`dj`/`dk` step, `db` breakpoint, `du` UI, `dc` continue) |
| `<leader>dp` | +profiler | perfilado |
| `<leader>f` | +file/find | buscadores y ficheros vía snacks picker |
| `<leader>g` | +git | git, diffs, blame y lazygit (ver sección 3) |
| `<leader>gh` | +hunks | hunks de mini.diff (stage/reset/preview) |
| `<leader>i` | +Impact (codegraph) | análisis de impacto del símbolo actual |
| `<leader>o` | +Obsidian | notas Obsidian (definido en `which-key.lua`) |
| `<leader>q` | +quit/session | salir y sesiones |
| `<leader>s` | +search | buscadores extra y símbolos (`<leader>sk` keymaps) |
| `<leader>u` | +ui | toggles de UI (`<leader>uk` Screenkey) |
| `<leader>w` | +windows | ventanas (`wd` cerrar, `wm` zoom) |
| `<leader>x` | +diagnostics/quickfix | problemas y listas (trouble) |

> Atajo universal: `<leader>?` muestra los bindings locales; `<leader>sk` busca keymaps.

## 3. Mapeos de usuario (ficheros propios)

### `lua/config/keymaps.lua`
| LHS | Modo | Descripción |
|---|---|---|
| `<C-b>` | i | borra hasta el final de la palabra sin salir de inserción (`<C-o>de`) |
| `<C-c>` | i,n,v | salir del modo (`<C-\><C-n>`) |
| `<leader>uk` | n | Screenkey (mostrar teclas pulsadas en pantalla) |
| `<C-h>` `<C-j>` `<C-k>` `<C-l>` | n | navegación entre paneles integrada con tmux |
| `<C-\>` | n | saltar al último panel activo (tmux nav) |
| `<C-Space>` | n | saltar al siguiente panel (tmux nav) |
| `<leader>oc` | n | Obsidian: marcar/desmarcar checkbox |
| `<leader>ot` | n | Obsidian: insertar template |
| `<leader>oo` | n | Obsidian: abrir nota en la app Obsidian |
| `<leader>ob` | n | Obsidian: mostrar backlinks |
| `<leader>ol` | n | Obsidian: mostrar links |
| `<leader>on` | n | Obsidian: crear nota nueva |
| `<leader>os` | n | Obsidian: buscar notas |
| `<leader>oq` | n | Obsidian: quick switch |
| `-` | n | abrir el directorio padre con Oil |
| `<leader>bq` | n | borrar todos los buffers excepto el actual |
| `<leader>sg` | v | grep del texto seleccionado |
| `<leader>sG` | v | grep del texto seleccionado desde la raíz git |
| `<leader>md` | n | borrar todas las marcas (`delmarks!` + `delmarks A-Z0-9`) |
| `<C-s>` | n | guardar fichero (función SaveFile) |
| `<A-j>` `<A-k>` | i,n,x | deshabilitados a propósito (`<Nop>`) |
| `J` `K` | x | deshabilitados a propósito (`<Nop>`) |

### `lua/config/codegraph.lua` — análisis de impacto (requiere índice y CLI `codegraph`)
| LHS | Modo | Descripción |
|---|---|---|
| `<leader>ia` | n | tests afectados por el fichero actual |
| `<leader>ic` | n | callers del símbolo bajo el cursor |
| `<leader>iC` | n | callees del símbolo bajo el cursor |
| `<leader>ii` | n | impacto del símbolo bajo el cursor |
| `<leader>iq` | n | consulta de símbolos (pide texto) |
| `<leader>is` | n | estado del índice codegraph |

> Los resultados se abren en la quickfix list y en el picker de snacks. Si falta el CLI o el índice `.codegraph`, el comando avisa en lugar de fallar.

### `lua/plugins/git-diff.lua` — diffs, blame y hunks
| LHS | Modo | Descripción |
|---|---|---|
| `<leader>gB` | n | blame en línea de la línea actual (Snacks, inline) |
| `<leader>gc` | n | diff de git por hunks (Snacks picker) |
| `<leader>gC` | n | diff de git contra `origin` (Snacks picker) |
| `<leader>ghs` | n / x | stage del hunk / de la selección (mini.diff) |
| `<leader>ghr` | n / x | reset del hunk / de la selección (mini.diff) |
| `<leader>ghp` | n | previsualizar el hunk (overlay) |
| `<leader>gw` | n | ventana flotante de blame (git.nvim) |
| `<leader>gO` | n / x | abrir el fichero/línea en el remoto git (git.nvim browse) |

> mini.diff ya trae la navegación `]h`/`[h`/`]H`/`[H` y los operadores `gh`/`gH`; `<leader>gh*` son envoltorios descubribles. Los keymaps globales por defecto de git.nvim (`gp`/`gn`/`gr`/`gR`…) están desactivados a propósito.

### `lua/plugins/diffview.lua` — diffview.nvim
| LHS | Modo | Descripción |
|---|---|---|
| `<leader>gd` | n | diffview del árbol de trabajo (`DiffviewOpen`) |
| `<leader>gD` | n | histórico del repositorio (`DiffviewFileHistory`) |
| `<leader>gv` | n | histórico del fichero actual (`DiffviewFileHistory %`) |

### `lua/plugins/editor.lua` (goto-preview)
| LHS | Modo | Descripción |
|---|---|---|
| `gpd` | n | vista previa de definición |
| `gpD` | n | vista previa de declaración |
| `gpi` | n | vista previa de implementación |
| `gpy` | n | vista previa de definición de tipo |
| `gpr` | n | vista previa de referencias |
| `gP` | n | cerrar todas las ventanas de vista previa |

### `lua/plugins/rip.lua`
| LHS | Modo | Descripción |
|---|---|---|
| `<leader>fs` | n / x | buscar y sustituir en el proyecto (nvim-rip-substitute) |

### `lua/plugins/ui.lua`
| LHS | Modo | Descripción |
|---|---|---|
| `<leader>z` | n | Zen Mode |
| `<leader>fb` | n | Find Buffers (snacks picker, override de la config) |

### `lua/plugins/obsidian.lua` (solo en buffers de notas Obsidian, buffer-local)
| LHS | Modo | Descripción |
|---|---|---|
| `gf` | n | seguir enlace de la nota |
| `<leader>ch` | n | alternar checkbox |
| `<cr>` | n | acción inteligente de Obsidian |

### `lua/plugins/pi-teacher.lua` — chat del profesor
| LHS | Modo | Descripción |
|---|---|---|
| `<leader>a` | n / v | abrir/cerrar el chat del profesor (pi) |
| `<C-q>` | t / n | (dentro del float) volver al archivo |

> Nota: `oil.lua` define `<leader>E` (Oil flotante) y `<leader>-`, pero los defaults de LazyVim los tapan: en esta sesión `<leader>E` abre el explorador y `<leader>-` parte la ventana; el único atajo de Oil alcanzable es `-`.

## 4. Defaults de LazyVim más útiles

Nota: **todo lo de abajo son defaults de LazyVim**, no keymaps propios. Es un subconjunto curado de conveniencia; la sección en vivo al final del prompt es la fuente autoritativa.

### Ficheros y buscadores (snacks picker)
| Keymap | Acción |
|---|---|
| `<leader>ff` | buscar ficheros (raíz del proyecto) |
| `<leader>fF` | buscar ficheros (cwd) |
| `<leader>fr` | ficheros recientes |
| `<leader>fg` | buscar ficheros (git-files) |
| `<leader>sg` | grep live (raíz del proyecto) |
| `<leader>fb` | buffers |
| `<leader>sk` | buscar keymaps |
| `<leader>sm` | buscar marcas (marks) |
| `<leader>/` | grep raíz del proyecto |
| `<leader>,` | buffers abiertos |
| `<leader><leader>` | buscar ficheros (raíz del proyecto) |

### Buffers, ventanas, salir
| Keymap | Acción |
|---|---|
| `[b` / `]b` | buffer anterior / siguiente |
| `<leader>bb` | alternar con el buffer anterior |
| `<leader>bd` | borrar buffer (conservando la ventana) |
| `<leader>bD` | borrar buffer y ventana |
| `<C-w>h/j/k/l` | mover entre ventanas (además `<C-h/j/k/l>` vía tmux nav) |
| `<leader>w` | grupo de ventanas; `<leader>wd` cierra la ventana actual |
| `<leader>qq` | salir (quit all) |
| `<leader>qs` | restaurar sesión |

### Diagnostics y listas
| Keymap | Acción |
|---|---|
| `]d` / `[d` | siguiente / anterior diagnostic |
| `<leader>xx` | diagnostics del buffer (trouble) |
| `<leader>xX` | diagnostics del buffer (trouble) |
| `<leader>xt` | TODO/FIXME del proyecto (trouble) |
| `<leader>xT` | TODO/FIXME (trouble) |
| `<leader>xL` | lista de loclist (trouble) |
| `<leader>xQ` | quickfix list (trouble) |

### LSP y código
| Keymap | Acción |
|---|---|
| `gd` | ir a definición (buffer-local, requiere LSP) |
| `gr` | referencias/definiciones/... (nvim 0.11+, requiere LSP) |
| `grn` | renombrar símbolo |
| `gra` | code action |
| `gri` | ir a implementación |
| `grr` | referencias |
| `grt` | definición de tipo |
| `gai` / `gao` | call hierarchy: llamadas entrantes / salientes (buffer-local, requiere LSP) |
| `K` | documentación (hover) del símbolo bajo el cursor |
| `<leader>ca` | code actions (menú, buffer-local con LSP) |
| `<leader>cm` | Mason |
| `<leader>cs` | esquema de símbolos del fichero |
| `gpd/gpD/gpi/gpy/gpr/gP` | goto-preview (ver sección 3) |

> Ojo: `<leader>gd` NO es ir a definición; es Diffview (sección 3). Para definición usa `gd` (sin leader).

### Git (defaults LazyVim)
| Keymap | Acción |
|---|---|
| `<leader>gg` / `<leader>gG` | abrir lazygit (raíz del proyecto / cwd) |
| `<leader>gb` | picker de git log/blame de la línea (`Snacks.picker.git_log_line`) |
| `<leader>gf` | histórico del fichero actual |
| `<leader>gl` / `<leader>gL` | git log (raíz / cwd) |
| `<leader>gs` | git status |
| `<leader>gS` | git stash |
| `<leader>ge` | git explorer |
| `<leader>gI` / `<leader>gi` | GitHub issues (todos / abiertos) |
| `<leader>gP` / `<leader>gp` | GitHub pull requests (todos / abiertos) |
| `<leader>gY` | copiar la URL del remoto para la página actual |
| `]h` / `[h` | siguiente / anterior hunk (mini.diff) |
| `]H` / `[H` | primer / último hunk (mini.diff) |
| `gh` / `gH` | aplicar / resetear hunks (mini.diff) |

> Para revisar un cambio (diffs en paralelo, blame, stage por hunk) usa los keymaps de usuario de la sección 3, no estos defaults.

### Movimientos y edición (vanilla + LazyVim)
| Keymap | Acción |
|---|---|
| `zz` / `zt` / `zb` | centrar / arriba / abajo (vanilla) |
| `gg` / `G` / `5G` | inicio / final / línea 5 (vanilla) |
| `gi` | saltar a la última inserción (vanilla) |
| `<leader>un` | descartar todas las notificaciones (`Dismiss All Notifications`) |
| `%` | ir al par que coincide (matchit: `{}`, `()`, tags) |
| `gc` (visual) | comentar selección (mini.comment) |
| `gcc` | comentar línea |
| `y` / `p` / `P` | yank/pegado vanilla; `"+y` al portapapeles del sistema |
| `"a` … `"z` | registros nombrados (vanilla): `"ayy`, `"ap` |

## 5. Comandos útiles de nvim
- `:map`, `:nmap`, `:imap`, `:vmap` — listar mapeos; `:verbose map <tecla>` dice de dónde salió.
- `:help <tema>` — tema de ayuda (p. ej. `:help registers`, `:help jump-motions`, `:help text-objects`).
- `:Oil` — explorador de ficheros Oil (en tu config también con `-`).
- `:Obsidian <subcmd>` — notas (check, template, open, backlinks, links, new, search, quick_switch).
- `:DiffviewOpen`, `:DiffviewFileHistory`, `:DiffviewClose` — diffview.
- `:Screenkey` — mostrar teclas en pantalla (`<leader>uk`).
- `:NzUpdatePlugins` — actualizar plugins de forma segura (script `scripts/safe-update.sh`); `:NzUpdatePlugins check` es la variante de solo comprobación.
- `:Teacher` — abrir/cerrar este chat del profesor (pi).

Fuente de verdad original: `~/.config/nvim/lua/` (`config/keymaps.lua`, `config/codegraph.lua`, `plugins/*.lua`) y el volcado completo generado en vivo al abrir el chat.
