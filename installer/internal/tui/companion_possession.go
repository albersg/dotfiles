package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// THE POSSESSED FORM
//
// A boss fight is the one screen in the trainer where the creature is not on the
// player's side, and the possession is that joke made literal: the same animal, with
// a spiked crest, fangs and the lights turned red, standing across the code from the
// player. It is the trainer's own boss screen that turns it, and leaving that screen
// turns it back.
//
// The form is a state in the companion's own machine rather than a flag painted over
// a frame, for three reasons the rest of this package already pays for:
//
//   - The art tables ARE the sprite. A state is a frame per rung, so the possessed
//     creature exists at the one-row floor and at both volume rungs, and the frame
//     guard that refuses an overflowing frame keeps refusing it.
//   - Nothing here reads the clock. The state comes from the model's screen and the
//     fight's lives and the art comes from the tick, so a snapshot can pin the
//     possession instead of flaking on time.
//   - The pose and the eye mark are the two places this package expresses a state, so
//     the rage below is one more value in the same switches rather than a second
//     renderer.
//
// The escalation is the fight itself: with lives to spare the creature is possessed,
// and on the last life it shakes with rage on the same one-pixel tremble a startle
// uses. Nothing about the possession outlives the fight, because the only question
// this file answers is which screen the player is looking at.
// ============================================================================

const (
	// companionPossessionCalm is the ordinary creature.
	companionPossessionCalm = iota
	// companionPossessionFight is a boss fight with lives to spare.
	companionPossessionFight
	// companionPossessionRage is the last life: the deepest form, and the one that
	// trembles.
	companionPossessionRage
)

// companionPossessionRageLives is how many lives are left when the creature stops
// holding still. One, because that is the life the player is about to spend, and the
// boss header already says so: the tremble is the creature agreeing with it.
const companionPossessionRageLives = 1

// companionPossessionDepth is how deep the possession has gone: the ordinary creature,
// the possessed form of a fight with lives to spare, or the rage of the last life. It
// reads the fight the trainer is actually running -- the screen, the mode and the lives
// the boss screen already shows -- so the creature and the fight cannot disagree about
// how the fight is going.
func (m Model) companionPossessionDepth() int {
	if m.Screen != ScreenTrainerBoss || m.TrainerGameState == nil || !m.TrainerGameState.IsBossMode {
		return companionPossessionCalm
	}
	if m.TrainerGameState.BossLives <= companionPossessionRageLives {
		return companionPossessionRage
	}
	return companionPossessionFight
}

// companionEmberGlyph is the character the possessed art marks its eyes with, and the
// renderer paints every occurrence of it in the ember colour. It is a glyph rather than
// the eye row's own metadata because the one-row art has no interior eye cell for the
// gaze composer to move -- and the one-row rung is exactly what the trainer draws at
// the documented 80x24 floor, which is where the red eyes have to be visible.
const companionEmberGlyph = '@'

// CompanionEmberStyle is the colour the possessed creature's eyes are drawn in. A fixed
// red rather than a theme colour, because "red eyes" is the whole of the request and a
// theme's accent is not red. Bold, because one red glyph in a twenty-four colour
// terminal is easy to miss.
var CompanionEmberStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#ff2d2d")).
	Bold(true)

// The possessed rows. Each is the ordinary cat's row with the three changes the form
// is: a spiked crest where the ears' span was, the ember glyph for both eyes, and two
// fangs where the mouth's curve was. Every row keeps the width of the row it replaces,
// so the creature still cannot jitter sideways as the eye reads it.
const (
	companionPossessedCrest      = ` /\^^^^^^/\   `
	companionPossessedEyeRow     = `(  @    @  )  `
	companionPossessedFangRow    = `(  v    v  )  `
	companionPossessedCrestSmall = ` /\^^^^^/\    `
	companionPossessedEyeSmall   = `(  @   @  )   `
	companionPossessedFangSmall  = `  >  v  <     `
	// The second compact frame is the same snarl with its fangs drawn in: two frames of
	// one form, so the head is not a still image at that rung either.
	companionPossessedFangSmallAlt = `  > v   <     `
	// The one-row art is seven columns wide, which is room for the face and the spike
	// its crest becomes at that rung. The face stays three cells wide and carries no
	// gaze, the same documented exception the rest of the one-row art has.
	companionPossessedMini     = `(@.@) ^`
	companionPossessedMiniRage = `^(@.@) `
)

