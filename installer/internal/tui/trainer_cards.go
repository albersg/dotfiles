package tui

import (
	"fmt"
	"strconv"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/albersg/dotfiles/installer/internal/tui/trainer"
)

// ============================================================================
// THE TRAINER — the reference grammar
// ============================================================================
//
// The trainer's six screens draw the same objects the rest of the redesign does
// (see components.go): a card whose title is a filled chip on its top border with
// a muted caption at the right and a rule under the pair, a row of glyph, title,
// muted subtitle and chevron with the row under the cursor filled, a numbered
// step, a fact row whose values share a column, and a small stat box. Nothing
// here is a second vocabulary: the code window is a card, the module list is a
// card of rows, the module under the cursor is a card of steps and facts, and the
// result screens are a card of facts.
//
// The trainer's identity is what the shapes carry, not what the grammar changes:
// the code window keeps its line-number gutter and its cursor and selection
// marks, the status ladder keeps its glyph-plus-word, the lives keep their
// hearts, the session's score, streak, combo and countdowns keep the header chips
// they had, and the glyphs are one cell each and drawn from the Nerd Font or the
// plain marks a terminal without one still draws. No emoji.
//
// Two shapes, one preference. The cards are drawn where the frame holds two
// columns and the rows the body needs; the 80x24 floor and every frame too short
// for the card keep the compact body the screens shipped with (view.go), which is
// what the trainer's own frame guard measures. The gates read the frame only --
// never the cursor, never a message -- so a screen cannot change shape while the
// player walks it.

const (
	// trainerCardChrome is the rows a grammar card spends on its own edges and the
	// rule under its chip: the top border, the rule and the closing border.
	trainerCardChrome = 3

	// trainerExerciseCardChrome is what the exercise screen's right column spends
	// besides the mission text: the card's own three rows, the Answer divider and
	// its line, and the feedback area the screen holds whether or not there is
	// anything to say. The mission gets the body's rows minus this.
	trainerExerciseCardChrome = trainerCardChrome + 2 + trainerMessageRows

	// trainerMenuCardMinRows is the least the menu's body needs before the cards
	// are worth drawing: the list's own card with one row and the detail card's
	// shortest form.
	trainerMenuCardMinRows = trainerCardChrome + 4

	// trainerResultCardMinRows is the least the result screens need: the summary
	// card with its rule and its first group.
	trainerResultCardMinRows = trainerCardChrome + 3
)

// trainerCardsFit reports whether a trainer body of need rows is worth drawing as
// cards. It is the one gate every trainer card body asks, so the six screens
// cannot disagree about what "room for the grammar" means, and it reads the frame
// and not the model's state.
func trainerCardsFit(l layout, budget, need int) bool {
	return l.TwoColumn && budget >= need
}

// trainerBodyBudget is the rows a trainer body may spend: the frame minus the
// rows the chrome and the footer take, and above the companion's own block when
// the terminal is tall enough for the ladder to draw one. It is the same
// arithmetic trainerWideTextBudget reads, so a card body and the code window it
// holds cannot be measured against two different frames.
func (m Model) trainerBodyBudget(footerRows int) int {
	rows := installerBodyRows(m.Height, footerRows)
	if m.Height > trainerFloorHeight {
		rows -= m.companionHeightNow()
	}
	return max(1, rows)
}

// trainerCardMessageRows is the feedback area: the message wrapped to the width
// it is drawn at and held to the fixed number of rows every trainer screen
// reserves, so a message arriving mid-exercise cannot move what is below it.
func (m Model) trainerCardMessageRows(width int) []string {
	rows := make([]string, trainerMessageRows)
	for i, row := range wrapText(m.TrainerMessage, width, trainerMessageRows) {
		rows[i] = WarningStyle.Render(row)
	}
	return rows
}

// trainerSelectedModule is the module under the menu's cursor, or false when the
// menu has no rows.
func (m Model) trainerSelectedModule() (trainer.ModuleInfo, bool) {
	if m.TrainerCursor < 0 || m.TrainerCursor >= len(m.TrainerModules) {
		return trainer.ModuleInfo{}, false
	}
	return m.TrainerModules[m.TrainerCursor], true
}

