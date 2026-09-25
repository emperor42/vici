# VICI

VICI is two independent pieces:

1. `vici.js` is a browser utility for non-`HttpOnly` cookies and Web Crypto
   encryption/decryption.
2. `main.go` is an optional local card/timeline and lore editor with a small
   JSON and CSV API.

The Go demo is not a WordPress-style page CMS, an authentication service, or a
payment system. The browser utility and the Go demo do not share state or an
API. `legacy/` is parked historical material and is not part of the supported
runtime.

## Browser utility (`vici.js`)

### Loading and requirements

Load the plain browser script normally:

```html
<script src="/path/to/vici.js"></script>
```

In a browser it exposes:

- `window.Vici` — the `Vici` class.
- `window.vici` — a ready-to-use singleton.

The file has no npm build step and no server dependency. Cookie methods use
`document.cookie`; encryption methods require a browser/Web Crypto environment
with `crypto.subtle` and `crypto.getRandomValues` (normally a secure context in
current browsers).

### Cookies

```js
const result = vici.cookieSet("theme", "dark", 30, {
  path: "/",
  sameSite: "Lax"
});
// { ok: true, warnings: [] }

const theme = vici.cookieGet("theme");       // "dark" or null
vici.cookieDelete("theme", { path: "/" });
const allReadableCookies = vici.cookieAll();
```

`new Vici({ domain, path, secure })` supplies defaults for later calls. The
actual methods and options are:

| API | Behavior |
| --- | --- |
| `cookieSet(name, value, days, options)` | Writes a URI-encoded cookie. Positive `days` adds an expiry; zero/omitted creates a session cookie. Supports `path`, `domain`, `secure`, and `sameSite`. |
| `cookieGet(name)` | Returns the decoded value visible to JavaScript, or `null`. |
| `cookieDelete(name, options)` | Writes an expired cookie using the supplied/default path and domain. |
| `cookieAll()` | Returns all cookies readable by the current page. |

Passing `httpOnly: true` returns a warning because JavaScript cannot set the
`HttpOnly` attribute. The server must emit that attribute in a `Set-Cookie`
header. Cookies without `Secure` or `SameSite` remain subject to the browser's
normal cookie rules; this utility does not provide a session or an
authorization boundary.

The aliases `setCookie`, `getCookie`, and `deleteCookie` point to the cookie
methods above.

### Encryption

```js
const payload = await vici.encrypt("private text", "a strong passphrase");
const plaintext = await vici.decrypt(payload, "a strong passphrase");
```

`encrypt` uses PBKDF2-SHA-256 (100,000 iterations in the current file) to
derive a non-exportable AES-256-GCM key, then returns a portable string in the
form:

```text
base64(salt).base64(iv).base64(ciphertext)
```

With `{ raw: true }`, it returns `base64(iv || ciphertext)` instead. `decrypt`
accepts either form. `randomBytes`, `randomToken`, and `sha256Hex` are also
exposed. The `hash` alias calls `sha256Hex`.

These operations are useful for data that the page deliberately handles in the
browser, but they do not protect a value from script running on the same
origin, a compromised browser/profile, or a server that receives plaintext.
There is no key management, user identity, authentication, or server API in
this file.

## Optional Go card demo

The Go program serves a card-based fictional-world editor. It uses only the
standard library and stores cards in a JSON file.

### Run locally

From this directory:

```sh
gofmt -w main.go
go run .
```

The default listener is `127.0.0.1:8085`. The loopback bind is intentional.
The API is unauthenticated only in this local mode; a non-loopback listener is
refused at startup unless `VICI_API_TOKEN` is set. When set, the token is
required for all non-static routes (bearer or `X-Vici-Token`). Open
<http://127.0.0.1:8085/> in a browser.

Environment variables:

| Variable | Default | Meaning |
| --- | --- | --- |
| `VICI_HOST` | `127.0.0.1` | Listen address. |
| `VICI_PORT` | `8085` | Listen port. |
| `VICI_API_TOKEN` | unset | Required when binding a non-loopback address; protects the API. |
| `VICI_DATA_FILE` | `cards.json` | JSON data file. |

