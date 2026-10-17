package diskclean

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// This file reads the numbers a tool prints about its own stores. Those numbers
// are not measurements by this package, and they are never presented as such:
// Size.Source records that they came from the owner, which is the only honest
// way to show a container engine's accounting -- its store is not a directory
// this utility may walk, let alone remove.

// toolStore is one store a tool reports in its disk-usage output, normalized
// across the shapes docker and podman print.
type toolStore struct {
	// Kind is one of the normalized kinds: images, containers, volumes,
	// build-cache.
	Kind string
	// Label is the tool's own name for it.
	Label string
	// Count is how many objects the store holds.
	Count int
	// Size is what the store occupies, Reclaimable what the tool says could be
	// freed now.
	Size        int64
	Reclaimable int64
}

// normalizeStoreKind maps a tool's label to this package's kind. An unknown
// label is kept normalized-but-unknown, so a future store the tool adds is not
// silently attached to a kind this package already handles.
func normalizeStoreKind(label string) string {
	lower := strings.ToLower(strings.TrimSpace(label))
	switch {
	case strings.Contains(lower, "image"):
		return "images"
	case strings.Contains(lower, "container"):
		return "containers"
	case strings.Contains(lower, "volume"):
		return "volumes"
	case strings.Contains(lower, "build"):
		return "build-cache"
	default:
		return lower
	}
}

// ParseToolBytes reads a size a tool printed: "1.23GB", "500MB (40%)",
// "1.2 kB", "0B", "12.5 MiB". It returns the byte count and whether the string
// was understood at all; an unreadable size is never turned into a zero.
func ParseToolBytes(value string) (int64, bool) {
	text := strings.TrimSpace(value)
	if text == "" {
		return 0, false
	}
	// Docker appends the percentage in parentheses: "500MB (40%)".
	if index := strings.Index(text, "("); index >= 0 {
		text = strings.TrimSpace(text[:index])
	}
	// Spaces between the number and the unit are what podman prints.
	number := text
	unit := ""
	for i, r := range text {
		if (r >= '0' && r <= '9') || r == '.' {
			continue
		}
		number, unit = text[:i], strings.TrimSpace(text[i:])
		break
	}
	if number == "" {
		return 0, false
	}
	amount, err := strconv.ParseFloat(number, 64)
	if err != nil || amount < 0 {
		return 0, false
	}

	scale := float64(1)
	switch strings.ToLower(unit) {
	case "", "b", "byte", "bytes":
		scale = 1
	case "kb":
		scale = 1e3
	case "mb":
		scale = 1e6
	case "gb":
		scale = 1e9
	case "tb":
		scale = 1e12
	case "pb":
		scale = 1e15
	case "kib":
		scale = 1 << 10
	case "mib":
		scale = 1 << 20
	case "gib":
		scale = 1 << 30
	case "tib":
		scale = 1 << 40
	case "pib":
		scale = 1 << 50
	default:
		return 0, false
	}
	return int64(amount * scale), true
}

// tableSplit separates the columns of a disk-usage table, which are aligned
// with runs of spaces rather than a single separator.
var tableSplit = regexp.MustCompile(`[ \t]{2,}`)

// ParseSystemDF reads the disk-usage output of docker or podman. It accepts
// every shape those commands produce: one JSON object per line, a JSON array,
// a JSON object keyed by store kind, and the plain table. The first shape that
// parses wins, and output that nothing parses is an empty result rather than an
// error: the caller shows the tool as present but unmeasurable, and no number
// is invented for it.
func ParseSystemDF(output []byte) []toolStore {
	text := strings.TrimSpace(string(output))
	if text == "" {
		return nil
	}
	if stores := parseSystemDFJSON(text); len(stores) > 0 {
		return stores
	}
	return parseSystemDFTable(text)
}

// parseSystemDFJSON tries the JSON shapes in turn.
func parseSystemDFJSON(text string) []toolStore {
	// One object per line, which is what `--format {{json .}}` prints.
	if strings.HasPrefix(text, "{") && strings.Contains(text, "\n") {
		var stores []toolStore
		failed := false
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var object map[string]any
			if err := json.Unmarshal([]byte(line), &object); err != nil {
				failed = true
				break
			}
			if store, ok := storeFromObject(object); ok {
				stores = append(stores, store)
			}
		}
		if !failed && len(stores) > 0 {
			return stores
		}
	}

	// A JSON array of objects, which is podman's `--format json`.
	var array []map[string]any
	if err := json.Unmarshal([]byte(text), &array); err == nil && len(array) > 0 {
		var stores []toolStore
		for _, object := range array {
			if store, ok := storeFromObject(object); ok {
				stores = append(stores, store)
			}
		}
		if len(stores) > 0 {
			return stores
		}
	}

	// A JSON object keyed by store kind, which some podman versions print.
	var keyed map[string]map[string]any
	if err := json.Unmarshal([]byte(text), &keyed); err == nil && len(keyed) > 0 {
		var stores []toolStore
		for label, object := range keyed {
			if object == nil {
				continue
			}
			object["Type"] = label
			if store, ok := storeFromObject(object); ok {
				stores = append(stores, store)
			}
		}
		if len(stores) > 0 {
			return stores
		}
	}

	// A single JSON object: the table as one object.
	var single map[string]any
	if err := json.Unmarshal([]byte(text), &single); err == nil && len(single) > 0 {
		if store, ok := storeFromObject(single); ok {
			return []toolStore{store}
		}
	}
	return nil
}

