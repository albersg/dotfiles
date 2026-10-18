# gai-preference — the installer prefers IPv4, because the download paths need it

Branch `feat/gai-preference`, base `origin/main` = `b15f25d`, worktree
`/home/albeertoo/my-things/dotfiles-gai`. The request is issue #361, written by the
owner, and this front is the installer reproducing by itself the change he made by hand.

## Specs

- **S1.** The owner's words: *"Configurar `/etc/gai.conf` durante la instalación, para que
  la máquina prefiera IPv4 sobre IPv6."* It is a change he already made by hand on his
  machine and wants the installer to reproduce.
- **S2.** The lines his `/etc/gai.conf` holds, with their numbers, quoted verbatim:

  ```
   68:precedence ::1/128 50
   69:precedence ::/0 40
   70:precedence
   71: 2002::/16 30
   72:precedence ::/96 20
   73:precedence ::ffff:0:0/96 100
  ```

  And the owner's own reading of them: *"La última, `precedence ::ffff:0:0/96 100`, es la
  que decide: sube las direcciones IPv4 mapeadas por encima de todo lo demás, y es la
  forma documentada de que `getaddrinfo(3)` devuelva IPv4 primero. Comprobado en la
  máquina: `getent ahosts github.com` sólo devuelve direcciones IPv4."*
- **S3.** The file belongs to a package, and that is the central decision: *"`dpkg -S
  /etc/gai.conf` dice `libc-bin`"*, so *"un `apt upgrade` de `libc-bin` lo trata como
  conffile: si el paquete trae una versión nueva y el fichero local cambió, dpkg pregunta,
  y una pregunta en mitad de un upgrade desatendido lo bloquea"*, and *"si el instalador lo
  reescribe y luego el paquete lo actualiza, el cambio se pierde"*. The task list fixes what
  to do about it: *"comprobar si el fichero está gestionado (`dpkg -S`) y decir en el log
  qué pasará en el próximo upgrade, en vez de dejarlo como sorpresa"*.
- **S4.** Placement: *"Lo natural es un paso propio antes de `deps`"*, because *"el motivo de
  preferir IPv4 es que las descargas y los gestores de paquetes dejan de caer en rutas IPv6
  rotas, y quien se beneficia de eso es exactamente `deps`."*
- **S5.** Platforms: Linux and WSL. The owner's reason: *"En WSL el motivo (rutas IPv6 rotas
  hacia servicios que sólo tienen A) es más fuerte que en Linux nativo, y el paso de WSL ya
  escribe en `/etc`."*
- **S6.** What is written: *"La regla del repositorio es no destruir configuración del
  usuario, así que lo mínimo es añadir o reemplazar un bloque marcado al final, nunca
  reescribir el fichero entero."* And the warning that makes any uncommented line
  suspicious: *"El fichero por defecto es todo comentario: 65 de sus 73 líneas lo son. Eso
  significa que cualquier línea sin `#` que aparezca ahí es de alguien, y merece una segunda
  mirada antes de tocarla."*
- **S7.** A user who already set his own precedence: *"Ahí la respuesta honesta es no tocar
  nada y decirlo, en vez de imponer un orden que él eligió al revés."*
- **S8.** Idempotency and the way back: *"Correr el instalador diez veces tiene que dejar el
  mismo fichero, y tiene que haber una forma de volver atrás — el respaldo que ya hace
  `writeWithSudo` permite ofrecerlo."*
- **S9.** The proof in a container: *"`gai.conf` de fábrica → instalar → `getent ahosts`
  devuelve IPv4 primero; y con una precedencia ajena puesta → no se toca."*
- **S10.** Documentation: *"el manual y `docs/tools.md` dicen qué hace el paso, qué no puede
  prometer sobre un futuro upgrade de `libc-bin`, y cómo se deshace."*
- **S11.** The mechanism already exists and must be reused, not rebuilt: *"el paso de WSL
  escribe `/etc/wsl.conf` con `applyArtifact` + `writeWithSudo`, con respaldo del fichero
  anterior y verificación de lo escrito (`installer/internal/tui/wsl.go`,
  `defaultWSLConfPath` y el override `DOTFILES_WSL_CONF_PATH`)"*.

## Decisions the six open questions resolve to

1. **Own step**, `ID: "gai"`, placed immediately before `deps` (S4). A step is visible in the
   plan and its own row can be described; folding it into `deps` would hide it.
2. **Linux and WSL** (S5). Both are Linux; the WSL case is the stronger one.
3. **A marked block appended at the end** of the file, replaced in place when the marker is
   already there, and the rest of the file byte-identical (S6).
4. **A precedence the user set is never overridden** (S7): the step writes nothing, says what
   it found, and does not fail.
5. **The package question is reported, not fought** (S3): `dpkg -S` is asked when the tool is
   there; the log says the file belongs to `libc-bin` and what an upgrade may do to it. The
   step never touches dpkg's state.
6. **Idempotent, with the existing writer's backup** (S8, S11): a second run leaves the file
   identical, and the backup path the writer logs is the way back.

## Tasks

- [ ] **T1** — this document states decisions 1-6 before the first line of code (S1-S11).
- [ ] **T2** — the step: `installer/internal/tui/gai.go` with the block builder, the
      "foreign precedence" reader, the `dpkg -S` report and the writer call (S2, S3, S6, S7,
      S11). Route: `gentle-ai-worker`.
- [ ] **T3** — the plan carries it: a `gai` step before `deps` on Linux and WSL only, and
      its executor entry (S4, S5).
- [ ] **T4** — the guard tests: idempotency (twice = same bytes), a foreign precedence is
      left alone, the block replaces in place, and a factory file keeps its other 65
      comment lines (S6, S7, S8).
- [ ] **T5** — the container proof of S9 where the repository's e2e suite runs, or a Go test
      with the path override when it cannot run locally, with the exact command recorded.
- [ ] **T6** — `docs/tui-installer.md` and `docs/tools.md` say what the step does, what it
      cannot promise about a `libc-bin` upgrade, and how to undo it (S10).
- [ ] **T7** — verified, committed as work units, PR opened and merged.

## Log

- **L1.** The owner's request, verbatim: *"Configurar `/etc/gai.conf` durante la instalación,
  para que la máquina prefiera IPv4 sobre IPv6. Es un cambio que el mantenedor ya hizo a
  mano en su equipo y quiere que el instalador lo reproduzca."* (issue #361)
- **L2.** The session's request, verbatim: *"revisa todo el trabajo pendiente ... para
  solventarlo"* and *"todo debe acabar en main"*.
- **L3.** The worker's own note, kept rather than papered over: RED was **not** observed for
  the step's decision logic. `gai_test.go` names symbols that `gai.go` introduces, so before
  the implementation the package did not compile and the pre-implementation observation was a
  build error, not a failing assertion. The RED that *was* observed is the plan half: with
  `gai` scheduled first, the package suite failed four assertions in `panels_test.go`
  (`"Steps 9"`, `"Install Dependencies"`) until the expectations were extended. The decision
  logic is therefore covered by green tests whose red state could not be isolated, and the
  independent verification is asked to check that those tests pin behaviour rather than the
  implementation's own shape.
- **L4.** `Docker` is not installed in this environment, so the S9 container proof in
  `installer/e2e/e2e_test.sh` is written and unrun here. The maintainer command is
  `cd installer && ./e2e/docker-test.sh e2e ubuntu`; it is never reported as passing.