// trainerModuleStates is the four questions every module row asks of the profile.
// It reads the recorded progress through the stats' own predicates, so the menu's
// row, the menu's detail card and the boss gate cannot disagree about a module.
func (m Model) trainerModuleStates(module trainer.ModuleInfo) (unlocked, bossDefeated, bossReady, practiceReady bool) {
	if m.TrainerStats == nil {
		return false, false, false, false
	}
	return m.TrainerStats.IsModuleUnlocked(module.ID),
		m.TrainerStats.IsBossDefeated(module.ID),
		m.TrainerStats.IsBossReady(module.ID),
		m.TrainerStats.IsPracticeReady(module.ID)
}

// trainerModuleReading is a module's recorded progress and the practice reading
// derived from it. A module that has never been opened reports a nil progress, so
// asking the menu for it does not create the record a later save would persist.
func (m Model) trainerModuleReading(module trainer.ModuleInfo) (*trainer.ModuleProgress, trainer.PracticeStats) {
	if m.TrainerStats == nil {
		return nil, trainer.GetPracticeStatsForModule(module.ID, nil)
	}
	progress := m.TrainerStats.ModuleProgressOrNil(module.ID)
	return progress, trainer.GetPracticeStatsForModule(module.ID, progress)
}

// trainerModuleCardRow draws one module as a grammar row: the state's glyph in the
// gutter, the module's name, the state word and the module's own commands on the
// muted line under it, and the two meters trailing at the row's right edge. The
// chevron says Enter opens the module, which is what every row of this list does.
func (m Model) trainerModuleCardRow(module trainer.ModuleInfo, selected bool, width int) []string {
	unlocked, bossDefeated, bossReady, practiceReady := m.trainerModuleStates(module)
	progress, practice := m.trainerModuleReading(module)

	return cardRowLines(cardRow{
		Icon:     trainerModuleStatusGlyph(unlocked, bossDefeated, bossReady, practiceReady),
		Title:    module.Name,
		Subtitle: trainerModuleStatusWord(unlocked, bossDefeated, bossReady, practiceReady) + " · " + module.Description,
		Trailing: trainerModuleMeterText(progress, practice),
		Chevron:  true,
	}, width, selected)
}

// trainerMenuCardBody draws the module menu as the reference does: the list in the
// card at the left, windowed around the cursor when the frame cannot hold every
// module, and the module under the cursor in the card at the right. It reports
// false when the frame cannot hold the two cards, which is the caller's signal to
// draw the compact body.
func (m Model) trainerMenuCardBody(l layout, budget int) ([]string, bool) {
	if len(m.TrainerModules) == 0 || !trainerCardsFit(l, budget, trainerMenuCardMinRows) {
		return nil, false
	}

	rows := m.TrainerModules
	content := cardContentWidth(l.Left)
	heights := make([]int, len(rows))
	for i := range heights {
		heights[i] = 2 // every module row carries its state and commands under its name
	}

	// The window is measured in the lines the rows will draw, with the notes that
	// say what it hid and the feedback area under the card counted in the same
	// budget, so a list longer than the card degrades out loud instead of ending
	// where the card ends and pushing the legend off the frame.
	room := max(1, budget-trainerCardChrome-trainerMessageRows)
	start, end := utilitiesRowWindow(heights, m.TrainerCursor, room, false)
	if start > 0 {
		room--
	}
	if end < len(rows) {
		room--
	}
	start, end = utilitiesRowWindow(heights, m.TrainerCursor, room, false)

	var list []string
	if start > 0 {
		list = append(list, MutedStyle.Render(fmt.Sprintf("↑ %d more above", start)))
	}
	for i := start; i < end; i++ {
		list = append(list, m.trainerModuleCardRow(rows[i], i == m.TrainerCursor, content)...)
	}
	if end < len(rows) {
		list = append(list, MutedStyle.Render(fmt.Sprintf("↓ %d more below", len(rows)-end)))
	}
	if len(list)+trainerCardChrome > budget {
		return nil, false
	}

	cleared := 0
	if m.TrainerStats != nil {
		cleared = len(m.TrainerStats.BossesDefeated)
	}
	left := card("Modules", fmt.Sprintf("%d modules · %d cleared", len(rows), cleared), l.Left, list)
	right := m.trainerModuleCard(l.Right)

	body := composeColumns(left, right, l)
	body = append(body, m.trainerCardMessageRows(l.Composition)...)
	return body, len(body) <= budget
}