// storeFromObject reads one store out of a decoded JSON object, accepting the
// field names both tools have used for the same fact.
func storeFromObject(object map[string]any) (toolStore, bool) {
	label := stringField(object, "type", "kind", "name")
	if label == "" {
		return toolStore{}, false
	}
	store := toolStore{Kind: normalizeStoreKind(label), Label: label}
	store.Count = intField(object, "totalcount", "total", "count")
	store.Size = bytesField(object, "size", "diskusage", "disksize")
	store.Reclaimable = bytesField(object, "reclaimable", "reclaimablesize")
	return store, true
}

// stringField returns the first field whose name matches, case-insensitively.
func stringField(object map[string]any, names ...string) string {
	for _, name := range names {
		for key, value := range object {
			if strings.EqualFold(key, name) {
				switch typed := value.(type) {
				case string:
					return typed
				case float64:
					return strconv.FormatFloat(typed, 'f', -1, 64)
				}
			}
		}
	}
	return ""
}

// intField returns the first matching field as a whole number.
func intField(object map[string]any, names ...string) int {
	for _, name := range names {
		for key, value := range object {
			if !strings.EqualFold(key, name) {
				continue
			}
			switch typed := value.(type) {
			case float64:
				return int(typed)
			case string:
				if n, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
					return n
				}
			}
		}
	}
	return 0
}

// bytesField returns the first matching field as a byte count, reading both
// "1.23GB" and a plain number.
func bytesField(object map[string]any, names ...string) int64 {
	for _, name := range names {
		for key, value := range object {
			if !strings.EqualFold(key, name) {
				continue
			}
			switch typed := value.(type) {
			case float64:
				return int64(typed)
			case string:
				if bytes, ok := ParseToolBytes(typed); ok {
					return bytes
				}
			}
		}
	}
	return 0
}

// tableCellIsSize reports whether a table cell is a byte size rather than a
// count. A count is a bare number; a size carries a unit (with the percentage
// docker appends ignored).
func tableCellIsSize(field string) bool {
	head := field
	if index := strings.Index(head, "("); index >= 0 {
		head = head[:index]
	}
	unit := false
	for _, r := range head {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			unit = true
			break
		}
	}
	if !unit {
		return false
	}
	_, ok := ParseToolBytes(field)
	return ok
}

// parseSystemDFTable reads the default table output:
//
//	TYPE            TOTAL     ACTIVE    SIZE      RECLAIMABLE
//	Images          5         2         1.23GB    500MB (40%)
//	Local Volumes   2         0         100MB     100MB (100%)
//
// The columns are aligned with runs of spaces, and a store whose name is two
// words -- Local Volumes -- has to stay one field, which is why the table is
// not split on single spaces.
func parseSystemDFTable(text string) []toolStore {
	var stores []toolStore
	for index, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := tableSplit.Split(strings.TrimSpace(line), -1)
		// The header names the columns; a first line that starts with TYPE is
		// skipped, and so is any line too short to carry a size.
		if index == 0 && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(fields[0])), "TYPE") {
			continue
		}
		if len(fields) < 3 {
			continue
		}
		store := toolStore{Label: strings.TrimSpace(fields[0])}
		store.Kind = normalizeStoreKind(store.Label)
		if store.Kind == "" {
			continue
		}
		var sizes []int64
		for _, field := range fields[1:] {
			field = strings.TrimSpace(field)
			// A table cell is a byte size only when it carries a unit: the
			// columns before it are whole-number counts, and reading "5" as
			// five bytes is exactly the mistake a table parser makes.
			if tableCellIsSize(field) {
				if bytes, ok := ParseToolBytes(field); ok {
					sizes = append(sizes, bytes)
				}
				continue
			}
			if store.Count == 0 {
				if count, err := strconv.Atoi(field); err == nil {
					store.Count = count
				}
			}
		}
		if len(sizes) > 0 {
			store.Size = sizes[0]
		}
		if len(sizes) > 1 {
			store.Reclaimable = sizes[1]
		}
		stores = append(stores, store)
	}
	return stores
}
