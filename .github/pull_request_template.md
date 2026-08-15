# Summary

Describe the change and why it is needed.

# Backlog ticket

- [ ] Ticket ID: `ABC-123`
- [ ] One backlog ticket only; unrelated changes were split out.

# Final commit

- [ ] PR title uses conventional commit syntax.
- [ ] Final squash commit body will include `Refs: <ticket-id>`.
- [ ] Breaking changes use `!` in the subject and include a `BREAKING CHANGE:`
      footer with the migration impact and replacement.

# Tests and verification

- [ ] Added or updated tests for changed behavior.
- [ ] Ran the narrowest relevant check.
- [ ] Ran broader checks when the change touches generated files, scripts,
      package docs, or repo-wide contracts.
- [ ] Commands and results are included below.

```text
<command and result>
```

# Documentation

- [ ] Updated README/docs/API examples where behavior or usage changed.
- [ ] Updated release notes or compatibility docs when public behavior changed.

# Compatibility impact

**Compatibility classification:** select one.

- [ ] No public effect.
- [ ] Additive API.
- [ ] Behavioral change.
- [ ] Deprecation.
- [ ] Breaking change.
- [ ] New stable exported identifiers have doc comments, inventory entries,
      examples or exact exception rows, and release notes.
- [ ] Contrib drift is documented where relevant.

# Security impact

**Security classification:** select one.

- [ ] No new trust boundary or sensitive data handling.
- [ ] New or changed trust boundary is tested and documented.

# Dependency impact

**Dependency classification:** select one.

- [ ] No dependency, license, or supply-chain impact.
- [ ] Dependency, license, or supply-chain impact is documented and reviewed.

# Generated-file impact

- [ ] No generated files changed.
- [ ] Generated files were regenerated and included in this PR.

# Benchmark impact

- [ ] No performance-sensitive path changed.
- [ ] Benchmarks or rationale are included for performance-sensitive changes.

# Migration impact

- [ ] No migration required.
- [ ] Migration steps and compatibility window are documented.

# Breaking change details

If the compatibility classification is **Breaking change**, paste the final
commit footer here:

```text
BREAKING CHANGE: <migration impact and replacement>
```
