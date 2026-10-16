// Package diskclean finds what the development stores on this machine actually
// hold, measures what could be reclaimed, and removes only what the user has
// seen and approved.
//
// The package is deliberately free of any terminal or interface code, and of
// any dependency beyond the standard library: the screen in internal/tui is an
// adapter over the values here, which is what lets the measurement and the
// deletion protections be exercised against temporary trees rather than against
// the developer's own machine.
//
// Three rules shape everything below.
//
//   - A finding is offered only with the evidence that found it. Every Item
//     carries the Reason it was detected, and a detector that cannot establish
//     presence says so instead of guessing.
//   - A size is measured, never assumed. Filesystem stores are walked; a store
//     whose owner reports its own accounting (the container engines) is asked,
//     and the source of the number is kept beside it.
//   - Nothing is removed by this package on its own. Clean runs a plan the
//     caller built from an explicit selection, and every direct removal passes
//     the guard again at the moment it unlinks.
package diskclean
