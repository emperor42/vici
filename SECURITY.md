# VICI security notes

## Scope

VICI contains a browser cookie/encryption helper and an optional Go card
editor. The two pieces have different trust boundaries. Neither implements a
user account system, authorization service, or production CMS.

## Browser helper

`vici.js` runs with the privileges of the page that loads it.

- `document.cookie` can read and write only cookies visible to JavaScript.
  JavaScript cannot add the `HttpOnly` attribute; that requires a server
  `Set-Cookie` header.
- `cookieSet` is a convenience wrapper, not a session, CSRF defense, or
  access-control mechanism.
- Web Crypto provides authenticated encryption for data handled in the
  browser, but the passphrase/key and the decrypted value are still exposed to
  same-origin scripts and the browser profile. It is not protection from XSS,
  a malicious extension, or a compromised origin.
- The file does not sanitize DOM content, make network authorization decisions,
  or provide secure key storage.

Applications should use a strict script policy, HTTPS, server-side session and
authorization controls, and `HttpOnly`/`Secure` cookies where appropriate.

## Go card demo

The demo binds to `127.0.0.1:8085` by default. A non-loopback listener is
refused unless `VICI_API_TOKEN` is set; with a token, all non-static routes
require a constant-time bearer/`X-Vici-Token` check. This is a demo guard, not
authorization, CSRF protection, rate limiting, TLS, or multi-user isolation.
Reachable clients can still read and modify all cards, import/export data, and
clear the store. Keep the process on loopback or place it behind a separately
reviewed authentication and network policy.

The handler has basic method, body-size, field-length, and CSV-row limits. Card
values are rendered with `html/template`, and the browser demo uses ordinary
DOM APIs. Those measures reduce accidental mistakes; they do not turn the
demo into a secure service.

`cards.json` is plaintext local data. The server writes it through a temporary
file and rename, but it does not encrypt it, replicate it, back it up, or
protect it from an operator or process that can read the data directory. The
client-side encryption helper is unrelated to this file.

The Dockerfile exposes port 8085 inside the container so a port mapping can
work. Set `VICI_API_TOKEN` at runtime and publish the port only to a protected
network unless a complete deployment review has added the controls above.

## Reporting a suspected vulnerability

Report suspected vulnerabilities privately to the project maintainers. Do not
include real card data, credentials, session cookies, or a public exploit
target in a public issue. Include the affected file, deployment assumptions,
and a minimal safe reproduction.
