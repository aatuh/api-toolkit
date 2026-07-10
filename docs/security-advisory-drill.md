# Security Advisory Drill

Audience: maintainers who need evidence that the private vulnerability process
can be followed without exposing a report before a fix or mitigation is ready.

## Completed Exercise

- Date: `2026-07-10`
- Drill reference: `SIM-2026-07-10-001`
- Exercise type: tabletop triage with local validation on commit `0ff597c`.
- Scenario severity: High, for response-planning purposes only.

This was a fictional simulation. No vulnerability was discovered, reported, or
confirmed. No GitHub Security Advisory, CVE, public issue, pull request,
release, customer notification, or external disclosure was created. The
scenario contains no reporter, customer, secret, private URL, or reproduction
details.

## Scenario

A fictional private report claimed that a supported request-authentication path
might accept an attacker-controlled tenant identity. The exercise assumed that
the claim was not yet reproduced and that the latest supported release was
`v3.1.2`. It did not name a real package, endpoint, dependency, or code path.

The exercise classified the hypothetical impact as High until a maintainer could
establish reachability, affected versions, exploitability, and an effective
workaround. That is consistent with `SECURITY.md`: acknowledge within three
business days, confirm the impact and mitigation path within three business days
after acknowledgement, and target a patch, workaround, or advisory within 14
calendar days.

## Exercise Log

| Step | Simulated maintainer action | Result |
| --- | --- | --- |
| Private intake | Receive the report only through GitHub's private "Report a vulnerability" flow. | No report details are copied into an issue, pull request, commit, chat transcript, release evidence, or test fixture. |
| Triage | Confirm the supported-version question, request a minimal private reproduction if needed, and assign one maintainer owner. | The claim remains unconfirmed and private; no severity is published as a fact. |
| Containment | Pause related public discussion, avoid speculative fixes, and identify a safe application-level mitigation if one exists. | The simulated reporter would receive the acknowledgement target and next private update time. |
| Investigation | Reproduce only in a private branch or private advisory collaboration space, then determine the exact affected package and versions. | Do not create a CVE or public advisory until a fix, workaround, or coordinated disclosure date is ready. |
| Fix and verification | Review the smallest fix, add a regression test without disclosure-sensitive inputs, and run release-relevant checks. | A real fix would include supported-version impact, workaround, release note, and disclosure decision. |
| Coordinated disclosure | Publish only after the fix or approved mitigation is available and affected parties have the agreed notice. | The public advisory would state affected versions, fixed version, mitigation, acknowledgements, and credit only with reporter consent. |

## Local Validation Performed

The following commands completed successfully during this fictional exercise:

```sh
GOTOOLCHAIN=local GOWORK=off make vuln
GOTOOLCHAIN=local GOWORK=off go test ./middleware/auth/... ./webhooks -count=1
```

These are supporting triage signals, not proof that an unreported vulnerability
does or does not exist. A real advisory would run the narrowest reproduction and
regression tests first, then the release checks appropriate to the affected
surface.

## Closure And Follow-Up

The drill closed with no product-code change, advisory, issue, CVE, or user
action because the scenario was fictional. The documentation record is retained so a
future maintainer can rehearse the actual private path and timing targets. Rerun
this exercise after a material change to `SECURITY.md`, the reporting channel,
supported-version policy, or release process, and at least once per calendar
year.