// trainerModuleCard is the selected module as its own card: the module's name on
// the chip, its state word as the caption, the module's progression ladder as the
// grammar's numbered steps and the practice reading under a divider of its own.
// Every number is the recorded progress or the practice stats derived from it.
func (m Model) trainerModuleCard(width int) []string {
	module, ok := m.trainerSelectedModule()
	if !ok {
		return nil
	}
	unlocked, bossDefeated, bossReady, practiceReady := m.trainerModuleStates(module)
	content := cardContentWidth(width)

	groups := []cardGroup{{Lines: numberedStepLines(m.trainerModuleLadder(module), content)}}
	if readings := m.trainerModuleReadings(module, content); len(readings) > 0 {
		groups = append(groups, cardGroup{Title: "Practice", Lines: readings})
	}
	return cardGroups(module.Name,
		trainerModuleStatusWord(unlocked, bossDefeated, bossReady, practiceReady), width, groups)
}

// trainerModuleLadder is one module's progression as the three rungs the player
// walks: the lessons, the practice that opens when they are passed, and the boss
// that opens on the practice accuracy. Each rung's subtitle is the module's own
// recorded count, so the ladder is a reading of the profile and not a diagram.
func (m Model) trainerModuleLadder(module trainer.ModuleInfo) []numberedStep {
	progress, practice := m.trainerModuleReading(module)

	lessonsDone, lessonsTotal := 0, practice.TotalExercises
	if progress != nil {
		lessonsDone, lessonsTotal = progress.LessonsCompleted, progress.LessonsTotal
	}
	if lessonsTotal == 0 {
		lessonsTotal = practice.TotalExercises
	}

	practiceStep := "not played yet"
	if progress != nil && progress.PracticeAttempts > 0 {
		practiceStep = fmt.Sprintf("%.0f%% over %d answers", progress.PracticeAccuracy*100, progress.PracticeAttempts)
		if due := progress.ReviewsDue(module.ID, time.Now()); due > 0 {
			word := "reviews"
			if due == 1 {
				word = "review"
			}
			practiceStep += fmt.Sprintf(" · %d mastered %s due", due, word)
		}
	}

	bossState := "locked"
	switch {
	case progress != nil && progress.BossDefeated:
		bossState = "defeated"
	case progress != nil && m.trainerModuleBossReady(module):
		bossState = "unlocked"
	}

	return []numberedStep{
		{Title: "Lessons", Subtitle: fmt.Sprintf("%d/%d completed", lessonsDone, lessonsTotal)},
		{Title: "Practice", Subtitle: practiceStep},
		{Title: "Boss", Subtitle: module.BossName + " · " + bossState},
	}
}

// trainerModuleBossReady reports whether the module's boss gate is open, which is
// the only rung the ladder's word adds to the profile's own predicates.
func (m Model) trainerModuleBossReady(module trainer.ModuleInfo) bool {
	_, _, bossReady, _ := m.trainerModuleStates(module)
	return bossReady
}

