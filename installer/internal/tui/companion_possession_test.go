package tui

import (
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/tui/trainer"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// possessedModel is a model standing on the trainer's boss screen with a fight
// running, which is the only situation the possession is about.
func possessedModel(t *testing.T, lives int) Model {
	t.Helper()

	m := NewModel()
	m.Width = 80
	m.Height = 24
	m.Animating = true
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
	m.TrainerGameState.StartBoss(trainer.ModuleHorizontal)
	m.TrainerGameState.BossLives = lives
	m.TrainerModules = trainer.GetAllModules()
	m.Screen = ScreenTrainerBoss
	return m
}

// TestCompanionIsPossessedOnlyDuringABossFight pins the trigger: the creature turns
// while a boss fight is on screen, and it is its ordinary self on every other screen
// and with no fight loaded.
func TestCompanionIsPossessedOnlyDuringABossFight(t *testing.T) {
	fighting := possessedModel(t, 3)
	if got := fighting.companionStateNow(); got != companionPossessedState {
		t.Fatalf("on the boss screen the creature draws %s, want the possessed form",
			companionStateNames[got])
	}
	if got := fighting.companionPossessionDepth(); got != companionPossessionFight {
		t.Errorf("a fight with lives to spare has possession depth %d, want %d",
			got, companionPossessionFight)
	}

	ordinary := possessedModel(t, 3)
	ordinary.Screen = ScreenTrainerMenu
	if got := ordinary.companionStateNow(); got == companionPossessedState {
		t.Error("the creature is possessed on the trainer menu, which is not a fight")
	}
	if got := ordinary.companionPossessionDepth(); got != companionPossessionCalm {
		t.Errorf("outside a fight the possession depth is %d, want 0", got)
	}

	noFight := NewModel()
	noFight.Width = 80
	noFight.Height = 24
	noFight.Screen = ScreenTrainerBoss
	if got := noFight.companionStateNow(); got == companionPossessedState {
		t.Error("the creature is possessed with no fight loaded")
	}
}

// TestCompanionPossessionDeepensOnTheLastLife pins the escalation: the last life is
// the deepest form and it shakes, the same one-pixel tremble a startle draws, on
// alternate frames.
func TestCompanionPossessionDeepensOnTheLastLife(t *testing.T) {
	raging := possessedModel(t, 1)
	if got := raging.companionPossessionDepth(); got != companionPossessionRage {
		t.Fatalf("the last life has possession depth %d, want %d", got, companionPossessionRage)
	}

	shivers := map[int]bool{}
	for tick := 0; tick < 2; tick++ {
		raging.AnimTick = tick
		shiver := raging.companionShiverPx()
		if shiver == 0 {
			t.Errorf("tick %d: the creature at its last life does not shake", tick)
		}
		shivers[shiver] = true
	}
	if len(shivers) != 2 {
		t.Errorf("the rage tremble does not alternate: %v", shivers)
	}

	calm := possessedModel(t, 3)
	for tick := 0; tick < 2; tick++ {
		calm.AnimTick = tick
		if got := calm.companionShiverPx(); got != 0 {
			t.Errorf("tick %d: a fight with lives to spare shakes by %d, want 0", tick, got)
		}
	}
}

// TestCompanionPossessedArtExistsAtEveryRung pins the art: every rung of the sprite
// ladder has a possessed frame, it fits the rung, and it carries the ember glyph the
// renderer paints. The one-row rung is the documented 80x24 floor, so it is the rung
// the possession most has to survive.
func TestCompanionPossessedArtExistsAtEveryRung(t *testing.T) {
	for _, height := range companionHeights() {
		art := companionArtFor(companionPossessedState, height, 0, companionGaze{})
		if len(art) != height {
			t.Errorf("height %d: the possessed art is %d rows, want %d", height, len(art), height)
			continue
		}
		if !companionFrameFits(art, companionReservedRows(height)) {
			t.Errorf("height %d: the possessed art does not fit its reserved rows", height)
		}
		if !strings.Contains(strings.Join(art, "\n"), string(companionEmberGlyph)) {
			t.Errorf("height %d: the possessed art carries no ember glyph, so no eye can be red:\n%s",
				height, strings.Join(art, "\n"))
		}
	}
}

// TestCompanionEmberGlyphBelongsToThePossession pins the other half of the rendering
// rule: only the possessed art carries the ember glyph, so a frame that is not a
// possession can never light up red.
func TestCompanionEmberGlyphBelongsToThePossession(t *testing.T) {
	for _, height := range companionHeights() {
		for _, frame := range companionFramesAt(height) {
			if frame.state == companionPossessedState {
				continue
			}
			if strings.Contains(strings.Join(frame.rows, "\n"), string(companionEmberGlyph)) {
				t.Errorf("height %d: the ordinary state %s carries the ember glyph",
					height, companionStateNames[frame.state])
			}
		}
	}
}

// TestCompanionPossessedGlyphRowPaintsOnlyTheEyesInEmber pins that the ember colour
// reaches the eyes and nothing else, and that an ordinary row is left exactly as the
// shared style drew it.
func TestCompanionPossessedGlyphRowPaintsOnlyTheEyesInEmber(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	ember := CompanionEmberStyle.Render(string(companionEmberGlyph))
	rendered := companionGlyphRow(companionPossessedMini)
	if !strings.Contains(rendered, ember) {
		t.Errorf("the possessed row does not paint its eye in the ember colour:\n%q", rendered)
	}
	if got := strings.Count(rendered, ember); got != 2 {
		t.Errorf("the possessed row paints %d ember glyphs, want the two eyes", got)
	}
	if strings.Contains(companionGlyphRow(`(o.o)`), ember) {
		t.Error("an ordinary row was painted with the ember colour")
	}
	if got, want := plainRow(companionGlyphRow(companionPossessedMini)), companionPossessedMini; got != want {
		t.Errorf("the ember styling changed the glyphs: %q, want %q", got, want)
	}
}

// TestCompanionPossessedVolumeDrawsEmberEyesAndHorns pins the volumetric form, which
// is what a true-colour terminal draws: the eyes take the ember tone and the head
// carries the horn tone. The ordinary creature draws neither.
func TestCompanionPossessedVolumeDrawsEmberEyesAndHorns(t *testing.T) {
	size, ok := companionVolumeSizeFor(companionFullHeight)
	if !ok {
		t.Fatalf("height %d is not a volume rung", companionFullHeight)
	}

	hasTone := func(tone companionTone) bool {
		grid := companionVolumeTones(companionPossessedState, 0, companionGaze{}, 0, size)
		for _, row := range grid {
			for _, got := range row {
				if got == tone {
					return true
				}
			}
		}
		return false
	}

	if !hasTone(companionToneEmber) {
		t.Error("the possessed volume draws no ember eye")
	}
	if !hasTone(companionToneHorn) {
		t.Error("the possessed volume draws no horn")
	}
	if got := companionEyeMarkFor(companionPossessedState); got != companionEyeSlit {
		t.Errorf("the possessed eye mark is %d, want the slit", got)
	}

	grid := companionVolumeTones(companionIdleState, 0, companionGaze{}, 0, size)
	for _, row := range grid {
		for _, tone := range row {
			if tone == companionToneEmber || tone == companionToneHorn {
				t.Fatalf("the ordinary creature draws the possessed tone %d", tone)
			}
		}
	}
}

// TestTrainerBossScreenDrawsThePossessedCreature is the user-facing pin: the screen
// the player is looking at renders the turned creature, and the menu does not.
func TestTrainerBossScreenDrawsThePossessedCreature(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	ember := CompanionEmberStyle.Render(string(companionEmberGlyph))

	boss := possessedModel(t, 3)
	if got := boss.renderTrainerBoss(); !strings.Contains(got, ember) {
		t.Errorf("the boss screen does not draw the possessed creature's eyes:\n%s", got)
	}

	menu := possessedModel(t, 3)
	menu.Screen = ScreenTrainerMenu
	if got := menu.renderTrainerMenu(); strings.Contains(got, ember) {
		t.Errorf("the trainer menu draws the possessed creature:\n%s", got)
	}
}
