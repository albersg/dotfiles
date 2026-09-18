-- Live plugin inventory for the pi "Neovim teacher" chat.
-- Builds the list of plugins that are actually enabled AND installed in the
-- user's config at teacher-open time, so the teacher can prefer a
-- plugin-native solution when it is simpler than vanilla Neovim, and can name
-- the plugin instead of guessing.
--
-- Source (empirically verified on nvim 0.12.4 + lazy.nvim, 2026-09-13):
--   require("lazy.core.config").plugins is the authoritative runtime map.
--   lazy.nvim's own Meta:resolve() (lazy/core/meta.lua fix_cond/fix_disabled)
--   already removed every plugin with `enabled = false` or a function `enabled`
--   that returns false, before this module reads the table. Verified: the
--   disabled.lua entries (precognition.nvim, avante.nvim, codecompanion.nvim,
--   copilot.lua, CopilotChat.nvim, gemini-cli.nvim, opencode.nvim,
--   claudecode.nvim, noice.nvim, bufferline.nvim, smear-cursor.nvim) are
--   ABSENT, while the active core set is present.
--   Rejected alternatives:
--     * lazy-lock.json            -> still lists disabled plugins (6 of the
--                                    disabled entries appear only there).
--     * config.spec.plugins       -> same 63 entries today, but it is the
--                                    pre-resolve spec tree; the runtime map is
--                                    the one lazy itself acts on.
--   Reading the runtime map is dynamic by construction: re-enabling or
--   disabling a plugin automatically adds/removes it on the next teacher open.
--   This module does NOT hardcode the active list.
--
-- The section is cached at ~/.cache/nvim/teacher-plugins.md on every show so a
-- long-lived chat can re-read the freshest list with its `read` tool.
--
-- Performance: a single pass over ~63 lazy spec entries plus one isdirectory()
-- per entry; measured well under the 5 ms budget (see the cheap M.list()).
-- The curated hint lookup is an O(1) hash lookup.

local M = {}

M.cache_path = vim.fn.expand("~/.cache/nvim/teacher-plugins.md")

-- Curated one-line hints (neutral Spanish, tú form) for well-known plugins.
-- Keyed by plugin name first, then by repo basename as a fallback. Plugins not
-- in this table fall back to their lazy `desc`, then to the bare name.
local HINTS = {
  ["LazyVim"] = "framework de configuración: defaults, LSP y keymaps base",
  ["lazy.nvim"] = "gestor de plugins",
  ["snacks.nvim"] = "picker, notificador, terminal, explorador y utilidades",
  ["fzf-lua"] = "buscador difuso (picker) sobre fzf",
  ["telescope.nvim"] = "buscador difuso (picker)",
  ["oil.nvim"] = "gestor de ficheros editando el directorio como un buffer",
  ["mini.files"] = "explorador y gestor de ficheros",
  ["neo-tree.nvim"] = "árbol de ficheros lateral",
  ["harpoon"] = "marcas de ficheros para saltar rápido entre ellos",
  ["trouble.nvim"] = "lista de diagnósticos, quickfix y referencias",
  ["flash.nvim"] = "saltos y motions guiados por etiquetas",
  ["mini.surround"] = "añadir, cambiar o borrar envoltorios (surroundings)",
  ["mini.ai"] = "objetos de texto mejorados",
  ["mini.pairs"] = "cierre automático de paréntesis y comillas",
  ["mini.diff"] = "diff en línea y manejo de hunks de git",
  ["mini.hipatterns"] = "resaltar patrones en el código (colores, TODO)",
  ["mini.icons"] = "iconos por tipo de fichero",
  ["nvim-dap"] = "depurador (Debug Adapter Protocol)",
  ["nvim-dap-ui"] = "interfaz del depurador",
  ["nvim-dap-virtual-text"] = "texto virtual del depurador en el código",
  ["mason.nvim"] = "instalador de LSP, DAP, linters y formatters",
  ["mason-lspconfig.nvim"] = "puente entre mason y nvim-lspconfig",
  ["mason-nvim-dap.nvim"] = "puente entre mason y nvim-dap",
  ["nvim-lspconfig"] = "configuración de clientes LSP",
  ["blink.cmp"] = "autocompletado",
  ["nvim-cmp"] = "autocompletado",
  ["LuaSnip"] = "motor de snippets",
  ["friendly-snippets"] = "colección de snippets",
  ["conform.nvim"] = "formateador de código",
  ["nvim-lint"] = "linters",
  ["obsidian.nvim"] = "notas de Obsidian",
  ["grug-far.nvim"] = "buscar y reemplazar en todo el proyecto",
  ["todo-comments.nvim"] = "resaltar y buscar TODO/FIXME/NOTE",
  ["nvim-treesitter"] = "resaltado y parseo con Treesitter",
  ["nvim-treesitter-textobjects"] = "objetos de texto con Treesitter",
  ["nvim-ts-autotag"] = "cerrar y renombrar etiquetas HTML/JSX",
  ["render-markdown.nvim"] = "renderizar markdown dentro del buffer",
  ["markdown-preview.nvim"] = "vista previa de markdown en el navegador",
  ["which-key.nvim"] = "popup que muestra los atajos disponibles",
  ["lualine.nvim"] = "barra de estado",
  ["bufferline.nvim"] = "pestañas de buffers",
  ["noice.nvim"] = "interfaz de cmdline, mensajes y notificaciones",
  ["persistence.nvim"] = "guardar y restaurar sesiones",
  ["zen-mode.nvim"] = "modo sin distracciones",
  ["twilight.nvim"] = "atenuar el código inactivo",
  ["vim-be-good"] = "minijuego para practicar motions",
  ["nvim-tmux-navigation"] = "navegar entre splits de nvim y tmux",
  ["goto-preview"] = "vista previa de definiciones LSP",
  ["symbols-outline.nvim"] = "esquema de símbolos del fichero",
  ["nvim-docs-view"] = "panel de documentación LSP",
  ["nvim-rip-substitute"] = "sustitución en el proyecto con ripgrep",
  ["incline.nvim"] = "etiqueta flotante con el nombre del fichero",
  ["screenkey.nvim"] = "mostrar las teclas que pulsas",
  ["lazydev.nvim"] = "LSP para la configuración Lua de Neovim",
  ["git.nvim"] = "integración git (blame, abrir en el remoto)",
  ["plenary.nvim"] = "librería Lua de utilidades",
  ["nui.nvim"] = "librería de componentes de interfaz",
  ["nvim-nio"] = "librería de E/S asíncrona",
  ["nvim-web-devicons"] = "iconos por tipo de fichero",
  ["catppuccin"] = "colorscheme",
  ["tokyonight.nvim"] = "colorscheme",
  ["kanagawa.nvim"] = "colorscheme",
  ["SchemaStore.nvim"] = "esquemas JSON para validación",
  ["vim-multiple-cursors"] = "múltiples cursores",
  ["ts-comments.nvim"] = "mejores comentarios según la sintaxis",
  ["lazygit.nvim"] = "interfaz de git (lazygit)",
  ["neogit"] = "interfaz de git dentro de Neovim",
  ["gitsigns.nvim"] = "señales de git en el margen",
  ["neotest"] = "ejecutar y ver tests",
}

