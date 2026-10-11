package tui

import (
	"fmt"
	"strings"
	"testing"
)

// ============================================================================
// THE PET'S OWN GUARDS
// ============================================================================
//
// Every guard here is written so that removing the behaviour it protects turns it
// red, and each comment names the failure that removal produces. They are the
// proof that the creature moves, that it reacts to what the installer is doing,
// that it never claims a step finished, that no frame outgrows the space reserved
// for it, and that its own play is reachable and leaves again.

// companionFramesOf lists every hand-drawn frame of one state at one height, in
// the order the sequence draws them.
func companionFramesOf(state companionState, height int) []string {
	var rows []string
	for _, frame := range companionFramesAt(height) {
		if frame.state == state {
			rows = append(rows, strings.Join(frame.rows, "\n"))
		}
	}
	return rows
}

// TestCompanionPetFrameSequenceAdvancesWithTheClock proves each action is an
// animation and not one glyph that shifts position: a walking cat shows more than
// one frame, a jump has a crouch, an air pose and a landing, a bout of play shows
// its paws move, and an idle frame holds its cadence and then breathes.
//
// The failure it produces without the fix: with one frame per action the
// per-height frame counts below are 1 and the loops over the cycle see a single
// distinct pose, so the test reports "walk has 1 frame" and "the jump drew the
// same pose at every step".
func TestCompanionPetFrameSequenceAdvancesWithTheClock(t *testing.T) {
	for _, height := range companionHeights() {
		height := height
		t.Run(companionStateNames[companionWalkingState], func(t *testing.T) {
			walk := companionFramesOf(companionWalkingState, height)
			if len(walk) < 2 {
				t.Fatalf("the walk at height %d has %d frames, want at least two to alternate", height, len(walk))
			}
			distinct := map[string]struct{}{}
			for tick := 0; tick < len(walk); tick++ {
				distinct[strings.Join(companionArtFor(companionWalkingState, height, tick, companionGaze{}), "\n")] = struct{}{}
			}
			if len(distinct) != len(walk) {
				t.Errorf("the walk at height %d drew %d distinct poses across %d frames", height, len(distinct), len(walk))
			}
		})

		t.Run(companionStateNames[companionJumpingState], func(t *testing.T) {
			seen := map[string]int{}
			for tick := 0; tick < 3; tick++ {
				seen[strings.Join(companionArtFor(companionJumpingState, height, tick, companionGaze{}), "\n")] = tick
			}
			if len(seen) != 3 {
				t.Errorf("the jump at height %d drew %d distinct poses across its three phases, want crouch, air and landing", height, len(seen))
			}
		})

		t.Run(companionStateNames[companionPlayingState], func(t *testing.T) {
			seen := map[string]struct{}{}
			for frame := 0; frame < 3; frame++ {
				tick := frame * companionPlayStepTicks
				seen[strings.Join(companionArtFor(companionPlayingState, height, tick, companionGaze{}), "\n")] = struct{}{}
			}
			if len(seen) < 3 {
				t.Errorf("the play at height %d drew %d distinct poses across three frame slots, want its paws to move", height, len(seen))
			}
		})
	}

	// The idle cadence: two ticks inside one period draw the same frame, and the
	// frame two periods on is the breath. The breath is what proves the sequence
	// advances with the clock rather than being a single table row.
	before := strings.Join(companionArtFor(companionIdleState, companionFullHeight, 0, companionGaze{}), "\n")
	inside := strings.Join(companionArtFor(companionIdleState, companionFullHeight, 1, companionGaze{}), "\n")
	if before != inside {
		t.Errorf("the idle frame changed inside its %d-tick cadence:\nbefore:\n%s\nafter:\n%s", companionIdleFrameTicks, before, inside)
	}
	breath := strings.Join(companionArtFor(companionIdleState, companionFullHeight, companionIdleFrameTicks, companionGaze{}), "\n")
	if before == breath {
		t.Error("the idle sequence never reached its second frame: the glyph cat is a still drawing")
	}
}

