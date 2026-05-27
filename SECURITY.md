# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest  | ✅        |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Use GitHub's private vulnerability reporting:

1. Go to the [Security tab](https://github.com/chasedputnam/go-music-gen/security) of this repository
2. Click **"Report a vulnerability"**
3. Fill in the details and submit

You can expect:
- **Acknowledgement** within 48 hours
- **Status update** within 7 days
- **Resolution or mitigation** as quickly as possible depending on severity

## What to Include

- Type of vulnerability (e.g. path traversal, command injection, unsafe deserialization)
- Full paths of affected source files
- Steps to reproduce
- Proof-of-concept or exploit code (if available)
- Impact assessment

## Scope

This project spawns a Python subprocess (`inference_worker.py`) and communicates over JSON-RPC via stdin/stdout. It also accepts multipart file uploads and serves an HTTP API. Relevant attack surfaces include:

- File upload handling in `/cover` and `/repaint` endpoints
- Subprocess invocation and JSON-RPC input parsing
- Temp directory creation and cleanup
- HTTP request validation in `internal/schema`
- Environment variable handling in `internal/config`

## Preferred Languages

English.
