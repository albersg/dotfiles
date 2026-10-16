package tui

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// WSLResources is the machine-derived part of a rendered .wslconfig.
//
// A zero field means "omit the key", which is a deliberate answer rather than a
// missing one: WSL then applies its own proportional default, computed from the
// real host by Windows. Omitting a key is always better than writing a limit
// that does not fit the machine, so every failure path in the host detection
// lands here as zeros.
type WSLResources struct {
	MemoryMB   int // 0 means "omit the key"
	Processors int // 0 means "omit the key"
	SwapMB     int // 0 means "omit the key"
}

const (
	bytesPerMiB = 1 << 20

	// wslMemoryStepMB is the granularity memory and swap are rounded down to.
	// It is policy rather than a WSL requirement: whole 512 MiB steps keep the
	// rendered file readable and stop the plan from promising a capacity the
	// rounding would not survive.
	wslMemoryStepMB = 512

	// wslWindowsReserveBytes is what the policy leaves for Windows itself. WSL2
	// runs on top of Windows, so a VM limit of half the host can still starve the
	// desktop on a small machine; the reserve is the floor Windows keeps. It is an
	// absolute amount rather than a proportion because what Windows needs to stay
	// usable does not shrink with the machine.
	wslWindowsReserveBytes = 4 << 30 // 4 GiB

	// wslMinMemoryBytes is the smallest VM limit worth writing. Below 1 GiB the
	// distribution is too cramped to be useful and WSL's own default is better.
	wslMinMemoryBytes = 1 << 30 // 1 GiB

	// wslMemoryFloorDivisor is the floor the plan will not go under, expressed as
	// the divisor of the host it is: half. It equals WSL's own proportional
	// default, so the plan never hands the VM a worse deal than the default WSL
	// would have applied by itself -- including on a host too small to honour the
	// Windows reserve as well, where the floor is what wins and Windows keeps less
	// than wslWindowsReserveBytes.
	wslMemoryFloorDivisor = 2

	// wslMemoryRatioDivisor is the share of the host the VM takes before the
	// reserve is applied, expressed as "leave one part in this many to Windows":
	// two thirds to the VM, one third to Windows.
	wslMemoryRatioDivisor = 3
)

// PlanWSLResources turns host capacities into the values the .wslconfig template
// should render. It is pure: no environment, no filesystem, no processes.
//
// The policy is a ratio, a reserve and a floor, clamped to what the host can
// actually give, and it exists because the VM's memory limit is the only thing
// between one runaway workload and the whole distribution:
//   - memory: two thirds of the host, but never so much that Windows is left
//     below 4 GiB, and never below half the host; rounded down to
//     wslMemoryStepMB, and omitted below wslMinMemoryBytes;
//   - processors: every host logical CPU but one, so Windows keeps one for its
//     own desktop, and at least one when the host reports a single CPU; omitted
//     when the host is unknown;
//   - swap: a quarter of the planned memory, rounded down to the same step, and
//     omitted when that rounds to nothing.
//
// Rounding happens in MiB, because .wslconfig expresses memory in MB. The host
// reports bytes, so the conversion happens once, in integer arithmetic, and no
// fractional MiB is ever produced.
func PlanWSLResources(host system.HostResources) WSLResources {
	plan := WSLResources{}

	if host.LogicalCPUs > 0 {
		plan.Processors = planProcessors(host.LogicalCPUs)
	}

	plan.MemoryMB = planMemoryMB(host.MemoryBytes)
	if plan.MemoryMB > 0 {
		plan.SwapMB = planSwapMB(plan.MemoryMB)
	}

	return plan
}

// planProcessors applies the CPU policy: every logical CPU but one, so Windows
// keeps one for the desktop it is also drawing while the VM still gets the
// parallelism a build needs. The floor is 1 and not 0, because 0 is what the
// template reads as "omit the key", and WSL's own default for an absent key is
// every CPU the host has -- the opposite of the policy.
//
// A host whose count could not be read never reaches here; the caller omits the
// key instead.
func planProcessors(logicalCPUs int) int {
	if logicalCPUs <= 1 {
		return 1
	}
	return logicalCPUs - 1
}

