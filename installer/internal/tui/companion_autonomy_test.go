package tui

import "testing"

// Cursor motion is information for the user, not a destination for the pet.
func TestCompanionDoesNotFollowCursor(t *testing.T) {
	m := Model{Animating: true, Width: 160, Height: 50, Screen: ScreenMainMenu, Cursor: 0}
	m.CompanionPos = 0
	m.Cursor = len(m.GetCurrentOptions()) - 1
	m.pauseCompanionAfterInput()
	m.CompanionIdle = 0
	for i := 0; i < 8; i++ {
		m.AnimTick++
		m.advanceCompanion()
	}
	if m.CompanionPos != 0 {
		t.Fatalf("cursor moved the companion to cell %d; autonomous pet must ignore the user's selection", m.CompanionPos)
	}
	if m.CompanionGaze != (companionGaze{}) {
		t.Fatalf("cursor changed the companion's gaze to %+v; autonomous gaze must not track user input", m.CompanionGaze)
	}
}

func TestCompanionAutonomyIsSeededBoundedAndQuiet(t *testing.T) {
	makePet := func(seed uint32) Model {
		return Model{
			Animating:           true,
			Width:               160,
			Height:              50,
			CompanionIdle:       companionRoamAfterTicks,
			CompanionRandom:     seed,
			CompanionRoamTarget: -1,
		}
	}
	first, second := makePet(0x12345678), makePet(0x12345678)
	moved := false
	previous := first.CompanionPos
	for i := 0; i < 500; i++ {
		first.AnimTick++
		second.AnimTick++
		first.advanceCompanion()
		second.advanceCompanion()
		if distance := companionDistance(first.CompanionPos, previous); distance > 1 {
			t.Fatalf("tick %d jumped %d cells; autonomous stroll must take discrete steps", i, distance)
		}
		moved = moved || first.CompanionPos != previous
		previous = first.CompanionPos
		if first.CompanionPos != second.CompanionPos || first.CompanionGaze != second.CompanionGaze ||
			first.CompanionHop != second.CompanionHop || first.CompanionRandom != second.CompanionRandom {
			t.Fatalf("same injected seed diverged at tick %d", i)
		}
	}
	if !moved {
		t.Fatal("the pet did not choose an autonomous stroll during its quiet play window")
	}

	// Recent user activity wins over a pending game: it gets several seconds of
	// visual calm rather than a pet moving under the user's hands.
	first.CompanionIdle = 0
	first.pauseCompanionAfterInput()
	start := first.CompanionPos
	for i := 0; i < companionRoamAfterTicks-1; i++ {
		first.AnimTick++
		first.advanceCompanion()
	}
	if first.CompanionPos != start || first.CompanionMoving {
		t.Fatalf("pet moved during the post-input quiet interval: cell %d -> %d", start, first.CompanionPos)
	}
}
