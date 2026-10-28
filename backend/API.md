# Raspberry Bin API

Base URL: `https://raspberry-bin-27.swedencentral.cloudapp.azure.com`
(the web app is served from the same domain, so it can use relative URLs like
`/api/auth/login`; in development use a Vite proxy to `http://localhost:8080`).

All bodies are JSON unless noted. Errors look like `{"error": "message"}`.
`429` always means "too many requests / too many attempts, wait a bit".

## How a scan works

```
App: Start Scan → camera reads the QR on the bin → POST /api/scan-requests {bin_id, code}
Pi:  GET /api/device/commands (long polling) → {"command":"capture","request_id":…}
Pi:  takes the photo → POST /api/scan (bin_id, request_id, image) → opens the lid if can_sort
App: GET /api/scan-requests/{id}?wait=25 (long polling) → {"status":"done","result":…}
```

**Long polling:** with `?wait=25` the server keeps the request open until
something happens (then answers at once) or 25 s pass. Send the next request
right after you get an answer. Maximum `wait` is 30.

## Accounts

### `POST /api/auth/register`
`{"username": "alice", "password": "…"}` → `201 {"username": "alice"}`

- username: 3–32 characters `a-z 0-9 . _ -` (stored in lower case)
- password: 12–72 characters with letters and digits
- `400` invalid username/password (message says why), `409` username taken,
  `429` too many registrations from this IP

### `POST /api/auth/login`
`{"username": "alice", "password": "…"}` →
`200 {"token": "…", "expires_at": "2026-10-28T20:00:00Z", "role": "user"}`

- `role` is `user` or `admin`; show the admin screens only for `admin`
- `401` wrong username or password, `429` locked for 5 min after 5 wrong passwords
- Send the token as `Authorization: Bearer <token>` on every request below.
  It expires after 8 hours; any `401` means "log in again".
- Keep the token in memory or `sessionStorage`, not in a cookie or `localStorage`.

### `GET /api/auth/me` → `200 {"username": "alice", "role": "user"}`
### `POST /api/auth/logout` → `204`

## Scanning (user token)

### `POST /api/scan-requests`
The QR on the bin contains `rbin:<bin_id>:<code>`. Split it and send:
`{"bin_id": "bin-27", "code": "…"}` → `202 {"request_id": "3f9c…"}`

- `403` wrong code (QR not from this bin, or replaced)
- `409` this bin is busy with another scan, or you already have one running
- `429` too many scan starts (about 1 per 3 s per user)

### `GET /api/scan-requests/{request_id}?wait=25`
- `{"status": "pending"}`: still waiting (send the request again)
- `{"status": "done", "result": {...}}`: see "Scan result" below
- `{"status": "failed", "reason": "timeout"}`: the bin did not send a photo within 15 s
- `{"status": "failed", "reason": "internal_error"}`: server error, try again
- `404` unknown request or not yours. Results are kept for 5 minutes.

### Scan result
```json
{"category": "PMD", "confidence": 0.93, "can_sort": true, "reason": "",
 "bin_id": "bin-27", "scanned_at": "2026-10-28T14:03:11Z"}
```
If `can_sort` is `false`, all lids stay closed and `reason` says why:

| reason | Show the user |
|---|---|
| `unsupported_item` | This item does not belong in this bin |
| `low_confidence` | Not sure what this is, please try again |
| `classifier_unavailable` | Sorting is temporarily unavailable |
| `unreadable_image` (and other image errors) | Photo failed, please try again |

`category` is `GENERAL_WASTE`, `ORGANIC`, `PAPER`, `PMD`, `UNSUPPORTED`
(or `unknown` / `rejected` when the photo could not be classified).

## My data (user token)

### `GET /api/me/scans?limit=20&before=<scanned_at>`
Your scans, newest first, as a list of scan results. `limit` 1–50 (default 20).
For the next page pass the `scanned_at` of the last item as `before`.

### `GET /api/me/stats?from=2026-10-01&to=2026-10-28`
Dates in Dutch time, both included, default the last 30 days, at most 366 days.
```json
{
  "summary": {
    "total": 42, "sorted": 37, "rejected": 5,
    "by_category": {"PMD": 15, "ORGANIC": 9, "PAPER": 7, "GENERAL_WASTE": 6},
    "per_day": [{"date": "2026-10-26", "count": 12}, {"date": "2026-10-27", "count": 30}]
  },
  "streak": {"current": 3, "longest": 7, "last_sorted_date": "2026-10-27"}
}
```
The streak counts days in a row with at least one **sorted** item (rejected
scans do not count). `current` is 0 if the last sorted item was before yesterday.

## Admin (token of an account with role `admin`)

User tokens get `403` on these routes.

### `POST /api/admin/lids/{compartment}/{open|close}`
`compartment`: `GENERAL_WASTE`, `ORGANIC`, `PAPER`, `PMD`.
Body `{"bin_id": "bin-27"}` → `202 {"status": "queued"}`.
The Pi receives the command within a second; if it does not pick it up within
10 s the command is dropped.

`POST /api/admin/login`, `GET /api/admin/me` and `POST /api/admin/logout` also
exist; `/api/admin/login` only accepts admin accounts.

## Raspberry Pi (header `X-API-Key: <device key>`)

### `GET /api/device/commands?bin_id=bin-27&wait=25`
- `204`: nothing to do, send the request again
- `200 {"command": "capture", "request_id": "3f9c…"}`: take a photo and send it (below)
- `200 {"command": "open_lid", "compartment": "PMD"}` / `close_lid`: admin command
- `404` unknown bin

Use a client timeout a bit longer than `wait` (e.g. 35 s), and wait a few
seconds before retrying after a network error.

### `POST /api/scan` (multipart form)
Fields: `bin_id`, `request_id` (from the capture command), `image` (JPEG or PNG, max 5 MB)
→ `200 {"bin_id": "bin-27", "category": "PMD", "confidence": 0.93, "can_sort": true}`

**Open the lid for `category` only if `can_sort` is `true`.**

- `400` invalid input (message says what), `401` wrong device key
- `404` unknown request_id, `409` request already used or timed out (> 15 s)

The Pi only scans after a capture command; a photo without a valid
`request_id` is rejected.

## Server setup

- Bins are registered on the server; this prints the QR to print and stick on the bin:
  `sudo docker compose run --rm api ./create-bin -bin bin-27`
  (add `-T` and `-png > bin-27.png` for a PNG). Running it again gives the bin a
  new code; the old QR stops working.
- Admins: `sudo docker compose run --rm api ./create-admin -username admin`
- Caddy serves the built web app from `frontend/dist` (`npm run build`).
