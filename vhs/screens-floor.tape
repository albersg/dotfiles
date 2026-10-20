# The installer's own screens, captured from a real terminal at the width the
# interface is reviewed at (80x24). `scripts/screens.sh` builds the binary and
# runs this from the repository root, so every path here is relative to it.
#
# The journey uses only keys that are stable on every host: `s` from the welcome
# opens the menu, `u` opens the Utilities section. Deeper screens need host state
# (managers, backups) and are captured from the deterministic frame fixtures
# instead -- see vhs/README.md.
Output "captures/80x24/session.gif"
Set Shell bash
Set FontFamily "DejaVu Sans Mono"
Set FontSize 14
Set Columns 80
Set Rows 24
Set Padding 12
Set Theme "Catppuccin Mocha"
Set TypingSpeed 10ms

Env COLORTERM "truecolor"

Type "./installer/dotfiles"
Enter
Wait+Screen /Your terminal environment/
Sleep 700ms
Screenshot "captures/80x24/01-welcome.png"

Type "s"
Wait+Screen /What would you like to do/
Sleep 700ms
Screenshot "captures/80x24/02-main-menu.png"

Type "u"
Wait+Screen /Clean up development caches/
Sleep 700ms
Screenshot "captures/80x24/03-utilities.png"
