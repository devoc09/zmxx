-- zmxx - neovim integration: switch between zmx workspaces from a picker.
if vim.g.loaded_zmxx then
  return
end
vim.g.loaded_zmxx = 1

vim.api.nvim_create_user_command("ZmxxSessions", function()
  require("zmxx").sessions()
end, {
  desc = "Pick a zmxx workspace session to switch to",
})

vim.keymap.set("n", "Zl", function()
  require("zmxx").sessions()
end, { desc = "Zmxx: pick session", silent = true })