// trainerModuleReadings is the detail card's practice group: the accuracy the boss
// gate is measured against and the exercises the module misses most, each with the
// recorded wrong count. Both are omitted until there is a recorded attempt, so a
// fresh module states its ladder and no invented percentage.
func (m Model) trainerModuleReadings(module trainer.ModuleInfo, width int) []string {
	progress, practice := m.trainerModuleReading(module)

	var facts []iconFact
	if accuracy := trainerAccuracyCardValue(progress, module.ID, time.Now()); accuracy != "" {
		facts = append(facts, iconFact{Icon: "\uf080", Label: "Accuracy", Value: accuracy})
	}
	if weakest := trainerWeakEntryText(progress, practice); weakest != "" {
		facts = append(facts, iconFact{Icon: "\uf071", Label: "Weakest", Value: weakest})
	}
	return iconFactLines(facts, width)
}

// trainerAccuracyCardValue is the practice accuracy the boss gate is measured
// against, for the grammar's fact row: the percentage, the attempts behind it and
// the mastered exercises that have come due. It states no percentage for a module
// with no recorded attempt, and it is the same reading the compact line states --
// without the label, because the fact row owns its label.
func trainerAccuracyCardValue(progress *trainer.ModuleProgress, module trainer.ModuleID, now time.Time) string {
	if progress == nil || progress.PracticeAttempts == 0 {
		return ""
	}
	value := fmt.Sprintf("%.0f%% over %d answers", progress.PracticeAccuracy*100, progress.PracticeAttempts)
	if due := progress.ReviewsDue(module, now); due > 0 {
		word := "reviews"
		if due == 1 {
			word = "review"
		}
		value += fmt.Sprintf(" · %d mastered %s due", due, word)
	}
	return value
}

// trainerCodeCardCaption says which rows of the exercise the code card is showing,
// or nothing when it is showing all of them. It is the windowed label's own
// sentence, so the scroll keys and the card cannot disagree about the range.
func trainerCodeCardCaption(offset, rows, total int) string {
	if rows >= total {
		return ""
	}
	last := min(offset+rows, total)
	return fmt.Sprintf("rows %d-%d of %d", offset+1, last, total)
}

// trainerExerciseCardBody draws a lesson, a practice run or a boss step as the
// reference does: the code window in the card at the left with its rows as the
// caption, and the mission in the card at the right with the answer under a
// divider of its own and the feedback area under it. title is the screen's own
// word for the mission block, so the boss keeps "Challenge" and the exercise
// screens keep "Mission".
func (m Model) trainerExerciseCardBody(exercise *trainer.Exercise, title string, feedback lipgloss.Style, l layout) []string {
	mission, codeRows := m.trainerWideTextBudget(exercise, l)
	right := cardContentWidth(l.Right)
	viewport := m.trainerCodeViewport(exercise, codeRows, cardContentWidth(l.Left)-trainerGutterWidth)

	left := card("Code", trainerCodeCardCaption(viewport.offset, viewport.rows, len(exercise.Code)), l.Left, viewport.render())

	missionLines := make([]string, len(mission))
	for i, row := range mission {
		missionLines[i] = InfoStyle.Render(row)
	}
	groups := []cardGroup{
		{Lines: missionLines},
		{Title: "Answer", Lines: []string{KeyStyle.Render(tailToWidth(m.trainerAnswer(), right))}},
	}
	column := cardGroups(title, exercise.ID, l.Right, groups)
	column = append(column, m.trainerFeedbackRowsWidth(feedback, right)...)

	return composeColumns(left, column, l)
}

// renderBufferCardLines renders a buffer as the numbered lines the exercise screen
// draws, measured against the card's content so the block cannot reach the border.
// A line wider than the card is cut and marked, exactly as the compact body cuts
// it, so a buffer is never silently clipped.
func renderBufferCardLines(buffer []string, width int) []string {
	rows := make([]string, len(buffer))
	for i, line := range buffer {
		rows[i] = MutedStyle.Render(fmt.Sprintf("%2d │ ", i+1)) +
			CodeStyle.Render(truncate(line, max(1, width-trainerGutterWidth)))
	}
	return rows
}

