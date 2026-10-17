package diskclean

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The guard is the last thing between a plan and the filesystem, and it is
// deliberately the most boring code in the package: every rule it enforces is
// a path that must not be removed and a reason why.
//
// It exists because "the detector produced this path" is not the same claim as
// "this path may be removed now". A detector builds a path from a home
// directory and a tool's documented store name; between the moment the user
// previewed it and the moment it is unlinked, that path can be replaced by a
// link, become a mount point, or turn out to sit inside a repository. So the
// guard checks the path twice: once when the plan is built, which is what the
// preview shows, and once immediately before each removal, which is the check
// that actually decides.

// Approval is the verdict of one Check: the path, the root it was approved
// under, and the identity the path had when it was approved. Verify re-reads
// the identity before a removal, which is how a path swapped between the
// preview and the unlink is refused rather than removed.
type Approval struct {
	// Path is the approved path, canonical.
	Path string
	// Root is the approved root the path was found under.
	Root string
	// Device and Inode are the file identity at approval time. On a platform
	// that does not expose them they are zero, and identity cannot be checked.
	Device uint64
	Inode  uint64
	// Dir reports whether the approved path is a directory.
	Dir bool
}

// Guard decides what may be removed. It is a value: the lists are built once
// per scan and never mutated afterwards.
type Guard struct {
	probe Probe
	// approved are the roots removals may happen under: the store paths of the
	// items being cleaned, and the project directories the user added.
	approved []string
	// blocked are exact roots that may not be removed, and whose ancestors may
	// not be removed either -- removing an ancestor would delete them.
	blocked []string
	// protected are zones where neither the zone nor anything inside it may be
	// removed. They are the configuration and credential directories, which no
	// cache detector should ever produce.
	protected []string
}

// NewGuard builds the guard for one plan. approved are the paths a removal may
// happen under, normally the items' own store paths and any project root the
// user added; anything not under one of them is refused.
//
// Every approved root is canonicalized, and one that turns out to be a blocked
// or protected path is dropped rather than trusted: an approval that only
// exists because a detector was wrong is not an approval.
func NewGuard(probe Probe, approved []string) Guard {
	guard := Guard{probe: probe}
	// The lists are built from the home directory as the environment names it,
	// while every path this guard is asked about is canonical, so the rules are
	// canonicalized here too. One directory can have two names -- macOS
	// resolves /var to /private/var, and a home on another volume is the same
	// shape -- and comparing the two forms leaves the home root and the
	// configuration zones unblocked without a word.
	guard.blocked = canonicalPaths(probe, DefaultBlockedRoots(probe))
	guard.protected = canonicalPaths(probe, DefaultProtectedZones(probe))

	for _, root := range approved {
		canonical := canonicalPath(probe, root)
		if canonical == "" {
			continue
		}
		if guard.blockedReason(canonical) != "" || guard.protectedReason(canonical) != "" {
			continue
		}
		guard.approved = appendUnique(guard.approved, canonical)
	}
	return guard
}

// ApprovedRoots lists the roots that survived construction, in the order they
// were offered. It is what a preview prints so the user can see the scope that
// was actually approved.
func (g Guard) ApprovedRoots() []string {
	return append([]string(nil), g.approved...)
}

// DefaultBlockedRoots are the paths that are never a removal target: the
// filesystem root, the system directories, the home directory itself, and the
// well-known credential and configuration files and directories inside it.
//
// The rule for a blocked root is that neither it nor any ancestor of it may be
// removed. Removing `/home` would remove a user's home; removing `$HOME` would
// remove the credentials below it.
func DefaultBlockedRoots(probe Probe) []string {
	roots := []string{
		"/", "/bin", "/boot", "/dev", "/etc", "/home", "/lib", "/lib64", "/media",
		"/mnt", "/opt", "/proc", "/root", "/run", "/sbin", "/srv", "/sys", "/tmp",
		"/usr", "/var",
	}
	if probe.HomeDir != "" {
		roots = append(roots, probe.HomeDir)
	}
	return roots
}

