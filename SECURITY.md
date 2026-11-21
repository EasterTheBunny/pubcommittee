# Security Policy

PubCommittee asks people to trust it with their positions on legislation. Weaknesses that could expose who supported or opposed what, fake or inflate support, or let software write on someone's behalf are treated as security issues, not only conventional software bugs.

## Reporting a vulnerability

**Do not open a public issue, pull request, or discussion for a vulnerability.**

Report privately through GitHub: [Report a vulnerability](https://github.com/EasterTheBunny/pubcommittee/security/advisories/new) (the repository's **Security** tab → **Report a vulnerability**).

Please include:
- what is affected (file, route, design doc, or deployed URL);
- steps to reproduce, or the reasoning for a design-level issue;
- the impact you expect;
- any suggested fix.

## What to expect

| Step | Target |
| --- | --- |
| Acknowledge the report | Within 7 days |
| Initial assessment and severity | Within 14 days |
| Fix or mitigation plan | Depends on severity; shared with the reporter |
| Public disclosure | After a fix is available, coordinated with the reporter |

Reporters are credited in the advisory unless they ask not to be.

## Scope

**In scope**
- Code in this repository, including the `app/` proof of concept.
- The deployed demo at https://pubcommittee.netlify.app.
- Design flaws in `design/`, especially:
  - ways to infer an individual's support or opposition from public outputs ([design/11](design/11-ai-security.md));
  - ways to re-identify reviewers or profile them across bills;
  - ways to write comments, suggestions, or signals without a person acting ([design/10](design/10-machine-access.md));
  - ways to inject instructions that steer agents or the platform's own generated content;
  - ways to bypass verification, moderation, or audit ([design/08](design/08-onboarding-and-verification.md), [design/09](design/09-integrity-moderation-and-privacy.md)).
- Secrets or credentials committed anywhere in the repository history.

**Out of scope**
- The mock data itself (names, bills, numbers are invented).
- Placeholder features that are labelled as not yet built, unless the design for them is flawed.
- Denial-of-service and volumetric testing against the deployed demo.
- Social engineering, physical attacks, and reports about third-party services (hosting, sign-in providers) that are not caused by this project's configuration.

## Supported versions

The project is pre-release. Only the latest commit on `main` is supported. Fixes are not backported.

## Safe harbor

Good-faith research that follows this policy is welcome. Please:
- test only against your own accounts and data, or local builds;
- never access, change, or keep other people's data, including signals, addresses, or verification records;
- stop and report as soon as you find a way to reach someone else's data;
- give reasonable time for a fix before any public disclosure.

The maintainers will not pursue action against research that follows these rules.
