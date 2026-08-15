# Maintainers and Ownership

Audience: contributors, security reporters, and release reviewers who need the
current governance capacity rather than aspirational role assignments.

## Current Capacity

The repository currently records one CODEOWNERS account, `@aatuh`. This file
does not prove GitHub repository, release, package-registry, or private security
advisory access. It also does not claim an independent reviewer, backup release
manager, or a second security contact. Those authority facts require the
authenticated acceptance and dry-run evidence tracked by `EXT-010`.

| Role | Current local record | Activation requirement |
| --- | --- | --- |
| Project lead and documentation triage | `@aatuh` in `.github/CODEOWNERS` | Repository access is verified externally when a protected action is needed. |
| Release manager | No independently verified assignment | Accepted scope and a successful protected release dry run. |
| Backup release manager | Vacant | Accepted scope, release capability, and independent verification dry run. |
| Private security-advisory contact | See `SECURITY.md`; no second verified contact is recorded here | Private advisory access verified by EXT-010. |
| Module owners | Ownership routes below; not a grant of repository authority | Named maintainer acceptance plus the access needed for the role. |

Do not infer that a GitHub username in this file or CODEOWNERS has accepted a
role, has production credentials, or can publish a release.

## Module and Risk-Area Routes

| Surface | Local owner route | Release/support owner role |
| --- | --- | --- |
| Core root packages and public API | `/httpx/`, `/binding/`, `/middleware/`, `/endpoints/`, `/routecontracts/`, `/specs/` | Core API team |
| V5 CLI and generator | `/cmd/api-toolkit/`, `/contrib/cmd/api-toolkit/`, `/cmd/api-toolkit/` | CLI team |
| PostgreSQL and Redis adapters | `/contrib/adapters/` and future `/adapters/postgres/`, `/adapters/redis/` | Adapter team |
| Provider and observability adapters | `/contrib/integrations/`, `/contrib/middleware/` and future domain modules | Provider/observability owners |
| Optional runtime features | future `/runtime/` | Runtime team |
| Workflows, release evidence, and repository settings | `/.github/`, `/scripts/`, `/docs/release-*` | Release and security roles |
| Security and disclosure | `/SECURITY.md`, `/docs/security*`, `/docs/threat-model.md` | Security contacts |

The routes describe review ownership, not a claim that future directories or
modules have been created. ADR 0002 defines the planned module boundaries.

## Expectations and Conflicts

An active maintainer must keep their accepted scope current, disclose conflicts
on changes they review or release, avoid approving their own branch-protection
exception, and use the private security process for vulnerabilities. There is
no 24/7 support commitment. Inactivity, unavailable access, or an undisclosed
conflict pauses the affected role rather than weakening protected-release or
security controls.

Review and release activity is evidence-based: a maintainer records the ticket,
verification, and final commit or release evidence for work they approve. A
candidate becomes active only after they accept a documented scope and the
required external access is verified. Security and release roles require a
separate person whenever independent approval is required.

## Role Changes, Emeritus Status, and Removal

1. Propose a role change in a tracked governance issue without including
   credentials, private advisory details, or personal contact data.
2. Record accepted scope, conflict disclosure, module/risk routes, and the
   access checks still required. Grant the least privilege needed through the
   relevant service; never transfer secrets in Git or issues.
3. A maintainer may become emeritus voluntarily or after sustained inactivity.
   Emeritus status preserves attribution but removes active review, release, and
   security obligations and access as appropriate.
4. Remove active ownership when scope is withdrawn, access is no longer
   appropriate, a conflict cannot be managed, or the person asks to step down.
   Reassign or explicitly leave the route unstaffed; do not silently imply
   coverage.
5. Follow the handover and 90-day succession process in
   [docs/governance.md](docs/governance.md#maintainer-succession-and-unavailability)
   when an authorized transfer is possible. A public request never grants
   repository, release, or private-security authority.

## Verification Boundary

`make github-governance-check` can inspect available authenticated GitHub
settings, but only EXT-010 records the required human acceptance, independent
release dry run, and private-advisory access evidence. Until that ticket is
complete, the project remains a single-maintainer repository for governance and
release-risk decisions.
