package tui

// layout is the columns one screen has and how the frame spends them. It is
// computed once per render from the model's width, and every decision about
// where a row ends or where a second column starts is read from it rather than
// re-derived in a renderer, so the numbers a screen is measured against and the
// numbers it draws with cannot drift apart.
//
// The contract, in the order the fields are derived:
//
//	Inner        the room the screen has: contentWidth(m), minus the padding
//	             View() applies to every screen.
//	Composition  the printed body: the room, capped, because a 227-column row
//	             of text is harder to read than an 160-column one and the eye
//	             loses the line's start on the way back.
//	Leading      the left margin that centres the composition, so a terminal
//	             wider than the cap gets two symmetric margins instead of one
//	             void on the right.
//	TwoColumn    whether there is room for a body beside a panel.
//	Left, Right  the two columns, with the gutter between them, filling the
//	             composition exactly: Left + gutter + Right == Composition.
//	RowMeasure   how far a menu row and the rule under it run: the left column
//	             when there are two, otherwise the room capped at the same
//	             reading measure. A row is never a slab of bar across a wide
//	             terminal.
//
// The thresholds and widths are constants here, with a test beside them, rather
// than numbers scattered through the renderer where no one can check them.
const (
	// layoutMaxComposition caps how wide the printed body is allowed to get.
	layoutMaxComposition = 160

	// layoutTwoColumnWidth is the content width at which a body and a panel fit
	// beside each other. Below it there is not enough room for both a readable
	// row and a panel, and the screen stays one column.
	layoutTwoColumnWidth = 120

	// layoutLeftMin and layoutLeftMax bound the left column: wide enough for a
	// menu row to read as one line, and narrow enough to leave the panel real
	// room beside it.
	layoutLeftMin = 56
	layoutLeftMax = 80

	// layoutLeftShare is the left column's share of the composition, as a
	// percentage, before the two bounds above clamp it.
	layoutLeftShare = 45

	// layoutGutter is the blank columns between the two columns, so the panel
	// reads as a separate object rather than as text continuing the body.
	layoutGutter = 2

	// layoutRowMeasureMax is the longest a menu row runs in one column: past it
	// the bar stops being a row and becomes a band of colour across the screen.
	layoutRowMeasureMax = 80
)

type layout struct {
	Inner       int
	Composition int
	Leading     int
	Left        int
	Right       int
	RowMeasure  int
	TwoColumn   bool
}

// layoutFor measures the current screen. It reads only the model's width: the
// render path stays pure, so no layout decision depends on anything but the
// room it was given.
func layoutFor(m Model) layout {
	inner := contentWidth(m)

	l := layout{
		Inner:       inner,
		Composition: min(inner, layoutMaxComposition),
		TwoColumn:   inner >= layoutTwoColumnWidth,
	}
	l.Leading = (inner - l.Composition) / 2

	if l.TwoColumn {
		l.Left = max(layoutLeftMin, min(layoutLeftMax, l.Composition*layoutLeftShare/100))
		l.Right = l.Composition - l.Left - layoutGutter
		l.RowMeasure = l.Left
		return l
	}

	l.Left = l.Composition
	l.RowMeasure = min(inner, layoutRowMeasureMax)
	return l
}
