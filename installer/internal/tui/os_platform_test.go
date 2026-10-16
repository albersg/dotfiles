package tui

import (
	"runtime"
	"strings"
	"testing"
)

// TestThePlatformFlagOffersExactlyWhatTheScreenOffers holds the vocabulary of
// --os to the vocabulary of the setup screen's first row. The two are derived
// rather than copied -- the row's answer key comes from questionAnswerKey, which
// reads the option label the question screen's own handler reads -- so a row the
// screen gains cannot leave the flag behind, and the flag cannot offer a platform
// no row can record.
func TestThePlatformFlagOffersExactlyWhatTheScreenOffers(t *testing.T) {
	options := (Model{}).osOptions()
	if len(options) == 0 {
		t.Fatal("the operating system question offers nothing, so this guard proves nothing")
	}

	var fromScreen []string
	for _, option := range options {
		key := questionAnswerKey(ScreenOSSelect, option)
		if key == "" {
			t.Errorf("the option %q records no answer key, so a row and the flag cannot agree about it", option)
			continue
		}
		for _, seen := range fromScreen {
			if seen == key {
				t.Errorf("the option %q records the answer key %q a previous option already recorded", option, key)
			}
		}
		fromScreen = append(fromScreen, key)
	}

	fromFlag := SupportedOperatingSystems()
	if len(fromFlag) == 0 {
		t.Fatal("--os offers nothing, so this guard proves nothing")
	}
	seen := map[string]bool{}
	for _, value := range fromFlag {
		if seen[value] {
			t.Errorf("--os offers %q twice", value)
		}
		seen[value] = true
	}

	if strings.Join(fromFlag, ",") != strings.Join(fromScreen, ",") {
		t.Errorf("--os offers %v and the screen records %v: the two lists are one list", fromFlag, fromScreen)
	}
}

// TestThePlatformValueIsRefusedWhenTheScreenCannotOfferIt is the reason --os is
// worth having at all. An empty value means "ask the machine", every value the
// screen can record is accepted, and anything else is refused with the accepted
// values named -- never quietly read as Linux, which is the defect the
// non-interactive path already had by overwriting the choice.
func TestThePlatformValueIsRefusedWhenTheScreenCannotOfferIt(t *testing.T) {
	if err := ValidateOperatingSystem(""); err != nil {
		t.Errorf("an absent platform is the machine's answer and must be accepted: %v", err)
	}
	for _, value := range SupportedOperatingSystems() {
		if err := ValidateOperatingSystem(value); err != nil {
			t.Errorf("the screen can record %q and the flag refuses it: %v", value, err)
		}
	}

	for _, refused := range []string{"wsl", "windows", "Mac", "darwin", " "} {
		err := ValidateOperatingSystem(refused)
		if err == nil {
			t.Errorf("--os=%s is not something the screen can offer and the flag accepted it", refused)
			continue
		}
		for _, value := range SupportedOperatingSystems() {
			if !strings.Contains(err.Error(), value) {
				t.Errorf("the refusal of %q does not name %q, so it does not teach what is accepted: %v", refused, value, err)
			}
		}
	}
}

// TestTheChosenPlatformSurvivesTheNonInteractivePath is S8's half of the flag:
// the platform the caller chose is the platform the run works from, and only an
// absent one falls back to detecting the machine.
func TestTheChosenPlatformSurvivesTheNonInteractivePath(t *testing.T) {
	for _, chosen := range SupportedOperatingSystems() {
		if got := PlatformFor(UserChoices{OS: chosen}); got != chosen {
			t.Errorf("the caller chose %q and the path decided %q", chosen, got)
		}
	}

	detected := "linux"
	if runtime.GOOS == "darwin" {
		detected = "mac"
	}
	if got := PlatformFor(UserChoices{}); got != detected {
		t.Errorf("with no platform chosen the path decided %q, want the machine's %q", got, detected)
	}
}

// TestTheChosenPlatformReachesThePlan holds the flag's other end: the one builder
// schedules the steps of the platform the choices name, not of the host the
// process happens to run on. The Apple developer tools are the step that makes
// the difference visible, and it is asked for here with no host detected at all,
// which is exactly the case a flag exists for.
func TestTheChosenPlatformReachesThePlan(t *testing.T) {
	mac := &Model{Choices: UserChoices{OS: "mac"}}
	linux := &Model{Choices: UserChoices{OS: "linux"}}

	if _, ok := planStepByID(planFor(mac.planOptions()), "xcode"); !ok {
		t.Errorf("the plan for the platform that needs the Apple developer tools does not schedule them: %v", planIDList(planFor(mac.planOptions())))
	}
	if _, ok := planStepByID(planFor(linux.planOptions()), "xcode"); ok {
		t.Errorf("the plan for Linux schedules the Apple developer tools: %v", planIDList(planFor(linux.planOptions())))
	}
	if _, ok := planStepByID(planFor(linux.planOptions()), "deps"); !ok {
		t.Errorf("the plan for Linux does not schedule the dependency step: %v", planIDList(planFor(linux.planOptions())))
	}
}