// companionPossessedFrames is the possessed art at each rung, two frames each: the held
// form and the frame it alternates with, so the creature is not a still image while the
// player thinks. The full rung shifts its paws, and the one-row rung swaps its spike to
// the other side of the face.
var companionPossessedFrames = map[int][]companionFrame{
	companionFullHeight: {
		{companionPossessedState, []string{
			companionPossessedCrest, companionFullEyeEmpty, companionPossessedEyeRow,
			companionPossessedFangRow, companionFullPaws,
		}, companionFullEyes},
		{companionPossessedState, []string{
			companionPossessedCrest, companionFullEyeEmpty, companionPossessedEyeRow,
			companionPossessedFangRow, companionFullPawsStride,
		}, companionFullEyes},
	},
	companionCompactHeight: {
		{companionPossessedState, []string{
			companionPossessedCrestSmall, companionPossessedEyeSmall, companionPossessedFangSmall,
		}, companionCompactEyes},
		{companionPossessedState, []string{
			companionPossessedCrestSmall, companionPossessedEyeSmall, companionPossessedFangSmall,
		}, companionCompactEyes},
		{companionPossessedState, []string{
			companionPossessedCrestSmall, companionPossessedEyeSmall, companionPossessedFangSmallAlt,
		}, companionCompactEyes},
	},
	companionMiniHeight: {
		{companionPossessedState, []string{companionPossessedMini}, companionNoEyes},
		{companionPossessedState, []string{companionPossessedMiniRage}, companionNoEyes},
	},
}

// companionEarsFor is the creature's ears, or the possessed form's horns. The tips are
// the top of the sprite in every state, so the possessed form does not lift them: it
// narrows their base and splays the tips outward, which reads as a horn without taking
// a row the cell does not have and without a primitive the frame guard would have to
// allow for.
func companionEarsFor(state companionState, headX, headY, earBack, earTilt, earLift float64) [2]companionTriangle {
	if state != companionPossessedState {
		return [2]companionTriangle{
			{points: [3][2]float64{{headX - 0.22 + earBack, headY + 0.17}, {headX - 0.13 + earBack + earTilt, headY + 0.50 + earLift}, {headX + 0.01 + earBack, headY + 0.17}}, r: 0.025},
			{points: [3][2]float64{{headX + 0.02 + earBack, headY + 0.17}, {headX + 0.20 + earBack + earTilt, headY + 0.50 + earLift}, {headX + 0.26 + earBack, headY + 0.17}}, r: 0.025},
		}
	}
	return [2]companionTriangle{
		{points: [3][2]float64{{headX - 0.19 + earBack, headY + 0.17}, {headX - 0.26 + earBack + earTilt, headY + 0.50 + earLift}, {headX - 0.03 + earBack, headY + 0.17}}, r: 0.020},
		{points: [3][2]float64{{headX + 0.03 + earBack, headY + 0.17}, {headX + 0.26 + earBack + earTilt, headY + 0.50 + earLift}, {headX + 0.19 + earBack, headY + 0.17}}, r: 0.020},
	}
}

// companionGlyphRow renders one row of the glyph art: the shared style for the whole
// row, except that every ember glyph is painted in the ember colour. Runs of one style
// are written as one escape sequence, the way the volume's own row encoder writes them,
// because the creature is redrawn on every animation frame and a style per character
// would be the most expensive thing on the screen.
//
// An ordinary row carries no ember glyph, so this returns exactly what the shared style
// drew for it; the ember rule is pinned against the ordinary tables by a test rather
// than by a branch here.
func companionGlyphRow(row string) string {
	if !strings.ContainsRune(row, companionEmberGlyph) {
		return CompanionStyle.Render(row)
	}

	var out strings.Builder
	runes := []rune(row)
	for start := 0; start < len(runes); {
		ember := runes[start] == companionEmberGlyph
		end := start
		for end < len(runes) && (runes[end] == companionEmberGlyph) == ember {
			end++
		}
		run := string(runes[start:end])
		if ember {
			out.WriteString(CompanionEmberStyle.Render(run))
		} else {
			out.WriteString(CompanionStyle.Render(run))
		}
		start = end
	}
	return out.String()
}
