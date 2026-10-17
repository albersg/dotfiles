package diskclean

import (
	"testing"
)

// TestParseToolBytesReadsWhatTheEnginesPrint pins the size format both engines
// use, including the percentage docker appends and the space podman prints.
// An unreadable size is rejected rather than turned into a zero: a store shown
// as empty when nobody could read its number is the one lie the screen cannot
// afford.
func TestParseToolBytesReadsWhatTheEnginesPrint(t *testing.T) {
	cases := []struct {
		value string
		want  int64
		ok    bool
	}{
		{"1.23GB", 1_230_000_000, true},
		{"500MB (40%)", 500_000_000, true},
		{"1.2 kB", 1_200, true},
		{"12.5 MiB", 13_107_200, true},
		{"0B", 0, true},
		{"1024", 1024, true},
		{"1GiB", 1 << 30, true},
		{"", 0, false},
		{"some", 0, false},
		{"-5MB", 0, false},
		{"1.5XB", 0, false},
	}
	for _, c := range cases {
		got, ok := ParseToolBytes(c.value)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("ParseToolBytes(%q) = %d, %v; want %d, %v", c.value, got, ok, c.want, c.ok)
		}
	}
}

// TestParseSystemDFReadsEveryShapeTheEnginesPrint pins the tolerant reader: the
// JSON object per line that `--format {{json .}}` produces, podman's array, a
// JSON object keyed by store, and the plain table. A shape that cannot be read
// yields nothing rather than a guess.
func TestParseSystemDFReadsEveryShapeTheEnginesPrint(t *testing.T) {
	perLine := `{"Type":"Images","TotalCount":"5","Active":"2","Size":"1.23GB","Reclaimable":"500MB (40%)"}
{"Type":"Local Volumes","TotalCount":"2","Active":"0","Size":"100MB","Reclaimable":"100MB (100%)"}`
	array := `[{"Type":"Containers","Total":3,"Size":"10MB","Reclaimable":"5MB"},{"Type":"Build Cache","Total":1,"Size":"2GB","Reclaimable":"2GB"}]`
	keyed := `{"Images":{"total":2,"size":"900MB","reclaimable":"300MB"},"Local Volumes":{"total":1,"size":"1GB","reclaimable":"1GB"}}`
	table := "TYPE            TOTAL     ACTIVE    SIZE      RECLAIMABLE\n" +
		"Images          5         2         1.23GB    500MB (40%)\n" +
		"Local Volumes   2         0         100MB     100MB (100%)\n" +
		"Build Cache     4         0         2GB       2GB (100%)\n"

	cases := []struct {
		name   string
		output string
		want   map[string]int64
	}{
		{"per-line json", perLine, map[string]int64{"images": 500_000_000, "volumes": 100_000_000}},
		{"array json", array, map[string]int64{"containers": 5_000_000, "build-cache": 2e9}},
		{"keyed json", keyed, map[string]int64{"images": 300_000_000, "volumes": 1e9}},
		{"plain table", table, map[string]int64{"images": 500_000_000, "volumes": 100_000_000, "build-cache": 2e9}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stores := ParseSystemDF([]byte(c.output))
			got := make(map[string]int64, len(stores))
			for _, store := range stores {
				got[store.Kind] = store.Reclaimable
			}
			if len(got) != len(c.want) {
				t.Fatalf("read %v, want %v", got, c.want)
			}
			for kind, want := range c.want {
				if got[kind] != want {
					t.Errorf("%s reclaimable is %d, want %d", kind, got[kind], want)
				}
			}
		})
	}
}

// TestParseSystemDFRefusesWhatItCannotRead pins the honest half: output that is
// not one of the known shapes produces no stores, and the row that would have
// shown a number is simply absent.
func TestParseSystemDFRefusesWhatItCannotRead(t *testing.T) {
	for _, output := range []string{"", "   ", "not a table at all", "{broken", "1 2 3"} {
		if stores := ParseSystemDF([]byte(output)); len(stores) != 0 {
			t.Errorf("ParseSystemDF(%q) read %d stores, want none", output, len(stores))
		}
	}
}

// TestParseSystemDFTableKeepsAMultiWordStoreOneField pins the column rule: a
// store named "Local Volumes" stays one row and its numbers stay its own.
func TestParseSystemDFTableKeepsAMultiWordStoreOneField(t *testing.T) {
	table := "TYPE            TOTAL     ACTIVE    SIZE      RECLAIMABLE\n" +
		"Local Volumes   2         0         100MB     100MB (100%)\n"

	stores := ParseSystemDF([]byte(table))
	if len(stores) != 1 {
		t.Fatalf("the table produced %d stores, want one", len(stores))
	}
	store := stores[0]
	if store.Label != "Local Volumes" {
		t.Errorf("the label is %q, want Local Volumes", store.Label)
	}
	if store.Kind != "volumes" {
		t.Errorf("the kind is %q, want volumes", store.Kind)
	}
	if store.Count != 2 || store.Size != 100_000_000 || store.Reclaimable != 100_000_000 {
		t.Errorf("the store is %+v, want 2 objects of 100MB with 100MB reclaimable", store)
	}
}
