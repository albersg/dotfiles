# The installer's own screens, captured from a real terminal at the width the
# interface is reviewed at (80x24). `scripts/screens.sh` builds the binary and
# runs this from the repository root, so every path here is relative to it.
#
# The journey uses only keys that are stable on every host: `s` from the welcome
# opens the menu, `u` opens the Utilities section. Deeper screens need host state
# (managers, backups) and are captured from the deterministic frame fixtures
# instead -- see vhs/README.md.
Set Shell bash
Set FontFamily "MesloLGM Nerd Font Mono"
Set FontSize 14
Set Columns 80
Set Rows 24
Set Padding 12
Set Theme "Catppuccin Mocha"
Set TypingSpeed 10ms

Env COLORTERM "truecolor"

Type "./installer/dotfiles"
Enter
Wait+Screen /Your personal development environment/
Sleep 700ms
Screenshot "captures/80x24/01-welcome.png"

Type "s"
Wait+Screen /What would you like to do/
Sleep 700ms
Screenshot "captures/80x24/02-main-menu.png"

Enter
Wait+Screen /Configure your environment/
Sleep 700ms
Screenshot "captures/80x24/03-setup.png"

Escape
Wait+Screen /What would you like to do/

Type "u"
Wait+Screen /Clean Dev Caches/
Sleep 700ms
Screenshot "captures/80x24/04-utilities.png"

Enter
Wait+Screen /Choose a theme for your installer/
Sleep 700ms
Screenshot "captures/80x24/05-theme.png"

Escape
Wait+Screen /Clean Dev Caches/