// planMemoryMB applies the memory policy:
//
//	target = max(min(host - host/3, host - 4 GiB), host/2)
//
// The first term is the ratio: the VM takes two thirds of the host. The second is
// the absolute reserve Windows needs to stay usable, which is what decides on a
// host where a third of the memory is still less than 4 GiB. The third is the
// floor, half the host, which is exactly the default WSL applies when the key is
// absent: the plan never hands the VM less than WSL would have chosen by itself.
//
// On a host small enough that the reserve and the floor disagree, the floor wins
// and Windows keeps less than the reserve. That is deliberate. The guard this
// policy is designed around bounds one runaway workload; talking a user with a
// small machine out of the VM WSL would have given them anyway is not a safeguard
// but a regression, and a distribution too small to run a build is its own
// failure mode.
//
// An unknown host (0 bytes) has nothing to compute from, and a target below
// wslMinMemoryBytes is too small to write. Both omit the key, which hands the
// decision back to WSL.
func planMemoryMB(memoryBytes uint64) int {
	if memoryBytes == 0 {
		return 0
	}

	// The floor is the starting value as well as the lower bound, so the
	// arithmetic below can only ever grow it.
	target := memoryBytes / wslMemoryFloorDivisor

	// The reserve is checked before it is subtracted rather than computed and
	// clamped afterwards: MemoryBytes is unsigned, so subtracting the reserve on a
	// host below it would wrap around and report a huge "available" value. Below
	// the reserve the floor above is the whole answer.
	if memoryBytes > wslWindowsReserveBytes {
		// Two thirds, computed as "subtract one third" rather than "multiply by
		// two": the multiplication would overflow a uint64 on any host large
		// enough to make the whole question absurd, and the subtraction cannot.
		ratio := memoryBytes - memoryBytes/wslMemoryRatioDivisor
		if room := memoryBytes - wslWindowsReserveBytes; ratio > room {
			ratio = room
		}
		if ratio > target {
			target = ratio
		}
	}

	if target < wslMinMemoryBytes {
		return 0
	}

	// The division is exact on every host WSL runs on: WSL needs 64-bit Windows,
	// where int holds any plausible VM size.
	return int(roundDownToMemoryStep(target) / bytesPerMiB)
}

// planSwapMB applies the swap policy: a quarter of the planned memory, rounded
// down to the same step as memory. A quarter of the smallest plan is below one
// step, which is the documented "omit" case.
func planSwapMB(memoryMB int) int {
	return memoryMB / 4 / wslMemoryStepMB * wslMemoryStepMB
}

// roundDownToMemoryStep rounds a byte count down to a whole wslMemoryStepMB step.
func roundDownToMemoryStep(bytes uint64) uint64 {
	step := uint64(wslMemoryStepMB) * bytesPerMiB
	return bytes / step * step
}

// ============================================================================
// ONE RENDER, ONE MERGE, ONE WRITER: TWO ENTRIES INTO THE SAME FILE
// ============================================================================
//
// The .wslconfig has two ways in. The installation step applies it as part of a
// normal install, so a freshly installed machine already comes out right; the
// utilities section shows the same values and lets the user change them later.
// Both routes build their content with wslConfigContent and write it with
// writeWSLConfig, and both take the recommended values from PlanWSLResources, so
// there is exactly one calculation and one writer. A host and a pre-existing
// file that are the same produce the same bytes whichever route is used, and
// TestWSLResourceUtilityAndTheInstallerAgreeByteForByte fails the moment a
// second calculation or a second writer appears on either side.