// TestCompanionPetReactsToAStepStartingSucceedingAndFailing proves the three beats
// the installer itself produces are three different reactions: a step that is
// running draws the watching face, one that finished draws the pleased face, and
// one that failed flinches. It also pins the honesty rule directly -- the watching
// frame is not the pleased one -- so a reaction to a step can never be mistaken for
// a claim that the step is done.
//
// The failure it produces without the fix: with one shared "busy" reaction for all
// three beats the rendered frames below are not pairwise distinct, and the test
// reports "the started and finished frames are identical".
func TestCompanionPetReactsToAStepStartingSucceedingAndFailing(t *testing.T) {
	starting := Model{
		Animating: true,
		Screen:    ScreenInstalling,
		Steps:     []InstallStep{{ID: "clone", Status: StatusRunning}},
	}
	finished := Model{
		Animating:        true,
		Screen:           ScreenInstalling,
		Steps:            []InstallStep{{ID: "clone", Status: StatusDone}},
		CompanionPleased: companionPleasedTicks,
	}
	failing := Model{
		Animating: true,
		Screen:    ScreenInstalling,
		Steps:     []InstallStep{{ID: "clone", Status: StatusFailed}},
		ErrorMsg:  "exit status 1",
	}

	cases := []struct {
		name  string
		model Model
		state companionState
	}{
		{"step running is watched", starting, companionWatchingState},
		{"step finished is pleased", finished, companionPleasedState},
		{"step failed flinches", failing, companionFlinchingState},
	}

	rendered := map[string]string{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.model.companionStateNow(); got != tc.state {
				t.Fatalf("state = %s, want %s", companionStateNames[got], companionStateNames[tc.state])
			}
			rows := tc.model.companionSprite(80, companionFullHeight)
			if len(rows) != companionFullHeight {
				t.Fatalf("the reaction drew %d rows, want the reserved %d", len(rows), companionFullHeight)
			}
			rendered[tc.name] = strings.Join(rows, "\n")
		})
	}

	for i, a := range cases {
		for _, b := range cases[i+1:] {
			if rendered[a.name] == rendered[b.name] {
				t.Errorf("the %s and %s reactions draw the same frame:\n%s", a.name, b.name, rendered[a.name])
			}
		}
	}

	// The direct honesty check: the face for a step that is merely running is not
	// the face for a step that is done, at both the glyph and the volume tier.
	startingFrame := strings.Join(starting.companionSprite(80, companionFullHeight), "\n")
	pleasedFrame := strings.Join(companionArtFor(companionPleasedState, companionFullHeight, 0, companionGaze{}), "\n")
	if startingFrame == pleasedFrame {
		t.Errorf("a running step drew the pleased face, which claims it finished:\n%s", startingFrame)
	}
}

// TestCompanionPetNeverClaimsAStepFinished is the honesty guard: while one of the
// installer's own steps is running, the creature may react to it but may not draw
// the pleased face, and a progress message -- which only reports a fraction, not a
// finish -- may not arm the celebration either. It walks the whole idle cadence so
// every frame of every state is reached, not only the first.
//
// The failure it produces without the fix: if the watching state were mapped to
// the pleased art, or if a progress message armed CompanionPleased, the tick loop
// and the message below both report "a running step claimed a finish".
func TestCompanionPetNeverClaimsAStepFinished(t *testing.T) {
	m := Model{
		Animating: true,
		Screen:    ScreenInstalling,
		Width:     160,
		Height:    50,
		Steps:     []InstallStep{{ID: "clone", Name: "Clone the repository", Status: StatusRunning}},
	}

	pleased := companionFramesOf(companionPleasedState, companionFullHeight)
	pleasedSet := map[string]struct{}{}
	for _, frame := range pleased {
		pleasedSet[frame] = struct{}{}
	}

	for tick := 0; tick < 3*companionIdleFrameTicks; tick++ {
		m.AnimTick = tick
		state := m.companionStateNow()
		if state == companionPleasedState {
			t.Fatalf("tick %d drew the pleased face while a step was still running", tick)
		}
		frame := strings.Join(companionArtFor(state, companionFullHeight, tick, companionGaze{}), "\n")
		if _, ok := pleasedSet[frame]; ok {
			t.Fatalf("tick %d drew a pleased frame while a step was still running:\n%s", tick, frame)
		}
	}

	next, _ := m.Update(stepProgressMsg{stepID: "clone", progress: 0.5})
	got := next.(Model)
	if got.CompanionPleased != 0 {
		t.Errorf("a progress message armed the celebration: CompanionPleased = %d", got.CompanionPleased)
	}
	if got.companionStateNow() == companionPleasedState {
		t.Errorf("a progress message drew the pleased face: half a step is not a finished one")
	}
}