--- Repo basename for a spec, used as a secondary hint key. Handles the
--- trailing ".git" and URLs that end in a generic name (e.g. catppuccin/nvim).
---@param name string
---@param spec table
---@return string
local function repo_basename(name, spec)
  local url = spec and spec.url
  if type(url) == "string" and url ~= "" then
    local base = url:gsub("%.git$", ""):match("([^/]+)$")
    if base and base ~= "" then
      return base
    end
  end
  return name
end

--- One-line hint for a plugin, or nil when nothing is known.
---@param name string
---@param spec table
---@return string|nil
local function hint_for(name, spec)
  local hint = HINTS[name] or HINTS[repo_basename(name, spec)]
  if hint and hint ~= "" then
    return hint
  end
  if spec and type(spec.desc) == "string" and spec.desc ~= "" then
    return spec.desc
  end
  return nil
end

--- Resolve the active (enabled + installed) plugin list from lazy.nvim runtime
--- state. Returns [] when lazy.nvim is unavailable (never errors).
---@return table[] list of { name: string, hint: string|nil }
function M.list()
  local ok, cfg = pcall(require, "lazy.core.config")
  if not ok or type(cfg) ~= "table" or type(cfg.plugins) ~= "table" then
    return {}
  end

  local list, seen = {}, {}
  for name, spec in pairs(cfg.plugins) do
    -- Defensive enabled check: lazy already removed disabled plugins from this
    -- table, but a function-valued `enabled` is cheap to re-validate.
    local active = spec.enabled ~= false
    if active and type(spec.enabled) == "function" then
      local eval_ok, result = pcall(spec.enabled, spec)
      if eval_ok and result == false then
        active = false
      end
    end

    -- Installed check: a spec whose dir does not exist cannot be used.
    if active and (type(spec.dir) ~= "string" or vim.fn.isdirectory(spec.dir) == 1) then
      -- Dedupe by repo URL when present, else by name.
      local dkey = (type(spec.url) == "string" and spec.url ~= "") and spec.url or name
      if not seen[dkey] then
        seen[dkey] = true
        list[#list + 1] = { name = name, hint = hint_for(name, spec) }
      end
    end
  end

  table.sort(list, function(a, b)
    local al, bl = a.name:lower(), b.name:lower()
    if al ~= bl then
      return al < bl
    end
    return a.name < b.name
  end)
  return list
end

--- Build the markdown section (neutral Spanish, tú form — this text feeds the
--- chat prompt). Well-formed even when the list is empty.
---@return string
function M.section()
  local parts = {
    "## Plugins activos en tu configuración",
    "",
    "Esta es la lista EN VIVO de los plugins instalados y habilitados ahora mismo. Cuando una tarea sea más fácil o simple con uno de ellos, prefiere y nombra esa forma nativa del plugin antes que la solución vanilla. No recomiendes ni asumas plugins que no aparezcan aquí. El fichero `~/.cache/nvim/teacher-plugins.md` contiene esta sección actualizada; léelo con la herramienta `read` si lo necesitas.",
    "",
  }

  local plugins = M.list()
  if #plugins == 0 then
    parts[#parts + 1] = "(No se detectaron plugins activos en esta sesión.)"
  else
    for _, p in ipairs(plugins) do
      if p.hint then
        parts[#parts + 1] = ("- **%s** — %s"):format(p.name, p.hint)
      else
        parts[#parts + 1] = ("- **%s**"):format(p.name)
      end
    end
  end
  return table.concat(parts, "\n")
end

--- Build the section and write it to the cache file so an already-running chat
--- can re-read it. Mirrors dump.write_cache/usage.write_cache: callable with no
--- argument, and returns the section for reuse in the prompt assembly.
---@param section string|nil pre-built section (built on demand when nil)
---@return string section
function M.write_cache(section)
  section = section or M.section()
  vim.fn.mkdir(vim.fn.fnamemodify(M.cache_path, ":h"), "p")
  local f = io.open(M.cache_path, "w")
  if f then
    f:write(section .. "\n")
    f:close()
  end
  return section
end

return M
