-- Run with: nvim --headless -u NONE -l tests/picker.lua
-- Requires zmx. All sessions use a private socket directory.
vim.opt.runtimepath:prepend(vim.fn.getcwd())
local dir = vim.fn.tempname()
vim.fn.mkdir(dir, "p")
vim.env.ZMX_DIR = dir
vim.env.ZMX_SESSION = nil
vim.env.ZMX_SESSION_PREFIX = nil

local function zmx(args)
  local cmd = { "zmx" }
  vim.list_extend(cmd, args)
  local result = vim.system(cmd, { text = true, stdin = "" }):wait(10000)
  assert(result.code == 0, result.stderr)
  return result.stdout
end

local function wait_for(predicate, message)
  assert(vim.wait(5000, predicate, 20), message)
end

local function clients(name)
  local output = zmx({ "list" })
  for line in output:gmatch("[^\n]+") do
    if line:find("name=" .. name .. "\t", 1, true) then
      return tonumber(line:match("clients=(%d+)"))
    end
  end
end

local ok, err = xpcall(function()
  for _, name in ipairs({ "a", "b" }) do
    zmx({ "attach", "--labels", "zmxx=1", name, "sleep", "120" })
  end

  -- Drive the picker's Enter callback using the actual session list.
  local entries, opts
  package.loaded["fzf-lua"] = {
    fzf_exec = function(e, o)
      entries, opts = e, o
    end,
  }
  local picker = require("zmxx")
  picker.sessions()
  assert(#entries == 2, "expected both test sessions in picker")
  local original_buf = vim.api.nvim_get_current_buf()
  local original_tab = vim.api.nvim_get_current_tabpage()
  opts.actions.enter({ entries[1] })
  local terminal_buf = vim.api.nvim_get_current_buf()
  local channel = vim.bo[terminal_buf].channel
  assert(vim.bo[terminal_buf].buftype == "terminal", "attach needs a terminal")
  assert(vim.api.nvim_get_current_tabpage() ~= original_tab, "attach needs its own tab")
  wait_for(function() return clients("a") == 1 end, "client did not attach")

  -- Inside zmx the existing client must move, without opening another tab.
  vim.env.ZMX_SESSION = "a"
  local tabs = #vim.api.nvim_list_tabpages()
  picker.switch("b")
  wait_for(function() return clients("a") == 0 and clients("b") == 1 end, "client did not switch")
  assert(#vim.api.nvim_list_tabpages() == tabs, "switch opened an extra tab")
  vim.env.ZMX_SESSION = nil

  -- Detaching closes only the client tab and preserves both sessions.
  vim.api.nvim_chan_send(channel, string.char(28))
  wait_for(function() return not vim.api.nvim_buf_is_valid(terminal_buf) end, "terminal did not close")
  assert(vim.api.nvim_get_current_tabpage() == original_tab, "original tab was not restored")
  assert(vim.api.nvim_get_current_buf() == original_buf, "original buffer was not restored")
  assert(clients("a") == 0 and clients("b") == 0, "sessions must survive detaching")
end, debug.traceback)

vim.env.ZMX_SESSION = nil
for _, name in ipairs({ "a", "b" }) do
  pcall(zmx, { "kill", name })
end
vim.fn.delete(dir, "rf")
if not ok then
  io.stderr:write(err .. "\n")
  vim.cmd("cquit 1")
end
print("picker attach/switch/detach: OK")
vim.cmd("qa!")