// TestCompanionPetFrameNeverExceedsItsReservedSpace is the space guard. Every
// hand-drawn frame at every rung is exactly its rung's rows, every volume rung
// draws exactly half its pixel grid as terminal rows, and the guard itself refuses
// a frame one row too tall -- which is the failure the guard exists to catch.
//
// The failure it produces without the fix: an art row added to a table without a
// home makes the per-frame row count differ from the reservation and the test
// reports the state, the height and both numbers; and weakening companionFrameFits
// to accept anything makes the final teeth check report "the guard accepted a
// frame 6 rows tall in a 5-row reservation".
func TestCompanionPetFrameNeverExceedsItsReservedSpace(t *testing.T) {
	for _, height := range companionHeights() {
		reserved := companionReservedRows(height)
		for _, frame := range companionFramesAt(height) {
			if len(frame.rows) != reserved {
				t.Errorf("state %s at height %d draws %d rows, want the reserved %d",
					companionStateNames[frame.state], height, len(frame.rows), reserved)
			}
			if !companionFrameFits(frame.rows, reserved) {
				t.Errorf("state %s at height %d is refused by the frame guard it must pass",
					companionStateNames[frame.state], height)
			}
		}
		for _, state := range companionStates() {
			art := companionArtFor(state, height, 0, companionGaze{})
			if !companionFrameFits(art, companionReservedRows(height)) {
				t.Errorf("state %s at height %d composes %d rows into a %d-row reservation",
					companionStateNames[state], height, len(art), companionReservedRows(height))
			}
		}
	}

	for _, size := range companionVolumeRungs() {
		reserved := companionReservedRows(size.rows / 2)
		if reserved != size.rows/2 {
			t.Errorf("the %dx%d volume reserves %d rows, want %d", size.width, size.rows, reserved, size.rows/2)
		}
		rows := make([]string, size.rows/2)
		if !companionFrameFits(rows, reserved) {
			t.Errorf("the %dx%d volume's own row count is refused by its reservation", size.width, size.rows)
		}
	}

	// Teeth: one row over the reservation is refused. Without this half of the
	// guard a frame could outgrow its block and overdraw the rule below it.
	tooTall := make([]string, companionFullHeight+1)
	if companionFrameFits(tooTall, companionFullHeight) {
		t.Errorf("the frame guard accepted a %d-row frame in a %d-row reservation, so it has no teeth",
			len(tooTall), companionFullHeight)
	}
}

// TestCompanionPetIdleAndPlayAreReachableAndLeave pins the two states a still
// creature lives in. Idle is what an awake model with nothing to react to draws;
// play is reachable only after the same quiet stretch that earns a stroll, it is
// present inside its window, and it is gone once the window closes. Both are read
// from the model's own clock rather than from a timer, so both are pinnable.
//
// The failure it produces without the fix: if play had no window and never left,
// the "leaves when the window closes" assertion reports that the model is still
// playing; if the quiet stretch did not gate it, the "an early model cannot play"
// assertion reports play before the creature has waited.
func TestCompanionPetIdleAndPlayAreReachableAndLeave(t *testing.T) {
	idle := Model{Animating: true, Width: 160, Height: 50}
	if got := idle.companionStateNow(); got != companionIdleState {
		t.Fatalf("an awake model with nothing to react to draws %s, want idle", companionStateNames[got])
	}

	playing := Model{
		Animating:     true,
		Width:         160,
		Height:        50,
		CompanionIdle: companionRoamAfterTicks,
		AnimTick:      companionPlayPeriodTicks,
	}
	if got := playing.companionStateNow(); got != companionPlayingState {
		t.Fatalf("the play window draws %s, want playing", companionStateNames[got])
	}

	after := playing
	after.AnimTick = companionPlayFrameTicks
	if got := after.companionStateNow(); got != companionIdleState {
		t.Errorf("play did not leave when its window closed: state = %s", companionStateNames[got])
	}

	early := playing
	early.CompanionIdle = companionRoamAfterTicks - 1
	if got := early.companionStateNow(); got != companionIdleState {
		t.Errorf("play started before the quiet stretch was over: state = %s", companionStateNames[got])
	}
}