// DefaultProtectedZones are the zones inside a home directory where nothing is
// removed at all: configuration, credentials and key material. A cache store
// never lives in one of them, so a path that does is a detector bug and is
// refused with that reason rather than trusted.
func DefaultProtectedZones(probe Probe) []string {
	home := probe.HomeDir
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, ".ssh"),
		filepath.Join(home, ".gnupg"),
		filepath.Join(home, ".config"),
		filepath.Join(home, ".aws"),
		filepath.Join(home, ".kube"),
		filepath.Join(home, ".password-store"),
		filepath.Join(home, ".local", "share", "keyrings"),
		filepath.Join(home, ".gitconfig"),
		filepath.Join(home, ".git-credentials"),
		filepath.Join(home, ".netrc"),
		filepath.Join(home, ".npmrc"),
		filepath.Join(home, ".pypirc"),
		filepath.Join(home, ".docker", "config.json"),
	}
}

// Check decides whether a path may be removed now, and returns the approval
// that a later Verify compares against.
func (g Guard) Check(path string) (Approval, error) {
	if path == "" {
		return Approval{}, fmt.Errorf("the path is empty")
	}
	if !filepath.IsAbs(path) {
		return Approval{}, fmt.Errorf("%s is not an absolute path, so its scope cannot be established", path)
	}
	clean := filepath.Clean(path)
	if clean != path {
		return Approval{}, fmt.Errorf("%s is not a canonical path; %s is what would be removed, and an ambiguous path is never removed", path, clean)
	}
	if clean == string(os.PathSeparator) {
		return Approval{}, fmt.Errorf("the filesystem root is never removed")
	}
	// The rules below compare this path against lists that are canonical, so it
	// is canonicalized once and every comparison uses that form. The name the
	// caller passed stays the name that would be removed: removing it removes
	// the same object, and the scope check further down resolves it as well.
	canonical := canonicalPath(g.probe, clean)
	if canonical == "" {
		canonical = clean
	}
	// A repository and its history are never a cache: no removal may touch a
	// .git directory or anything that would contain one. A store can reach a
	// repository's metadata through a link, so both names are read for it.
	for _, candidate := range []string{clean, canonical} {
		for _, element := range strings.Split(candidate, string(os.PathSeparator)) {
			if element == ".git" {
				return Approval{}, fmt.Errorf("%s is inside a Git repository's metadata; a repository is never removed by this utility", clean)
			}
		}
	}

	info, err := g.probe.Lstat(clean)
	if err != nil {
		if os.IsNotExist(err) {
			return Approval{}, fmt.Errorf("%s does not exist, so there is nothing to remove", clean)
		}
		return Approval{}, fmt.Errorf("%s could not be inspected: %v", clean, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		// Removing a link would remove the name and free nothing, while the
		// store behind it stayed; and a link whose target is elsewhere is
		// exactly how a removal escapes its approved scope.
		return Approval{}, fmt.Errorf("%s is a symbolic link: removing it would free nothing and could point anywhere, so it is not removed through this name", clean)
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return Approval{}, fmt.Errorf("%s is neither a directory nor a regular file, so it is not a store this utility removes", clean)
	}

	if reason := g.blockedReason(canonical); reason != "" {
		return Approval{}, fmt.Errorf("%s is not removable: %s", clean, reason)
	}
	if reason := g.protectedReason(canonical); reason != "" {
		return Approval{}, fmt.Errorf("%s is not removable: %s", clean, reason)
	}

	// A path can be lexically inside the scope and still resolve outside it
	// when one of its ancestors is a link. The resolved path has to be inside
	// the scope as well, which is the check that would catch a cache directory
	// symlinked to another disk.
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil {
		return Approval{}, fmt.Errorf("%s could not be resolved: %v", clean, err)
	}
	resolvedRoot := g.approvingRoot(resolved)
	if resolvedRoot == "" {
		return Approval{}, fmt.Errorf("%s resolves to %s, which is outside the approved scope, so it is not removed", clean, resolved)
	}
	root := g.approvingRoot(clean)
	if len(resolvedRoot) > len(root) {
		// The narrowest scope that contains the path is the one a preview
		// should name.
		root = resolvedRoot
	}

	// A path that is itself a mount point is another filesystem: removing it
	// would either fail or empty a disk that belongs to something else.
	if parent, parentErr := g.probe.Lstat(filepath.Dir(clean)); parentErr == nil {
		if device, parentDevice := deviceOf(info), deviceOf(parent); device != 0 && parentDevice != 0 && device != parentDevice {
			return Approval{}, fmt.Errorf("%s is a mount point: it is the root of another filesystem, so it is not removed from here", clean)
		}
	}

	identity := g.probe.identity(clean, info)
	return Approval{Path: clean, Root: root, Device: identity.device, Inode: identity.inode, Dir: info.IsDir()}, nil
}

// Verify re-checks a path immediately before it is removed: the scope checks run
// again, the path has to be the same kind of object, and it has to be the same
// object the preview approved. A path that was replaced after the preview -- with
// a link, another directory, or something under a different root -- is refused
// here.
//
// The identity it compares is the platform's own device and inode pair. That is
// the cheapest check that exists and it closes the window for every ordinary
// case, but it has one honest limit: a filesystem that recycles inode numbers
// can hand a directory recreated at the same path the very same pair, and then
// the swap is invisible to this check. The window is between the preview and the
// removal, and only a process that can write inside the approved scope can race
// there at all. Everything else about the removal is still re-established here:
// the scope, the blocklist, the kind of object, and the refusal to unlink a
// symlink leaf or a mount point.
func (g Guard) Verify(approval Approval) error {
	current, err := g.Check(approval.Path)
	if err != nil {
		return fmt.Errorf("%s can no longer be removed: %w", approval.Path, err)
	}
	if current.Dir != approval.Dir {
		return fmt.Errorf("%s is no longer the same kind of object that was measured and approved", approval.Path)
	}
	if approval.Device != 0 || approval.Inode != 0 {
		if current.Device != approval.Device || current.Inode != approval.Inode {
			return fmt.Errorf("%s is not the object that was measured and approved: it was replaced between the preview and the removal, so nothing is removed", approval.Path)
		}
	}
	return nil
}

// blockedReason returns why a path may not be removed because of a blocked
// root, or an empty string. The path is refused when it is the blocked root
// itself or an ancestor of it: removing the ancestor would remove the blocked
// root along with it.
func (g Guard) blockedReason(path string) string {
	for _, root := range g.blocked {
		if samePath(path, root) {
			return root + " is a system or home root and is never removed"
		}
		if under(path, root) {
			return "it contains " + root + ", which is never removed"
		}
	}
	return ""
}

// protectedReason returns why a path may not be removed because it is inside a
// protected zone, or an empty string.
func (g Guard) protectedReason(path string) string {
	for _, zone := range g.protected {
		if samePath(path, zone) || under(zone, path) {
			return zone + " holds configuration or credentials and is never removed by this utility"
		}
	}
	return ""
}

// approvingRoot names the approved root a path was found under, or an empty
// string when there is none.
func (g Guard) approvingRoot(path string) string {
	best := ""
	for _, root := range g.approved {
		if !samePath(path, root) && !under(root, path) {
			continue
		}
		// The longest match is the narrowest scope, which is the one a preview
		// should name.
		if len(root) > len(best) {
			best = root
		}
	}
	return best
}

// canonicalPaths canonicalizes a list of rules. A rule that cannot be stated
// in the same form as the paths it judges is dropped: this guard refuses what
// it cannot compare, and it never keeps an approval it cannot justify.
func canonicalPaths(probe Probe, paths []string) []string {
	list := make([]string, 0, len(paths))
	for _, path := range paths {
		if canonical := canonicalPath(probe, path); canonical != "" {
			list = appendUnique(list, canonical)
		}
	}
	return list
}

// canonicalPath resolves a path as far as it exists and leaves the rest as it
// is, so a store that has not been created yet still has a canonical name. It
// returns an empty string for a path that cannot be canonicalized.
func canonicalPath(probe Probe, path string) string {
	if path == "" || !filepath.IsAbs(path) {
		return ""
	}
	clean := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(clean)
	if err == nil {
		return resolved
	}
	// The path does not exist yet, or an ancestor is not resolvable: resolve
	// the deepest ancestor that does exist and rejoin the rest.
	remainder := ""
	current := clean
	for {
		parent := filepath.Dir(current)
		if parent == current {
			return clean
		}
		remainder = filepath.Join(filepath.Base(current), remainder)
		current = parent
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			return filepath.Join(resolved, remainder)
		}
	}
}

// samePath reports whether two canonical paths name the same location.
func samePath(left, right string) bool { return left != "" && left == right }

// under reports whether path is strictly inside root.
func under(root, path string) bool {
	if root == "" || path == "" || root == path {
		return false
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if relative == "." {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}