// trainerResultCardBody draws the result screen as one card: what was answered as
// the grammar's fact rows, the verdict's message, the exercise's explanation and
// the buffer the answer produced (and the one it was expected to, when it was
// rejected). It reports false where the frame cannot hold the card, which is the
// floor's compact body.
func (m Model) trainerResultCardBody(l layout, budget int) ([]string, bool) {
	if !trainerCardsFit(l, budget, trainerResultCardMinRows) {
		return nil, false
	}

	width := l.Composition
	content := cardContentWidth(width)
	var groups []cardGroup

	exercise := (*trainer.Exercise)(nil)
	if m.TrainerGameState != nil {
		exercise = m.TrainerGameState.CurrentExercise
	}
	if exercise != nil {
		// The card's caption names the exercise; these rows name what it belongs to
		// and what it pays, so the card states each datum once.
		facts := []iconFact{
			{Icon: "\uf02d", Label: "Module", Value: string(exercise.Module)},
			{Icon: "\uf005", Label: "Level", Value: strconv.Itoa(exercise.Level)},
		}
		if lines := iconFactLines(facts, content); len(lines) > 0 {
			groups = append(groups, cardGroup{Lines: lines})
		}
	}

	message := wrapText(m.TrainerMessage, content, 0)
	if len(message) == 0 {
		message = []string{""}
	}
	lines := make([]string, len(message))
	for i, row := range message {
		lines[i] = InfoStyle.Render(row)
	}
	groups = append(groups, cardGroup{Lines: lines})

	if exercise != nil && exercise.Explanation != "" {
		explanation := wrapText(exercise.Explanation, content, 0)
		lines := make([]string, len(explanation))
		for i, row := range explanation {
			lines[i] = MutedStyle.Render(row)
		}
		groups = append(groups, cardGroup{Title: "Explanation", Lines: lines})
	}

	if exercise != nil && exercise.BufferVerified && m.TrainerValidation != nil && m.TrainerValidation.BufferVerified {
		if !m.TrainerValidation.IsCorrect {
			groups = append(groups, cardGroup{
				Title: "Expected buffer",
				Lines: renderBufferCardLines(m.TrainerValidation.TargetBuffer, content),
			})
		}
		groups = append(groups, cardGroup{
			Title: "Resulting buffer",
			Lines: renderBufferCardLines(m.TrainerValidation.ActualBuffer, content),
		})
	}

	board := cardGroups("Summary", m.trainerResultCaption(exercise), width, groups)
	body := centeredBody(board, l)
	return body, len(body) <= budget
}

// trainerResultCaption is the result card's caption: the exercise that was
// answered. The card's fact rows name the module and the level, so the caption is
// the identifier alone and no datum is stated twice.
func (m Model) trainerResultCaption(exercise *trainer.Exercise) string {
	if exercise == nil {
		return ""
	}
	return exercise.ID
}

// trainerBossResultCardBody draws the fight's result as the reference does: the
// run's own numbers in the grammar's stat boxes and the verdict in the card under
// them, with the fight's measured facts as fact rows. It reports false where the
// frame cannot hold both, which is the floor's compact body.
func (m Model) trainerBossResultCardBody(l layout, budget int) ([]string, bool) {
	if !trainerCardsFit(l, budget, trainerResultCardMinRows) {
		return nil, false
	}

	width := l.Composition
	content := cardContentWidth(width)
	state := m.TrainerGameState
	boss := (*trainer.BossExercise)(nil)
	if state != nil {
		boss = state.CurrentBoss
	}

	var body []string
	if boxes := m.trainerBossResultBoxes(width); len(boxes) > 0 {
		body = append(body, boxes...)
		body = append(body, "")
	}

	sentences := m.trainerBossResultSentences(state, boss)
	groups := []cardGroup{{Lines: sentences}}
	if facts := trainerBossResultFacts(state, boss, content, m.trainerBossBestTime); len(facts) > 0 {
		groups = append(groups, cardGroup{Title: "Fight", Lines: facts})
	}
	if m.TrainerMessage != "" {
		lines := wrapText(m.TrainerMessage, content, 0)
		for i, row := range lines {
			lines[i] = MutedStyle.Render(row)
		}
		groups = append(groups, cardGroup{Title: "Message", Lines: lines})
	}

	caption := ""
	if boss != nil {
		caption = boss.Name
	}
	board := cardGroups("Summary", caption, width, groups)
	body = append(body, centeredBody(board, l)...)
	return body, len(body) <= budget
}

