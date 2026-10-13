# One password, one screen

## What the user asked for

> "Quiero quitar toda la parte de multitrigger de password de Linux cuando estás instalado. Como
> mucho, si se necesita, que se pida una sola vez, y que no haya que darle al enter para continuar
> si es posible en mitad de la instalación también, y que no se cierre el installer, para volver a
> la terminal mientras que se están instalando cosas, que se mantenga en pantalla el installer, y
> no haya bloqueos de que le tengo que dar enter to continue."
>
> And separately: *"el mensaje de aceptar la instalación de la fuente no queda claro lo que hay
> que hacer."*

Four complaints, one cause. Measured, not inferred.

## The cause

The installer hands the terminal to a shell script whenever a step needs `sudo`, because `sudo`
wants a terminal to ask its question on.

```go
// installer/internal/tui/interactive.go
// runInteractiveStep creates a tea.Cmd that runs an interactive step
// This suspends the TUI and gives full terminal control to the process
```

That single decision produces all three of the first complaints, and the code says so in its own
comment: the TUI is **suspended**, so the installer leaves the screen; each script runs `sudo`
itself and so **asks again**; and because the terminal was handed over, the script prints a prompt
and waits so the handback is readable, which is the **enter to continue**.

## Evidence

| finding | where |
|---|---|
| no `sudo -v`, `sudo -n` or `SUDO_ASKPASS` anywhere in the repository | measured: the search returns nothing |
| at least twelve separate `sudo` invocations in one file | `interactive.go:156, 197, 199, 203, 224, 225, 253, 255, 256, 342, 353, 496, 509` |
| five steps hand the terminal over | four `Interactive: true` (`model.go:1140, 1176, 1273, 1294`) plus `Interactive: opts.OS == "linux"` (`model.go:1187`) |
| the prompt is ours, not `sudo`'s | a constant of our own, `interactiveContinuePrompt` (`interactive.go:15`), and **eight** occurrences of the same sentence (`interactive.go:15, 174, 289, 330, 364, 382, 411`, `cmd/dotfiles/main.go:312`) |
| the offer's second option names a reason, not an action | `model.go:663`: `"No, I already have it"` |

## The design

**Ask once, then keep the screen.**

1. **One credential, at the start.** `sudo -v` before the first step that needs it refreshes the
   cached credential once, and the run keeps it warm while it works so the cache cannot expire in
   the middle of a long install. Every script then runs `sudo -n` and either succeeds or fails
   immediately, instead of prompting at a point the user cannot see.
2. **The steps stop owning the terminal.** With the credential cached, a step's script needs no
   TTY, so it runs through the shared executor that already captures output line by line and puts
   it in the activity log. That is the machinery `SendLog` and `RunWithLogs` already provide; the
   interactive route stops being the only way to run a command that needs root.
3. **What genuinely needs a terminal keeps it, and says so.** A step that cannot work without a
   TTY is the exception and must name that fact, rather than the current arrangement where five
   steps are interactive because the mechanism requires it.
4. **The pauses go.** They exist only to make a handback readable. With no handback there is
   nothing to wait for.

The sequencing matters: 1 enables 2, and 2 removes the need for 4. Asking once is not a
convenience, it is what makes staying on the screen possible.

## The offer's message

`"No, I already have it"` answers a question the user was not asked. The question is *"install the
font?"*, and the option is a **reason** rather than an action: it does not say whether choosing it
skips the font, fails the step, or continues. The fix is that each option says what it does, and
that the row states what the installer found about this machine so the choice is informed by a
measurement rather than a memory.

## Tasks

- [ ] **T1 — Ask once.** A cached credential before the first step that needs it, a keep-alive
  while the install runs, and `sudo -n` in every script so a step that needs the password it does
  not have fails visibly instead of prompting where the user cannot see it. The check that asks
  is itself visible in the TUI.
- [ ] **T2 — The steps keep the screen.** The steps that hand the terminal over stop doing so,
  running through the captured-output executor instead, so the installer stays on screen for the
  whole run. Any step that genuinely cannot work without a TTY is named as such in code and in the
  report, with the reason.
- [ ] **T3 — The pauses go.** Every occurrence of the enter-to-continue prompt that exists only
  because a script owned the terminal is removed, and a guard refuses to let one come back into a
  step that no longer needs it.
- [ ] **T4 — The offer says what its options do.** Both labels name the action, and the screen
  states what was found about the font on this machine.
- [ ] **T5 — The local proof.** The whole install path run on a Linux host, showing one password
  prompt in total, no enter prompt, and the TUI on screen throughout.
