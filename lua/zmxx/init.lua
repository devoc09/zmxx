-- zmxx.nvim - fzf-lua picker for zmxx (git worktree + zmx) sessions.
--
-- The picker lists every zmx session that carries the `zmxx=1` label.
-- Selecting one runs `zmxx switch <name>`, which asks zmx to switch the
-- terminal's client to the target session; the neovim instance itself stays
-- running inside its original session. Outside zmx, a terminal tab hosts the
-- client instead.
local M = {}

local function notify(msg, level)
  vim.schedule(function()
    vim.notify("[zmxx] " .. msg, level or vim.log.levels.INFO)
  end)
end

---@return zmxx.Session[] decoded from `zmxx ls --json`
local function list_sessions()
  local job = vim.system({ "zmxx", "ls", "--json" }, { text = true })
  local result = job:wait(10000)
  if result.code ~= 0 then
    notify((result.stderr or "failed to list sessions"):gsub("%s+$", ""), vim.log.levels.ERROR)
    return {}
  end
  local ok, data = pcall(vim.json.decode, result.stdout)
  if not ok or type(data) ~= "table" then
    notify("failed to parse `zmxx ls --json` output", vim.log.levels.ERROR)
    return {}
  end
  return data
end

--- Switch sessions inside zmx, or attach in a terminal tab outside zmx.
---@param name string session name
function M.switch(name)
  if not name or name == "" then
    return
  end
  local current = vim.env.ZMX_SESSION
  if name == current then
    notify("already attached to session " .. name)
    return
  end
  if not current or current == "" then
    -- vim.system has no interactive terminal. A fresh client needs a PTY
    -- and visible input/output, unlike the Switch IPC used inside zmx.
    vim.cmd("tabnew")
    local buf = vim.api.nvim_get_current_buf()
    vim.bo[buf].bufhidden = "wipe"
    local job = vim.fn.termopen({ "zmxx", "switch", name }, {
      on_exit = function(_, code)
        vim.schedule(function()
          if code ~= 0 then
            -- Keep the terminal's diagnostics visible on failure.
            notify("failed to attach to session " .. name .. " (exit " .. code .. ")", vim.log.levels.ERROR)
            return
          end
          if vim.api.nvim_buf_is_valid(buf) then
            vim.api.nvim_buf_delete(buf, { force = true })
          end
        end)
      end,
    })
    if job <= 0 then
      notify("failed to start `zmxx switch`", vim.log.levels.ERROR)
      return
    end
    vim.cmd("startinsert")
    return
  end
  vim.system({ "zmxx", "switch", name }, { text = true }, function(result)
    if result.code ~= 0 then
      notify((result.stderr or "failed to switch session"):gsub("%s+$", ""), vim.log.levels.ERROR)
    end
  end)
end

--- Open the fzf-lua picker over all zmxx sessions.
---@param opts? table optional fzf-lua options override
function M.sessions(opts)
  opts = opts or {}
  local ok, fzf = pcall(require, "fzf-lua")
  if not ok then
    notify("fzf-lua is required for the session picker", vim.log.levels.ERROR)
    return
  end

  local sessions = list_sessions()
  if #sessions == 0 then
    notify("no zmxx sessions running (start one with `zmxx new <branch>`)")
    return
  end

  local entries = {}
  for _, s in ipairs(sessions) do
    local mark = s.current and "\u{2192}" or " "
    local clients = s.clients or "0"
    local branch = s.branch or "?"
    local wt = s.worktree or "?"
    local reponame = s.reponame or s.repo or "?"
    -- Field 1 (session name) is hidden from matching/display but kept for
    -- the enter action and the {1} preview placeholder.
    entries[#entries + 1] = string.format(
      "%s\t%s\t%s\t%s\t%s\t%s",
      s.name,
      mark,
      reponame,
      branch,
      wt,
      clients
    )
  end

  fzf.fzf_exec(entries, vim.tbl_deep_extend("force", {
    prompt = "zmxx> ",
    fzf_opts = {
      ["--delimiter"] = "[\t]",
      ["--with-nth"] = "2..",
      ["--nth"] = "2..",
      ["--tiebreak"] = "index",
      ["--preview-window"] = "right:60%:follow",
    },
    preview = "zmxx preview {1}",
    actions = {
      ["enter"] = function(selected)
        local line = selected and selected[1]
        local name = line and line:match("^([^\t]+)")
        if name then
          M.switch(name)
        end
      end,
    },
  }, opts))
end

return M