// wslConfigKey returns the key a configuration line sets, lowercased, and
// whether the line sets one at all. A comment (whole-line or inline) is not a
// key, because every managed key is matched by its own name rather than by
// position.
func wslConfigKey(line string) (string, bool) {
	key, _, ok := strings.Cut(line, "=")
	if !ok {
		return "", false
	}
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" || strings.ContainsAny(key, " \t#;") {
		return "", false
	}
	return key, true
}

// wslConfigSectionName returns the section a header line opens, lowercased.
func wslConfigSectionName(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
		return "", false
	}
	return strings.ToLower(strings.TrimSpace(trimmed[1 : len(trimmed)-1])), true
}

// wslManagedEntry is one key the template renders: the section it belongs to,
// its name, and the exact line the render produced for it.
type wslManagedEntry struct {
	section string
	key     string
	line    string
}

// wslManagedEntries lists the keys a rendered template owns, in the template's
// own order. The set is read from the render rather than typed beside it, so a
// key added to dotfiles-wsl/.wslconfig.tmpl becomes managed without a second
// list to keep in step.
func wslManagedEntries(rendered []byte) []wslManagedEntry {
	var entries []wslManagedEntry
	section := ""
	for _, raw := range strings.Split(string(rendered), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if name, ok := wslConfigSectionName(line); ok {
			section = name
			continue
		}
		key, ok := wslConfigKey(line)
		if !ok {
			continue
		}
		entries = append(entries, wslManagedEntry{section: section, key: key, line: line})
	}
	return entries
}

// wslConfigBlock is one [section] of a .wslconfig together with the lines under
// it. The header keeps its original text so the join is exact.
type wslConfigBlock struct {
	section string
	header  string
	lines   []string
}

// splitWSLConfig splits a file into the lines before its first section and one
// block per section. Every line keeps its order and its exact text -- comments,
// blank lines and the user's own keys included -- so joining the result is the
// identity for a file the merge does not change.
func splitWSLConfig(content string) (preamble []string, blocks []wslConfigBlock) {
	for _, line := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
		if name, ok := wslConfigSectionName(line); ok && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			blocks = append(blocks, wslConfigBlock{section: name, header: line})
			continue
		}
		if len(blocks) == 0 {
			preamble = append(preamble, line)
			continue
		}
		blocks[len(blocks)-1].lines = append(blocks[len(blocks)-1].lines, line)
	}
	return preamble, blocks
}

// MergeWSLConfig lays a rendered .wslconfig over the file a user already has.
//
// Only the keys the template renders are managed. Each one is updated in place
// under its own section, and everything else -- the user's own keys, their
// comments, the order of their lines -- is preserved exactly. A managed key the
// template omits is left as it is rather than deleted: an unknown host omits
// memory, processors and swap, and a limit this installer cannot compute must
// never be thrown away.
//
// An empty destination receives the render unchanged, so a first install is byte
// for byte what RenderWSLConfig produced. The merge is pure and deterministic,
// which is what lets the installation route and the utilities route be checked
// against each other byte for byte.
func MergeWSLConfig(existing, rendered []byte) []byte {
	if len(bytes.TrimSpace(existing)) == 0 {
		return rendered
	}
	entries := wslManagedEntries(rendered)
	if len(entries) == 0 {
		return existing
	}

	preamble, blocks := splitWSLConfig(string(existing))
	index := make(map[string]int, len(blocks))
	for i, block := range blocks {
		if _, seen := index[block.section]; !seen {
			index[block.section] = i
		}
	}

	// The managed keys are grouped by section and applied in the template's
	// order, so a key the file is missing lands where a fresh install puts it.
	grouped := map[string][]wslManagedEntry{}
	var sections []string
	for _, entry := range entries {
		if _, seen := grouped[entry.section]; !seen {
			sections = append(sections, entry.section)
		}
		grouped[entry.section] = append(grouped[entry.section], entry)
	}

	for _, section := range sections {
		if section == "" {
			preamble = applyWSLManagedKeys(preamble, grouped[section])
			continue
		}
		if i, ok := index[section]; ok {
			blocks[i].lines = applyWSLManagedKeys(blocks[i].lines, grouped[section])
			continue
		}
		blocks = append(blocks, wslConfigBlock{
			section: section,
			header:  "[" + section + "]",
			lines:   applyWSLManagedKeys(nil, grouped[section]),
		})
		index[section] = len(blocks) - 1
	}

	out := make([]string, 0, len(preamble)+len(blocks))
	out = append(out, preamble...)
	for _, block := range blocks {
		out = append(out, block.header)
		out = append(out, block.lines...)
	}
	return []byte(strings.Join(out, "\n") + "\n")
}

