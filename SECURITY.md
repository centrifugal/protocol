# Security Policy

## Reporting Security Vulnerabilities

Security reports for this package are handled together with the rest of the Centrifugal ecosystem, in the [Centrifugo](https://github.com/centrifugal/centrifugo) repository.

If you discover a security vulnerability in this package, please report it through GitHub using the **“Report a vulnerability”** button in the Centrifugo repository’s [Security](https://github.com/centrifugal/centrifugo/security) tab, or directly via [this form](https://github.com/centrifugal/centrifugo/security/advisories/new). Reports submitted this way are visible only to maintainers. Mention that the report is about `centrifugal/protocol`, so that it can be triaged accordingly.

Please do **not** open a public GitHub issue for security-related problems.

When reporting a vulnerability, include as much detail as possible, such as:

* A description of the vulnerability
* Steps to reproduce
* Affected versions
* Potential impact

We will acknowledge receipt of the report and work to assess the issue promptly.

Since this package implements protocol encoding and decoding, we are especially interested in reports about the decoding of untrusted input – panics, unbounded allocations, or out-of-bounds access reachable from data received over the wire.

## Vulnerability Detection

Besides external reports, this package relies on:

* **Fuzzing** – decoders are fuzzed continuously in CI, see [.github/workflows/fuzz.yml](.github/workflows/fuzz.yml). Inputs that trigger a failure are added to the seed corpus under `testdata/fuzz`.
* **Static analysis and linters** – `golangci-lint` (including `gosec` and `govet`) runs on every push and pull request.
* **Dependency updates** – dependencies are monitored and updated via Dependabot.

## Triage and Assessment

Reported or detected vulnerabilities are reviewed by the project maintainers and assessed based on severity, exploitability, and the impact on applications using the package. Confirmed issues are fixed in a new release of this package, and the fix is propagated to [Centrifugo](https://github.com/centrifugal/centrifugo) and [Centrifuge](https://github.com/centrifugal/centrifuge) by updating the dependency there.