// TestCompanionPetReactionsAreSpecificNotAGenericWiggle pins the second half of
// the reaction rule: the theme the user chose and the release that is published are
// their own faces, distinct from one another, from the idle face, from the pleased
// face, and from each other, and each leaves when its condition does.
//
// The failure it produces without the fix: mapping both to the pleased face makes
// the pairwise comparison report "charmed and curious draw the same frame"; leaving
// the record on screen after the picker closes makes the leaving assertions report
// that the reaction lingers.
func TestCompanionPetReactionsAreSpecificNotAGenericWiggle(t *testing.T) {
	render := func(m Model) string {
		return strings.Join(companionArtFor(m.companionStateNow(), companionFullHeight, 0, companionGaze{}), "\n")
	}

	idle := Model{Animating: true}
	charmed := Model{Animating: true, Screen: ScreenThemePicker, DotfilesThemeRecord: &dotfilesThemeRecord{}}
	curious := Model{Animating: true, UpdateCheck: updateState{Latest: "v9.9.9"}}
	pleased := Model{Animating: true, CompanionPleased: companionPleasedTicks}

	cases := []struct {
		name  string
		model Model
		state companionState
	}{
		{"idle", idle, companionIdleState},
		{"a chosen theme charms", charmed, companionCharmedState},
		{"a published release is curious", curious, companionCuriousState},
		{"a finished step is pleased", pleased, companionPleasedState},
	}
	frames := map[string]string{}
	for _, tc := range cases {
		if got := tc.model.companionStateNow(); got != tc.state {
			t.Errorf("%s draws state %s, want %s", tc.name, companionStateNames[got], companionStateNames[tc.state])
		}
		frames[tc.name] = render(tc.model)
	}
	for i, a := range cases {
		for _, b := range cases[i+1:] {
			if frames[a.name] == frames[b.name] {
				t.Errorf("%s and %s draw the same frame, so the reactions are a generic wiggle:\n%s",
					a.name, b.name, frames[a.name])
			}
		}
	}

	// Both reactions leave when their condition does: the theme record without the
	// picker, and the update once this run has installed it.
	gone := charmed
	gone.Screen = ScreenMainMenu
	if gone.companionStateNow() == companionCharmedState {
		t.Error("a theme change still charms the creature after the picker is gone")
	}
	installed := curious
	installed.UpdateCheck.Installed = true
	if installed.companionStateNow() == companionCuriousState {
		t.Error("the creature is still curious after this run installed the published release")
	}
}

// TestCompanionPetIdleCycleIsVisibleWithinABound is the liveliness guard the
// report asked for: an idle creature is not a held frame. Every idle tier draws
// at least three distinct poses and reaches all of them inside one cycle of
// companionIdleFrameTicks ticks, and the volume's model cycle moves too.
//
// The failure it produces without the fix: with one idle pose the distinct-pose
// count is 1 and the test reports "want at least three so the cycle reads as
// motion"; with the old two-second cadence the 21-tick window never leaves the
// first pose.
func TestCompanionPetIdleCycleIsVisibleWithinABound(t *testing.T) {
	if companionIdleFrameTicks < 2 || companionIdleFrameTicks > 4 {
		t.Fatalf("the idle frame cadence is %d ticks, want 2-4 (%.2f-%.2f s at %d fps)",
			companionIdleFrameTicks, 2.0/animTicksPerSecond, 4.0/animTicksPerSecond, animTicksPerSecond)
	}
	for _, height := range companionHeights() {
		height := height
		t.Run(fmt.Sprintf("height %d", height), func(t *testing.T) {
			frames := companionFramesOf(companionIdleState, height)
			if len(frames) < 3 {
				t.Fatalf("the idle at height %d has %d poses, want at least three so the cycle reads as motion", height, len(frames))
			}
			bound := len(frames) * companionIdleFrameTicks
			seen := map[string]struct{}{}
			for tick := 0; tick < bound; tick++ {
				seen[strings.Join(companionArtFor(companionIdleState, height, tick, companionGaze{}), "\n")] = struct{}{}
			}
			if len(seen) != len(frames) {
				t.Errorf("the idle at height %d drew %d distinct poses in %d ticks, want all %d", height, len(seen), bound, len(frames))
			}
		})
	}

	// The volume's idle is a model cycle rather than a frame table, so it is checked
	// separately: it must change within a bounded number of ticks.
	size, _ := companionVolumeSizeFor(companionVolumeFullHeight)
	base := companionVolumeTones(companionIdleState, 0, companionGaze{}, 0, size)
	moved := false
	for tick := 1; tick <= companionIdleFrameTicks*4; tick++ {
		if fmt.Sprint(base) != fmt.Sprint(companionVolumeTones(companionIdleState, tick, companionGaze{}, 0, size)) {
			moved = true
			break
		}
	}
	if !moved {
		t.Errorf("the volume idle frame did not change within %d ticks", companionIdleFrameTicks*4)
	}
}

