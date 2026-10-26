package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

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

// credentialAskFlow names the flow that put the password screen up, so the answer
// resumes the flow that asked and not another one: the installation's step loop
// and the update centre's apply both ask, and each has its own way forward.
type credentialAskFlow int

const (
	// credentialAskNone is a screen nobody asked for: an answer that arrives with
	// no flow behind it resumes nothing.
	credentialAskNone credentialAskFlow = iota
	// credentialAskStepLoop is the installation's step loop, waiting before the
	// first step that needs root.
	credentialAskStepLoop
	// credentialAskUpdateCenter is the update centre's apply, waiting before its
	// first command that has to be the administrator's.
	credentialAskUpdateCenter
)

// sudoPasswordPrimer is the seam for the attempt made from the installer's own
// screen. It is a variable so a test can prove where the password travels without
// invoking sudo.
var sudoPasswordPrimer = defaultSudoPasswordPrimer

// sudoPasswordTimeout bounds one attempt made from the installer's own screen.
//
// The password is typed on that screen, not handed to sudo to prompt for, so the
// only thing sudo has left to do is hash what it was given and check it: a second
// is already generous. The bound exists for the cases where sudo never gets that
// far -- a sudo configured to read its prompt from a terminal it does not have, a
// sudo that ignores standard input while it waits for one, a machine whose PAM is
// not answering -- and it is what turns each of them into a message on the screen
// instead of a run that waits for ever. It is a variable so a guard can lower it
// and prove the release rather than wait for it.
var sudoPasswordTimeout = 30 * time.Second

// sudoPasswordTimeoutFailure is the line a stopped attempt leaves. It says what
// happened and who will report it, and it never carries sudo's stderr.
const sudoPasswordTimeoutFailure = "sudo did not answer in time, so the attempt was stopped"

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
//
// The wait is bounded by sudoPasswordTimeout, and the bound is enforced by killing
// the child rather than by abandoning it: a sudo left running behind a released
// screen would still be reading the bytes this command has already cleared.
func defaultSudoPasswordPrimer(password []byte, attempt int) tea.Cmd {
	return func() tea.Msg {
		defer zeroPasswordBytes(password)

		ctx, cancel := context.WithTimeout(context.Background(), sudoPasswordTimeout)
		defer cancel()

		c := exec.CommandContext(ctx, "sudo", "-S", "-v")
		c.Stdin = bytes.NewReader(password)

		var stderr bytes.Buffer
		c.Stderr = &stderr
		if err := c.Run(); err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return sudoPrimedMsg{attempt: attempt, err: errors.New(sudoPasswordTimeoutFailure)}
			}
			return sudoPrimedMsg{attempt: attempt, err: errors.New(sudoPasswordFailure(stderr.String()))}
		}
		return sudoPrimedMsg{attempt: attempt}
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
	case strings.Contains(lowered, "no tty"), strings.Contains(lowered, "askpass"),
		strings.Contains(lowered, "must have a terminal"), strings.Contains(lowered, "terminal is required"):
		return "this machine will not take the password without a terminal, so sudo refused it"
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

// askAdminPassword puts the installer's own password screen up for flow, on a
// field that starts empty and of the fixed capacity. The flow is recorded so the
// answer can resume it rather than guessing from the screen that happens to be up.
func (m *Model) askAdminPassword(flow credentialAskFlow) {
	m.AdminPassword = make([]byte, 0, adminPasswordCapacity)
	m.AdminPasswordPending = false
	m.CredentialAsk = flow
	m.Screen = ScreenAdminPassword
	m.LogLines = append(m.LogLines, "Administrator access: type the password on this screen; the installer does not leave it to ask.")
}

// clearAdminPassword overwrites whatever the field holds and empties it. It runs
// on every way out of the screen, so no copy of the model keeps a byte of the
// password after the attempt that read it.
func (m *Model) clearAdminPassword() {
	zeroPasswordBytes(m.AdminPassword)
	m.AdminPassword = m.AdminPassword[:0]
}

// answerAdminPassword is the reply to an attempt made on the installer's own
// screen, and it releases the screen whichever way the attempt went.
//
// A refusal is not a reason to wait for a second one: the reason is written where
// the run reports things, the screen goes, and the run continues so the command
// that needed the credential reports it in the manager's own words -- the same
// report a run that was never asked gave, and the one the user can act on. A
// success caches the credential and resumes the flow that asked.
func (m Model) answerAdminPassword(msg sudoPrimedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.LogLines = append(m.LogLines, "⚠️  Administrator access: "+msg.err.Error()+". A step that needs root will report it.")
		cmd := m.releaseAdminPassword(false)
		return m, cmd
	}
	m.LogLines = append(m.LogLines, "✓ Administrator credential cached for this run.")
	cmd := m.releaseAdminPassword(true)
	return m, cmd
}

// cancelAdminPassword is Escape and ctrl+c on the password screen: the run stops
// waiting, says why, and carries on. It is the same release a refusal takes, which
// is what makes "the screen is never the last word" true for every key.
func (m Model) cancelAdminPassword() (tea.Model, tea.Cmd) {
	reason := "the administrator password was not given"
	if m.AdminPasswordPending {
		reason += ", and the attempt was cancelled before sudo answered"
	}
	m.LogLines = append(m.LogLines, "⚠️  Administrator access: "+reason+". A step that needs root will report it.")
	cmd := m.releaseAdminPassword(false)
	return m, cmd
}