// applyWSLManagedKeys updates the managed keys of one section: the first line
// setting a managed key is replaced in place, a later line setting the same key
// is dropped (the render already decided the value, and two lines for one key
// would leave WSL's last-wins parser in charge), and a key the section does not
// set is appended in the render's order. Lines that set none of the managed keys
// are copied through untouched.
func applyWSLManagedKeys(lines []string, entries []wslManagedEntry) []string {
	rendered := make(map[string]string, len(entries))
	for _, entry := range entries {
		rendered[entry.key] = entry.line
	}

	out := make([]string, 0, len(lines)+len(entries))
	replaced := make(map[string]bool, len(entries))
	for _, line := range lines {
		if key, ok := wslConfigKey(strings.TrimSpace(line)); ok {
			if managed, isManaged := rendered[key]; isManaged {
				if replaced[key] {
					continue
				}
				replaced[key] = true
				out = append(out, managed)
				continue
			}
		}
		out = append(out, line)
	}
	for _, entry := range entries {
		if replaced[entry.key] {
			continue
		}
		out = append(out, entry.line)
	}
	return out
}

// wslConfigTemplate reads the shipped .wslconfig template from a checkout.
func wslConfigTemplate(repoDir string) ([]byte, error) {
	if repoDir == "" {
		return nil, fmt.Errorf("the repository has not been cloned in this run, so %s cannot be read", repoAssetWSLConfig)
	}
	return os.ReadFile(filepath.Join(repoDir, repoAssetWSLConfig))
}

// wslConfigContent renders the repository's .wslconfig for one plan and merges
// it over the file already at dst. It is the single content builder both routes
// use, and it is pure with respect to everything except that file: the same
// checkout, plan and file always produce the same bytes.
func wslConfigContent(repoDir string, plan WSLResources, dst string) ([]byte, error) {
	templateText, err := wslConfigTemplate(repoDir)
	if err != nil {
		return nil, err
	}
	rendered, err := RenderWSLConfig(string(templateText), plan)
	if err != nil {
		return nil, err
	}
	existing, readErr := os.ReadFile(dst)
	if readErr != nil {
		// No file yet, or one this user cannot read. The render is then the whole
		// answer; a file that cannot be read but can be written is reported by the
		// write that follows rather than guessed at here.
		return rendered, nil
	}
	return MergeWSLConfig(existing, rendered), nil
}

// wslConfigContentForHost detects the Windows host, plans the values for it and
// builds the content for dst. The host and the plan come back with the bytes so
// the caller can report where the values came from.
func wslConfigContentForHost(repoDir, dst string) (content []byte, host system.HostResources, plan WSLResources, err error) {
	host = system.DetectHostResources()
	plan = PlanWSLResources(host)
	content, err = wslConfigContent(repoDir, plan, dst)
	return content, host, plan, err
}

// writeWSLConfig writes already-built content to dst through the one writer both
// routes use: an existing destination is backed up first, the write is direct
// when this user may make it and escalates to sudo only when it is refused. It is
// gated on dryRun() exactly as executeStep is, and reports whether it wrote, so a
// dry run leaves no file and no backup behind.
func writeWSLConfig(content []byte, dst, stepID string) (bool, error) {
	if dryRun() {
		SendLog(stepID, "DRY RUN: skipping the write of "+dst)
		return false, nil
	}
	return true, applyArtifactContent(content, dst, stepID)
}

