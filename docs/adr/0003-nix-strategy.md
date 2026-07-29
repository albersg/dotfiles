# ADR 0003: Nix Integration — Separate Evaluation Track

## Status

Proposed (2026-07-29)

## Context

Upstream Gentlemant.Dots has a `nix-migration` branch with experimental Nix-based configuration management. The downstream distribution needs to decide whether to:

1. Include Nix integration from the start
2. Exclude it entirely
3. Evaluate it separately and integrate later

## Decision

**Evaluate Nix integration separately.** Do not include it in the initial downstream release. Track upstream's `nix-migration` branch and evaluate for a future release.

## Rationale

- Nix integration is experimental upstream — not yet on `main`
- Adding Nix would increase the initial scope and delay the first downstream release
- The existing non-Nix installation path (Homebrew, direct download) works for all target platforms
- Nix evaluation requires understanding of upstream's Nix approach, which is still evolving

## Consequences

- Downstream v0.1.0 ships without Nix support
- Nix users must use the non-Nix install path or wait for a future release
- A future ADR will document the Nix integration decision when the evaluation is complete

## Evaluation Criteria

When evaluating Nix for inclusion:

- Does upstream's `nix-migration` branch reach main and stabilize?
- Does Nix provide a materially better experience (reproducibility, rollback) for the target audience?
- What is the maintenance burden of supporting both Nix and non-Nix install paths?

## References

- Upstream Nix branch: `https://github.com/Gentleman-Programming/Gentleman.Dots/tree/nix-migration`
- ADR 0001: Downstream Bootstrap Strategy
