# Installer Redesign — Review Summary

Review artifact for the interface redesign delivered on `main` through the
`feat/installer-redesign` line. The code is already in `main`; this document
records what was reviewed, what was fixed and what remains deliberately open.

## Delivered

- One layout grammar across the installer and the trainer: header chip + status
  bar, card with chip and caption, rows with icon/title/subtitle/chevron, filled
  selection bar, fact rows, one footer notation.
- All installer screens converted: wizard questions, dashboard, setup, utilities,
  update center, doctor/disk/network, Learn/keymaps, backup/restore/complete/error,
  ports, SSH, WSL, terminal capabilities, install verify, plus the trainer's six
  screens with its pixel-art identity kept.
- The theme system is the only colour source; no hardcoded palette.
- The reported defects fixed: body vertically centred with the pet reserved, the
  update-center list scrolling inside its card with a truthful window and a
  proportional scrollbar, the section "Details" cards capped at their tallest
  reading, the shell-startup clock and its measure button, and the desktop-theme
  aviso removed from the utilities screen.

## Review

Five chained review lineages, all closed:

| Task | Lineage | Outcome |
| --- | --- | --- |
| T6 utilities | `review-a936b05a007a4b9d` | approved, 9 informational findings |
| T7 theme selector | `review-0ce2625867193a4f` | approved, 3 informational findings |
| T9 remaining pages | `review-a1147e36ffc35a3a` | approved, 4 informational findings |
| T1 theme audit | `review-8a362c50318e733e` | approved after a bounded correction |
| T4 dashboard | escalated earlier | informational, left as recorded |

T1's refuter found a critical defect in the new theme-switch guard: it demanded
that the applied palette's blue never appear in a plain frame, but the theme
picker draws the shipped definitions' swatches on purpose. The bounded
correction honours `surface.definitions` (5 diff lines), the provider's targeted
validation approved it, and the fix is in `main`.

The informational findings were triaged: nine fixed as comment/test-clarity
work (one golden moved by the separator fix it named), six left with reasons
(behavioural or harness-wide), three not applicable at HEAD.

## Verification

- `go build ./...`, `go vet`, `gofmt` clean on the touched packages.
- Focused guards for every change; golden snapshots at 80x24 and the wide sizes.
- VHS captures 01–06 at 200x55 and 80x24 relaid out for the current screens.
- The two long-standing red tests (animation shift, shell-audit card cut) were
  closed: the animated frame reserves exactly what the still frame reserves, and
  the methodology card declares any cut it makes.

## Deliberately open

- T7's activity slot sizes to zero on very short frames; T9's `referenceMaxScroll`
  can return one past the end. Both are behavioural and need their own guards.
- The alternate-theme VHS capture needs a real HOME; the isolated harness cannot
  detect the theme switch, so the step was removed from both tapes.
- `docs/vim-trainer-spec.md` still uses emoji in section headings; it is
  documentation, not a renderer.
