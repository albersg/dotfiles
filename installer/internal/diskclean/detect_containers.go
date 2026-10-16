package diskclean

import (
	"context"
	"fmt"
)

// The container engines. Their stores are never walked and never removed from
// the filesystem: an engine owns its images, its containers, its volumes and
// its build cache, and the only way this utility touches any of them is the
// engine's own prune command.
//
// The numbers come from `system df`, which is the engine's own accounting.
// A store whose reclaimable bytes are zero contributes no row: there is
// nothing to free, and a row offering to free nothing is noise.

// containerStore describes one store as a row: how the engine names it, what
// kind it is, and the command that reclaims it.
type containerStore struct {
	kind      string
	tool      string
	id        string
	title     string
	category  Category
	risk      Risk
	detail    string
	objects   string
	pruneArgs []string
}

// containerStoresFor builds the rows an engine's disk usage justifies. The
// rows are the engine's own stores, in the order a person would look at them.
func containerStoresFor(program string, tool string, stores []toolStore) []Item {
	byKind := make(map[string]toolStore, len(stores))
	for _, store := range stores {
		byKind[store.Kind] = store
	}

	var items []Item
	for _, spec := range containerRowSpecs(tool) {
		store, ok := byKind[spec.kind]
		if !ok || store.Reclaimable <= 0 {
			continue
		}
		items = append(items, Item{
			ID:              spec.id,
			Tool:            tool,
			Title:           spec.title,
			Detail:          spec.detail,
			Category:        spec.category,
			Risk:            spec.risk,
			Reason:          engineReason(tool, program, store),
			Mechanism:       Mechanism{Kind: MechanismTool, Program: program, Args: spec.pruneArgs},
			VolatileObjects: []string{spec.objects},
			Size: Size{
				// The size is the engine's own reclaimable figure, which is
				// also what the command is expected to free. It is a tool
				// report and is labelled as one.
				Logical:  store.Reclaimable,
				Physical: store.Reclaimable,
				Source:   SourceTool,
			},
		})
	}
	return items
}

// engineReason is the evidence for a container row: the engine is on PATH and
// its own disk-usage report says how much is reclaimable.
func engineReason(tool, program string, store toolStore) string {
	objects := ""
	if store.Count > 0 {
		objects = fmt.Sprintf(" across %d object(s)", store.Count)
	}
	return fmt.Sprintf("%s is on PATH and `%s system df` reports %s reclaimable in its %s store%s",
		tool, program, humanToolBytes(store.Reclaimable), store.Label, objects)
}

// humanToolBytes formats a tool-reported size for the reason line, using the
// same decimal units the tools print so the row and its reason agree.
func humanToolBytes(bytes int64) string {
	switch {
	case bytes >= 1e12:
		return fmt.Sprintf("%.1f TB", float64(bytes)/1e12)
	case bytes >= 1e9:
		return fmt.Sprintf("%.1f GB", float64(bytes)/1e9)
	case bytes >= 1e6:
		return fmt.Sprintf("%.1f MB", float64(bytes)/1e6)
	case bytes >= 1e3:
		return fmt.Sprintf("%.1f kB", float64(bytes)/1e3)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// containerRowSpecs is the fixed table of rows an engine may contribute. The
// volumes row is the delicate one: a volume is data a container wrote, it is
// not reproducible from a manifest, and the prune command only removes volumes
// no container references -- which is not the same as volumes nobody needs.
func containerRowSpecs(tool string) []containerStore {
	return []containerStore{
		{
			kind: "images", tool: tool,
			id: tool + "-images", title: "Unused " + tool + " images",
			category: CategoryImages, risk: RiskCareful,
			detail: "the images no container is running from right now, plus the intermediate layers left behind. " +
				"An image that was built locally from a Dockerfile is not stored anywhere else: removing it means " +
				"building or pulling it again. `image prune -a` is what runs; it keeps the images running " +
				"containers use.",
			objects:   "images not used by a running container",
			pruneArgs: []string{"image", "prune", "-a", "-f"},
		},
		{
			kind: "containers", tool: tool,
			id: tool + "-containers", title: "Stopped " + tool + " containers",
			category: CategoryImages, risk: RiskCareful,
			detail: "the containers that have exited and were not removed. A stopped container still holds its " +
				"writable layer -- files its process wrote that are not in an image -- and pruning it discards " +
				"them. `container prune -f` is what runs; running containers are not touched.",
			objects:   "stopped containers",
			pruneArgs: []string{"container", "prune", "-f"},
		},
		{
			kind: "build-cache", tool: tool,
			id: tool + "-build-cache", title: "The " + tool + " build cache",
			category: CategoryBuild, risk: RiskSafe,
			detail: "the layers and partial results the engine kept from earlier builds. The next build recreates " +
				"what it needs; the cache only makes a repeated build faster.",
			objects:   "build cache entries",
			pruneArgs: []string{"builder", "prune", "-f"},
		},
		{
			kind: "volumes", tool: tool,
			id: tool + "-volumes", title: tool + " volumes",
			category: CategoryVolumes, risk: RiskDelicate,
			detail: "the named and anonymous volumes no container currently references. A volume holds data a " +
				"container wrote -- a database, an upload directory, state a service needs -- and none of it can " +
				"be rebuilt from an image. `volume prune -f` removes the unreferenced ones, and this utility " +
				"never preselects a volume: choosing one is a decision about data, and it is yours.",
			objects:   "volumes no container references",
			pruneArgs: []string{"volume", "prune", "-f"},
		},
	}
}

// detectDocker asks the Docker engine for its own accounting and turns it into
// rows. A Docker that is installed but cannot be reached -- a stopped daemon,
// a missing socket -- contributes nothing: there is no store to measure and no
// command that would work.
func detectDocker(ctx context.Context, probe Probe, opts DetectOptions) []Item {
	program := firstProgram(probe, "docker")
	if program == "" {
		return nil
	}
	output, err := runTool(ctx, probe, opts, program, "system", "df", "--format", "{{json .}}")
	if err != nil || len(ParseSystemDF(output)) == 0 {
		// Older engines print the table rather than JSON, and the table is
		// read as it is. A failure here means the engine answered nothing
		// usable, and no row is invented for it.
		output, err = runTool(ctx, probe, opts, program, "system", "df")
		if err != nil {
			return nil
		}
	}
	return containerStoresFor(program, "docker", ParseSystemDF(output))
}

// detectPodman does the same for Podman, whose store is separate from
// Docker's even when the docker command is a podman alias.
func detectPodman(ctx context.Context, probe Probe, opts DetectOptions) []Item {
	program := firstProgram(probe, "podman")
	if program == "" {
		return nil
	}
	output, err := runTool(ctx, probe, opts, program, "system", "df", "--format", "json")
	if err != nil || len(ParseSystemDF(output)) == 0 {
		output, err = runTool(ctx, probe, opts, program, "system", "df")
		if err != nil {
			return nil
		}
	}
	return containerStoresFor(program, "podman", ParseSystemDF(output))
}