Example with a separate data file and port:

```sh
VICI_DATA_FILE=/tmp/vici-cards.json VICI_PORT=8090 go run .
```

The data file is rewritten atomically on successful changes. It is local demo
data, not a multi-user database; make a backup before using it for anything
important.

### Actual demo routes

| Route | Method | Description |
| --- | --- | --- |
| `/` | `GET` | Server-rendered card editor and timeline. |
| `/api/cards` | `GET` | Returns the current cards as JSON. |
| `/api/cards` | `POST` | Creates a card from JSON. `name` is required; `kind` defaults to `Event` and must be one of `Event`, `Faction`, `Nation`, `Species`, or `Leader`. The server assigns ID and timestamp. |
| `/api/cards` | `DELETE` | Clears all cards. |
| `/api/cards/{id}` | `GET` | Returns one card. |
| `/api/cards/{id}` | `PUT` | Replaces the editable fields of one card. |
| `/api/export` | `GET` | Downloads the current cards as CSV. |
| `/api/import` | `POST` | Imports a multipart CSV upload. The first row is treated as a header; imported rows are appended, not merged by ID. |
| `/static/*` | `GET` | Demo CSS and browser JavaScript. |

For example:

```sh
curl http://127.0.0.1:8085/api/cards
curl -X POST http://127.0.0.1:8085/api/cards \
  -H 'Content-Type: application/json' \
  -d '{"name":"The First Event","kind":"Event","text":"A beginning"}'
```

The page's card values are rendered with Go's `html/template` escaping. The
JSON API still accepts untrusted input, so the demo applies request-size,
field-length, CSV-row, and method limits, but it does not provide a trust
boundary.

### Security limitations

- There are no users, passwords, sessions, roles, permissions, CSRF tokens,
  audit log, or TLS termination in this demo.
- In the default loopback mode there is no API token. A non-loopback listener
  is refused unless `VICI_API_TOKEN` is set; with a token, all non-static
  routes require a constant-time bearer/`X-Vici-Token` check. Keep it on
  loopback or behind a separately reviewed authentication and network policy.
- The card store has a 10,000-card quota, an 8 MiB import limit, and bounded
  field lengths. `cards.json` contains the card data in plaintext. File
  permissions and backups are the operator's responsibility; the browser
  encryption helper does not encrypt this server-side file.
- CSV import is intentionally simple. It assumes the first row is a header,
  appends rows, and does not provide a migration or conflict-resolution
  workflow.
- The browser's `document.cookie` view excludes `HttpOnly` cookies, and
  client-side encryption cannot defend against same-origin XSS. Those are
  properties of the platform, not protections supplied by this repository.

### Container image

The Dockerfile uses `golang:1.21-alpine`, matching the `go 1.21` directive in
`go.mod`, and builds the module with `go build .`:

```sh
docker build -t vici-card-demo .
docker run --rm -p 127.0.0.1:8085:8085 \
  -e VICI_API_TOKEN='replace-with-a-long-random-token' \
  -v "$PWD/data:/app/data" \
  -e VICI_DATA_FILE=/app/data/cards.json \
  vici-card-demo
```

The container listens on `0.0.0.0:8085` internally so a published port works.
The example publishes it only to host loopback. Do not change that to a public
interface without adding authentication, authorization, CSRF handling, TLS,
and an appropriate data backup policy.

## Checks

From this directory:

For a container or other non-loopback deployment, provide `VICI_API_TOKEN` and
send it as `Authorization: Bearer …` (or `X-Vici-Token`) on every non-static
request. The token is a demo guard, not a user/session system.

```sh
gofmt -w main.go
go test ./...
node --check vici.js
node --check static/app.js
```

The Go tests use a temporary data file so running them does not rewrite the
checked-in demo data. There is no npm package, bundler, or browser test suite
in this repository.