// trainerBossResultBoxes is the fight's result in the grammar's stat boxes: the
// run's total score and the bosses it has cleared, and the module's best fight
// where the profile holds one. A box the model cannot fill is left out rather than
// drawn with a guess, and the whole row is left out when the boxes cannot hold
// their own labels.
func (m Model) trainerBossResultBoxes(width int) []string {
	if m.TrainerStats == nil {
		return nil
	}
	boxes := []statBox{
		{Label: "Score", Value: strconv.Itoa(m.TrainerStats.TotalScore), Tone: BrandStyle},
		{Label: "Bosses", Value: fmt.Sprintf("%d/%d", len(m.TrainerStats.BossesDefeated), len(trainer.GetAllModules())), Tone: SuccessStyle},
	}
	if state := m.TrainerGameState; state != nil {
		if best := m.trainerBossBestTime(state.CurrentModule); best > 0 {
			boxes = append(boxes, statBox{Label: "Best", Value: trainerFightDuration(best), Tone: MeterStyle})
		}
	}
	return statBoxLines(boxes, width)
}

// trainerBossResultSentences is the verdict in the screen's own words: which boss
// was beaten and with how many lives, or that it won this time. The unlock claim
// reads NextModule, so the screen never promises a module the corpus does not have.
func (m Model) trainerBossResultSentences(state *trainer.GameState, boss *trainer.BossExercise) []string {
	var out []string
	if m.TrainerLastCorrect {
		if state != nil && boss != nil {
			out = append(out, InfoStyle.Render("You defeated "+boss.Name+"!"))
			out = append(out, InfoStyle.Render(fmt.Sprintf("Lives remaining: %d/%d %s",
				state.BossLives, boss.Lives, trainerLivesGlyphs(state.BossLives, boss.Lives))))
		}
		out = append(out, SuccessStyle.Render(trainerBonusMark+" +500 bonus points!"))

		nextModuleExists := false
		if state != nil {
			_, nextModuleExists = trainer.NextModule(state.CurrentModule)
		}
		if nextModuleExists {
			out = append(out, SuccessStyle.Render(trainerUnlockMark+" Next module unlocked!"))
		} else {
			out = append(out, SuccessStyle.Render(trainerUnlockMark+" No modules left — you have cleared them all!"))
		}
		return out
	}

	if boss != nil {
		out = append(out, MutedStyle.Render(boss.Name+" wins this time..."))
	}
	return append(out, InfoStyle.Render("Keep practicing and try again!"))
}

// The marks the boss result's celebration lines lead with. Both are geometric
// glyphs rather than emoji: a terminal without an emoji font draws the star and
// the diamond, and the words after them carry the sentence either way. They are
// the trainer's own marks -- the star for the bonus, the diamond for the module
// the win unlocked -- and one cell each, so they cannot push the line off the
// card.
const (
	trainerBonusMark  = "★"
	trainerUnlockMark = "◆"
)

// trainerBossResultFacts is the fight's measured detail as fact rows: how long it
// took and the module's best. Both are omitted for a fight that never started, so
// a row appears only where there is a measurement behind it.
func trainerBossResultFacts(state *trainer.GameState, boss *trainer.BossExercise, width int, best func(trainer.ModuleID) time.Duration) []string {
	if state == nil || boss == nil {
		return nil
	}
	elapsed := state.BossFightElapsed()
	if elapsed <= 0 {
		return nil
	}

	facts := []iconFact{
		{Icon: "\uf017", Label: "Fight time", Value: trainerFightDuration(elapsed)},
	}
	if best := best(state.CurrentModule); best > 0 {
		facts = append(facts, iconFact{Icon: "\uf091", Label: "Best", Value: trainerFightDuration(best)})
	}
	return iconFactLines(facts, width)
}