// TestCompanionPetStrollAndPlayAreReachableWithinABound asserts the cadences the
// report names and that a model actually reaches each action inside them. The band
// checks come first, so a slow cadence fails even if a particular model strolls.
//
// The failure it produces without the fix: the old eight-second quiet gate and
// six-to-fourteen-second wait put the stroll bound at twenty-two seconds, far past
// the four asserted here, and the old play period at sixteen seconds, past twelve.
func TestCompanionPetStrollAndPlayAreReachableWithinABound(t *testing.T) {
	if got := companionRoamAfterTicks + companionRoamMaxWaitTicks; got > 4*animTicksPerSecond {
		t.Errorf("a stroll may begin up to %d ticks (%ds) after quiet, want within four seconds", got, got/animTicksPerSecond)
	}
	if companionPlayPeriodTicks < 8*animTicksPerSecond || companionPlayPeriodTicks > 12*animTicksPerSecond {
		t.Errorf("the play period is %d ticks (%ds), want eight to twelve seconds", companionPlayPeriodTicks, companionPlayPeriodTicks/animTicksPerSecond)
	}

	bound := companionRoamAfterTicks + companionRoamMaxWaitTicks
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Screen = ScreenMainMenu
	m.Width, m.Height = 160, 50
	m.Animating = true
	strolled := false
	for tick := 0; tick < bound; tick++ {
		m.AnimTick++
		m.advanceCompanion()
		if m.CompanionMoving || m.CompanionRoamActive {
			strolled = true
			break
		}
	}
	if !strolled {
		t.Errorf("the pet did not choose a stroll within %d ticks (%ds)", bound, bound/animTicksPerSecond)
	}

	play := NewModel()
	isolateGoldenTest(t, &play)
	play.Screen = ScreenMainMenu
	play.Width, play.Height, play.Animating = 160, 50, true
	play.CompanionIdle = companionRoamAfterTicks + 1
	played := false
	for delta := 0; delta < companionPlayPeriodTicks; delta++ {
		play.AnimTick = delta
		if play.companionStateNow() == companionPlayingState {
			played = true
			break
		}
	}
	if !played {
		t.Errorf("the play window did not open within %d ticks (%ds)", companionPlayPeriodTicks, companionPlayPeriodTicks/animTicksPerSecond)
	}
}

// TestCompanionPetPrintedFrameSequence is the look-at-it guard: it renders the
// real main-menu screen at successive ticks and logs the companion's rows from
// the rendered view, so the sequence can be read rather than only measured. It
// asserts that the printed frames genuinely differ, which is what the report's
// "look for five seconds and see the creature doing something" reduces to in a
// test.
func TestCompanionPetPrintedFrameSequence(t *testing.T) {
	for _, tier := range []struct {
		name  string
		pixel bool
	}{{"glyph", false}, {"volume", true}} {
		tier := tier
		t.Run(tier.name, func(t *testing.T) {
			m := NewModel()
			isolateGoldenTest(t, &m)
			m.Screen = ScreenMainMenu
			m.Width, m.Height = 160, 50
			m.Animating, m.PixelSprite = true, tier.pixel
			m.ink = companionInkFor(true)
			last, changes := "", 0
			distinct := map[string]struct{}{}
			for tick := 0; tick <= 20; tick++ {
				m.AnimTick = tick
				viewRows := strings.Split(m.View(), "\n")
				var frame strings.Builder
				for _, row := range companionOwnedRows(viewRows) {
					frame.WriteString(viewRows[row])
					frame.WriteByte('\n')
				}
				got := frame.String()
				if got != last {
					changes++
				}
				last = got
				distinct[got] = struct{}{}
				t.Logf("tick %2d state=%s position=%d:\n%s", tick, companionStateNames[m.companionStateNow()], m.CompanionPos, got)
			}
			if len(distinct) < 3 {
				t.Errorf("the %s sequence drew %d distinct frames across 21 ticks, want a moving creature", tier.name, len(distinct))
			}
			if changes < 4 {
				t.Errorf("the %s sequence changed only %d times across 21 ticks", tier.name, changes)
			}
		})
	}
}
