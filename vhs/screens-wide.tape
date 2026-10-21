# The installer's own screens, captured from a real terminal at the width the
# interface is reviewed at (200x55). `scripts/screens.sh` builds the binary and
# runs this from the repository root, so every path here is relative to it.
#
# The journey uses only keys that are stable on every host: `s` from the welcome
# opens the menu, `u` opens the Utilities section. Deeper screens need host state
# (managers, backups) and are captured from the deterministic frame fixtures
# instead -- see vhs/README.md.
Set Shell bash
Set FontFamily "MesloLGM Nerd Font Mono"
Set FontSize 18
Set Columns 200
Set Rows 55
Set Padding 12
Set Theme "Catppuccin Mocha"
Set TypingSpeed 10ms

Env COLORTERM "truecolor"

Type "./installer/dotfiles"
Enter
Wait+Screen /Your personal development environment/
Sleep 700ms
Screenshot "captures/200x55/01-welcome.png"

Type "s"
Wait+Screen /Start Installation/
Sleep 700ms
Screenshot "captures/200x55/02-main-menu.png"

Enter
Wait+Screen /Configure your environment/
Sleep 700ms
Screenshot "captures/200x55/03-setup.png"

Escape
Wait+Screen /Start Installation/

Type "u"
Wait+Screen /Clean Dev Caches/
Sleep 700ms
Screenshot "captures/200x55/04-utilities.png"

Enter
Wait+Screen /Choose a theme for your installer/
Sleep 700ms
Screenshot "captures/200x55/05-theme.png"

Escape
Wait+Screen /Clean Dev Caches/
