package diskclean

import (
	"context"
	"path/filepath"
)

// The JVM ecosystem: Gradle's caches and wrapper distributions, Maven's local
// repository, and Coursier's cache.
//
// None of the three has a command that prunes its whole store. Gradle and
// Maven documented no such command, so their stores are removed directly with
// the cost stated on the row; Coursier's `cs clean` only clears part of the
// cache, which is why the row does not use it and the guarded removal does the
// work instead.

// detectGradle finds Gradle's cache and the wrapper's downloaded
// distributions. GRADLE_USER_HOME moves both.
func detectGradle(ctx context.Context, probe Probe, opts DetectOptions) []Item {
	gradleHome := probe.Getenv("GRADLE_USER_HOME")
	named := gradleHome != ""
	if gradleHome == "" {
		gradleHome = filepath.Join(probe.HomeDir, ".gradle")
	}
	paths := existingPaths(probe,
		filepath.Join(gradleHome, "caches"),
		filepath.Join(gradleHome, "wrapper", "dists"),
	)
	program := firstProgram(probe, "gradle")
	if len(paths) == 0 && program == "" {
		return nil
	}

	reason := "the Gradle cache directory exists"
	switch {
	case named && len(paths) > 0:
		reason = "$GRADLE_USER_HOME names " + gradleHome + ", and its caches are there"
	case named:
		reason = "$GRADLE_USER_HOME names " + gradleHome + ", so Gradle is configured to keep its caches there"
	case len(paths) > 0 && program != "":
		reason = "gradle is on PATH and its cache directory exists"
	case len(paths) > 0:
		reason = "the Gradle cache directory exists, though gradle is not on PATH"
	default:
		reason = "gradle is on PATH, and its cache directory is empty or has not been created yet"
	}

	return []Item{{
		ID:        "gradle-caches",
		Tool:      "gradle",
		Title:     "The Gradle caches",
		Detail:    "the dependencies Gradle resolved, the transformed artifacts and the wrapper distributions downloaded per Gradle version. Gradle has no command that prunes them, so the directory is removed; the next build re-resolves and downloads what it needs. A Gradle daemon holding files in the cache can make part of the removal fail, and the result says which part.",
		Category:  CategoryCache,
		Risk:      RiskCareful,
		Reason:    reason,
		Paths:     paths,
		Mechanism: Mechanism{Kind: MechanismRemove},
	}}
}

// detectMaven finds Maven's local repository, the default ~/.m2/repository.
//
// A repository moved with a localRepository in settings.xml is not searched
// for: settings.xml is not parsed, and a store this package cannot establish
// is not offered rather than guessed at.
func detectMaven(ctx context.Context, probe Probe, opts DetectOptions) []Item {
	repository := filepath.Join(probe.HomeDir, ".m2", "repository")
	paths := existingPaths(probe, repository)
	program := firstProgram(probe, "mvn", "mvnw")
	if len(paths) == 0 && program == "" {
		return nil
	}

	reason := "the Maven local repository exists at its default location"
	switch {
	case len(paths) > 0 && program != "":
		reason = "maven is on PATH and its default local repository exists at " + repository
	case len(paths) > 0:
		reason = "the Maven local repository exists at " + repository + ", though maven is not on PATH"
	default:
		reason = "maven is on PATH, and the default local repository has not been created yet"
	}

	return []Item{{
		ID:        "maven-repository",
		Tool:      "maven",
		Title:     "The Maven local repository",
		Detail:    "every artifact Maven downloaded for the projects built on this machine, in the default ~/.m2/repository. Maven's own maintenance goals are project-scoped, so the directory is removed; the next build downloads what its dependencies need. A repository configured elsewhere in settings.xml is not touched.",
		Category:  CategoryDependencies,
		Risk:      RiskCareful,
		Reason:    reason,
		Paths:     paths,
		Mechanism: Mechanism{Kind: MechanismRemove},
	}}
}

// detectCoursier finds Coursier's cache, which Scala builds and the Scala CLI
// fill with downloaded artifacts.
func detectCoursier(ctx context.Context, probe Probe, opts DetectOptions) []Item {
	var cachePath string
	if value := probe.Getenv("COURSIER_CACHE"); value != "" {
		cachePath = value
	} else if opts.Facts.GOOS == "darwin" {
		cachePath = filepath.Join(probe.HomeDir, "Library", "Caches", "Coursier")
	} else {
		cachePath = userCacheDir(probe, opts.Facts, "coursier")
	}
	paths := existingPaths(probe, cachePath)
	program := firstProgram(probe, "cs", "coursier")
	if len(paths) == 0 && program == "" {
		return nil
	}

	reason := "the Coursier cache directory exists"
	if len(paths) > 0 {
		reason = "the Coursier cache directory exists at " + cachePath
	}

	return []Item{{
		ID:        "coursier-cache",
		Tool:      "coursier",
		Title:     "The Coursier cache",
		Detail:    "the JVM artifacts Coursier downloaded and resolved, shared between Scala, sbt and Scala CLI builds. The next build resolves and downloads them again.",
		Category:  CategoryCache,
		Risk:      RiskSafe,
		Reason:    reason,
		Paths:     paths,
		Mechanism: Mechanism{Kind: MechanismRemove},
	}}
}
