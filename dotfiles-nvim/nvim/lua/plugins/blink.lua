return {
  {
    "saghen/blink.cmp",
    opts = function(_, opts)
      local avante_sources = {
        avante_commands = true,
        avante_files = true,
        avante_mentions = true,
      }
      local sources = opts.sources or {}

      local function remove_avante_sources(source_list)
        if type(source_list) ~= "table" then
          return source_list
        end

        return vim.tbl_filter(function(source)
          return not avante_sources[source]
        end, source_list)
      end

      sources.default = remove_avante_sources(sources.default)
      sources.compat = remove_avante_sources(sources.compat)
      if sources.providers then
        for source in pairs(avante_sources) do
          sources.providers[source] = nil
        end
      end

      opts.sources = sources
    end,
  },
}