// ParseWSLConfigValues reads the managed values a .wslconfig holds. The three
// keys are only read under [wsl2], which is the section WSL reads them from. A
// key that is absent or that this parser cannot read is 0, the same "not set"
// the plan uses, so the screen never shows a limit the file does not impose.
func ParseWSLConfigValues(content []byte) WSLResources {
	var values WSLResources
	section := ""
	for _, raw := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if name, ok := wslConfigSectionName(line); ok {
			section = name
			continue
		}
		if section != "wsl2" {
			continue
		}
		key, ok := wslConfigKey(line)
		if !ok {
			continue
		}
		_, value, _ := strings.Cut(line, "=")
		switch key {
		case "memory":
			values.MemoryMB = parseWSLMegabytes(value)
		case "swap":
			values.SwapMB = parseWSLMegabytes(value)
		case "processors":
			values.Processors = parseWSLCount(value)
		}
	}
	return values
}

// parseWSLMegabytes reads one of WSL's memory sizes. The template writes "MB",
// "GB" is what most users type, and a bare number is read as MiB, which is the
// unit the template's own suffix stands for. Anything else is 0.
func parseWSLMegabytes(raw string) int {
	value := strings.ToUpper(strings.TrimSpace(strings.SplitN(raw, "#", 2)[0]))
	if value == "" {
		return 0
	}
	multiplier := 1
	switch {
	case strings.HasSuffix(value, "GB"):
		multiplier, value = 1024, strings.TrimSuffix(value, "GB")
	case strings.HasSuffix(value, "MB"):
		value = strings.TrimSuffix(value, "MB")
	case strings.HasSuffix(value, "KB"):
		// Below the smallest step this utility can offer, so it is not a value the
		// screen will show or write.
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < 0 {
		return 0
	}
	return n * multiplier
}

// parseWSLCount reads one of WSL's plain counts.
func parseWSLCount(raw string) int {
	n, err := strconv.Atoi(strings.TrimSpace(strings.SplitN(raw, "#", 2)[0]))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// wslDraftFrom is the value each row starts on: what the file holds today when
// it sets one, and the host's recommendation when it does not. A key that is
// neither is left at 0 -- "not set" -- rather than invented.
func wslDraftFrom(current, plan WSLResources) WSLResources {
	return WSLResources{
		MemoryMB:   pickWSLValue(current.MemoryMB, plan.MemoryMB),
		Processors: pickWSLValue(current.Processors, plan.Processors),
		SwapMB:     pickWSLValue(current.SwapMB, plan.SwapMB),
	}
}

func pickWSLValue(current, recommended int) int {
	if current > 0 {
		return current
	}
	return recommended
}

// wslAdjustValue moves one draft value by one step. Zero means "not set": it is
// left as the file has it, and the first step up from it lands on the step
// itself. Nothing here can reach a negative value, so a row can never plan a
// limit the render would not understand.
func wslAdjustValue(value, step, delta int) int {
	if delta <= 0 {
		if value <= 0 {
			return 0
		}
		if next := value + delta*step; next > 0 {
			return next
		}
		return 0
	}
	if value <= 0 {
		return step
	}
	return value + delta*step
}

// RenderWSLConfig renders the shipped .wslconfig template for one host plan.
//
// It is pure: the same template text and the same plan always produce the same
// bytes, which is what lets the step and the interactive script be checked
// against each other. A template that does not parse, or one that fails while
// executing against the plan, is returned as an error instead of a partially
// written file: a truncated .wslconfig would still be read by WSL.
func RenderWSLConfig(templateText string, res WSLResources) ([]byte, error) {
	tmpl, err := template.New("wslconfig").Parse(templateText)
	if err != nil {
		return nil, err
	}

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, res); err != nil {
		return nil, err
	}

	return rendered.Bytes(), nil
}
