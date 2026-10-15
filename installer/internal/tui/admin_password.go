package tui

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// adminPasswordCapacity is how many bytes the field holds, and it is fixed on
// purpose. Update returns the model by value, so every keystroke copies the slice
// header that points at the bytes typed so far. One backing array, shared by all
// those copies, is the only arrangement in which clearing the bytes once clears
// every copy of them; a buffer allowed to grow would reallocate, and the array
// that held the password would become unreachable and unclearable.
const adminPasswordCapacity = 256

// sudoNeedsPasswordMsg reports that sudo has no credential it can use without a
// password, so the installer has to ask for one. It is the probe's failure, and
// what puts the installer's own screen up instead of a terminal handover.
type sudoNeedsPasswordMsg struct{}

// sudoPasswordPrimer is the seam for the attempt made from the installer's own
// screen. It is a variable so a test can prove where the password travels without
// invoking sudo.
var sudoPasswordPrimer = defaultSudoPasswordPrimer

// defaultSudoPasswordPrimer hands sudo the password on the child's standard
// input. `-S` is what tells sudo to read it there rather than open the terminal,
// and that is the whole reason this prompt can live inside the interface: no
// terminal is needed, so none is given away.
//
// The bytes are cleared as soon as sudo has read them, in this command and before
// its result is returned, so the array the model points at holds zeros by the time
// the run continues. The clearing is deferred rather than done by the caller
// because bytes.Reader does not copy: clearing before sudo read would send it
// zeros.
func defaultSudoPasswordPrimer(password []byte) tea.Cmd {
	return func() tea.Msg {
		defer zeroPasswordBytes(password)

		c := exec.Command("sudo", "-S", "-v")
		c.Stdin = bytes.NewReader(password)

		var stderr bytes.Buffer
		c.Stderr = &stderr
		if err := c.Run(); err != nil {
			return sudoPrimedMsg{err: errors.New(sudoPasswordFailure(stderr.String()))}
		}
		return sudoPrimedMsg{}
	}
}

// sudoPasswordFailure turns sudo's refusal into the one line the screen shows. It
// reads sudo's stderr for the reason and deliberately does not pass it through:
// that text names the invoking account and the host, and the line the user needs
// says only what happened. Nothing here ever carries the password.
func sudoPasswordFailure(stderr string) string {
	lowered := strings.ToLower(stderr)
	switch {
	case strings.Contains(lowered, "incorrect password"), strings.Contains(lowered, "try again"):
		return "that password was not accepted"
	case strings.Contains(lowered, "not in the sudoers"), strings.Contains(lowered, "is not allowed"):
		return "this account is not allowed to use sudo on this machine"
	default:
		return "the administrator credential could not be validated"
	}
}

// zeroPasswordBytes overwrites every byte of the buffer. It is the only way the
// password leaves this program: it is not kept, not logged and not returned.
func zeroPasswordBytes(buf []byte) {
	for i := range buf {
		buf[i] = 0
	}
}

// renderAdminPassword draws the installer's own password prompt. It shows how
// much has been typed and never a character of it.
func (m Model) renderAdminPassword() string {
	hints := []installerHint{hintSelect, hintBack}

	typed := len(m.AdminPassword)
	masked := strings.Repeat("•", typed)
	if typed == 0 {
		masked = MutedStyle.Render("waiting")
	}

	body := []string{
		BrandStyle.Render(m.GetScreenTitle()),
		MutedStyle.Render("sudo needs it for the steps that change the machine."),
		"",
		HighlightStyle.Render("  > ") + masked,
		"",
		MutedStyle.Render(fmt.Sprintf("  %d characters typed; the field holds up to %d.", typed, adminPasswordCapacity)),
	}
	if m.AdminPasswordErr != "" {
		body = append(body,
			"",
			ErrorStyle.Render("  ✗ "+m.AdminPasswordErr),
			MutedStyle.Render("  Press Esc to be asked in the terminal instead."),
		)
	}
	body = append(body,
		"",
		MutedStyle.Render("  It is read here, on this screen: the installer does not leave its own"),
		MutedStyle.Render("  interface to ask for it. Nothing about it is written down."),
	)

	return m.frame(m.headerName(), "", body, hints)
}

// updateAdminPassword is the installer's password screen. It takes runes and
// backspace into the field, Enter to make the attempt, and Escape to fall back to
// the terminal handover for a system the in-screen prompt cannot cover.
func (m Model) updateAdminPassword(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyRunes:
		for _, r := range key.Runes {
			encoded := []byte(string(r))
			// The capacity check is what keeps append from reallocating: with the
			// fixed capacity never exceeded, the one backing array is the only one
			// that ever holds a byte of the password.
			if len(m.AdminPassword)+len(encoded) > cap(m.AdminPassword) {
				break
			}
			m.AdminPassword = append(m.AdminPassword, encoded...)
		}
		return m, nil

	case tea.KeyBackspace:
		if len(m.AdminPassword) > 0 {
			m.AdminPassword = m.AdminPassword[:len(m.AdminPassword)-1]
		}
		return m, nil

	case tea.KeyEnter:
		if len(m.AdminPassword) == 0 {
			return m, nil
		}
		m.AdminPasswordErr = ""
		m.LogLines = append(m.LogLines, "Administrator access: validating the password typed on the installer's screen.")
		return m, sudoPasswordPrimer(m.AdminPassword)

	case tea.KeyEsc:
		// S3's way out. A system whose PAM wants a terminal -- a two-factor or
		// fingerprint module, or a legacy `Defaults requiretty` -- cannot be
		// authenticated through `sudo -S` at all, so the handover the installer used
		// for every request stays reachable here rather than being deleted.
		zeroPasswordBytes(m.AdminPassword)
		m.AdminPassword = m.AdminPassword[:0]
		m.AdminPasswordErr = ""
		m.Screen = ScreenInstalling
		m.LogLines = append(m.LogLines, "Administrator access: asking in the terminal instead, for a system that needs one there.")
		return m, sudoTerminalPrimer()
	}

	return m, nil
}