// releaseAdminPassword takes the password screen down and resumes the flow that
// put it up: the installation's step loop, or the update centre's apply.
//
// It is the only way off the screen, and it is the same exit whether the
// credential was typed, refused or cancelled. The field is cleared first, so what
// the model keeps afterwards is an empty buffer of zeros; the run keeps its one
// attempt, so a refused or cancelled request is not asked again; and the flow is
// resumed so its own commands report the missing credential through `sudo -n`
// instead of the installer waiting for a password it does not have.
func (m *Model) releaseAdminPassword(primed bool) tea.Cmd {
	flow := m.CredentialAsk
	m.CredentialAsk = credentialAskNone
	m.AdminPasswordPending = false
	m.clearAdminPassword()
	m.SudoPrimed = primed
	if !primed {
		m.SudoPrimeAttempted = true
	}

	switch flow {
	case credentialAskUpdateCenter:
		m.Screen = ScreenUpdateCenter
		if !primed {
			// The screen has no log pane of its own, so the reason is said on the
			// notice line the run already uses: the report of what each command did
			// follows when the sequence reaches it.
			m.UpdateCenter.Notice = "The administrator password was not given. Each command that needs it will report it."
		}
		// A stopped run stays stopped: an answer that lands after the user stopped
		// the sequence must not start it again.
		if !m.UpdateCenter.Applying {
			return nil
		}
		return m.updateCenterBeginApply()
	case credentialAskStepLoop:
		m.Screen = ScreenInstalling
		if primed {
			return tea.Batch(m.runNextStep(), sudoKeepAliveTickCmd())
		}
		return m.runNextStep()
	default:
		// Nobody asked, so nothing is resumed.
		return nil
	}
}

// adminPasswordMaskLimit is how many bullets the field draws before it stops
// counting them out. The field holds up to adminPasswordCapacity bytes, and one
// bullet per byte would run past any terminal long before that; the count on the
// line beneath is the precise answer, so the mask does not have to be.
const adminPasswordMaskLimit = 24

// renderAdminPassword draws the installer's own password prompt. It shows how
// much has been typed and never a character of it, and it says what to do with
// the keyboard rather than leaving it to be guessed at: the field reads as a
// place to type, the line above it names the action, and Enter is named for the
// attempt it makes instead of for a selection.
//
// Every line goes through wrapText against the frame's own width rather than being
// hand-wrapped: the frame pads every row to the widest one in the body, so a single
// literal line longer than the terminal makes the whole screen overflow, which a
// guard measures at twelve terminal sizes down to 60x20.
func (m Model) renderAdminPassword() string {
	// The prose is the composition's measure, not the terminal's: a 200-column
	// terminal would otherwise wrap these sentences at 196 columns and then centre a
	// block wider than the composition every other screen shares.
	width := layoutFor(m).Composition
	hints := []installerHint{hintValidate, hintCancel}

	typed := len(m.AdminPassword)
	masked := strings.Repeat("•", typed)
	if typed > adminPasswordMaskLimit {
		masked = strings.Repeat("•", adminPasswordMaskLimit) + cutMarker
	}
	if typed == 0 {
		masked = MutedStyle.Render("type it here")
	}

	body := []string{BrandStyle.Render(m.GetScreenTitle())}
	body = append(body, wrapText("sudo needs it for the commands that change the machine.", width, 0)...)
	body = append(body, wrapText("Type your administrator password and press Enter.", width, 0)...)
	body = append(body, "", "  "+masked, "")
	body = append(body, wrapText(fmt.Sprintf("%d characters typed; the field holds up to %d.", typed, adminPasswordCapacity), width, 0)...)

	if m.AdminPasswordPending {
		body = append(body, "")
		for _, row := range wrapText("Checking it with sudo; Escape stops waiting for the answer.", width, 0) {
			body = append(body, MutedStyle.Render(row))
		}
	}

	body = append(body, "")
	// What Escape is for, said here rather than discovered: this screen is the only
	// place the credential is asked for, so the way out is a run that carries on
	// without one and reports what that costs, not a prompt somewhere else.
	going := "Nothing is written down, and nothing is kept: the field is cleared as soon as sudo has read it. Escape goes on without it, and a step that needs root will say so."
	for _, row := range wrapText(going, width, 0) {
		body = append(body, MutedStyle.Render(row))
	}

	if l := layoutFor(m); !m.frameComposesBody(l, m.panelsFor()) {
		body = centeredBody(body, l)
	}
	return m.frame(m.headerName(), "", body, hints)
}

// updateAdminPassword is the installer's password screen. It takes runes and
// backspace into the field, Enter to make the attempt, and Escape (or ctrl+c) to
// stop waiting for it.
//
// An attempt that is in flight owns the field: any other key is ignored while sudo
// has the bytes, because a second reader on the same buffer is a second attempt
// the run has not been asked for. The two cancel keys still work, so a sudo that
// never answers cannot trap the screen.
func (m Model) updateAdminPassword(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Type == tea.KeyEsc || key.String() == "ctrl+c" {
		return m.cancelAdminPassword()
	}

	switch key.Type {
	case tea.KeyRunes:
		if m.AdminPasswordPending {
			return m, nil
		}
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
		if m.AdminPasswordPending {
			return m, nil
		}
		if len(m.AdminPassword) > 0 {
			m.AdminPassword = m.AdminPassword[:len(m.AdminPassword)-1]
		}
		return m, nil

	case tea.KeyEnter:
		if m.AdminPasswordPending || len(m.AdminPassword) == 0 {
			return m, nil
		}
		m.AdminPasswordPending = true
		m.AdminPasswordAttempt++
		m.LogLines = append(m.LogLines, "Administrator access: validating the password typed on the installer's screen.")
		return m, sudoPasswordPrimer(m.AdminPassword, m.AdminPasswordAttempt)
	}

	return m, nil
}
