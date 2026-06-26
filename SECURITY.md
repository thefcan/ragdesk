# Security Policy

## Reporting a vulnerability

If you discover a security vulnerability in ragdesk, please report it privately
by opening a [security advisory](https://github.com/thefcan/ragdesk/security/advisories/new).
Please do **not** open a public issue for security problems.

We aim to acknowledge reports within 72 hours.

## Supported versions

ragdesk is in active development; security fixes target the `main` branch.

## Automated scanning

- **Dependabot** keeps Go, Python, Docker and GitHub Actions dependencies patched.
- **govulncheck** scans the Go service for known vulnerabilities on every CI run.
- **CodeQL** statically analyses the Go and Python code on every push.
