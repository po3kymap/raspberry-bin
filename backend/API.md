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
- Store the token in `sessionStorage` or `localStorage` (team decision, see the
  P-SBD); never put it in a URL. The strict CSP is what protects it against XSS.

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

## Check an item with the phone camera (user token)

### `POST /api/check` (multipart form)
Field `image` (JPEG or PNG, max 5 MB) →
`200 {"category": "PMD", "confidence": 0.91, "can_sort": true, "reason": ""}`

For trying at home which bin an item belongs in: no bin, no QR and no lid.
`can_sort` / `reason` mean the same as in a scan result. Checks never count
for the streak, the statistics or the activity log. The photo and result are
saved (Storage `checks/`, Firestore `checks`) as training data for the model.

- `400` no image, not JPEG/PNG or too large, `429` more than about 1 check per 3 s

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
    "per_day": [{"date": "2026-10-26", "count": 12, "sorted": 11}, {"date": "2026-10-27", "count": 30, "sorted": 26}]
  },
  "streak": {"current": 3, "longest": 7, "last_sorted_date": "2026-10-27"}
}
```
The streak counts days in a row with at least one **sorted** item (rejected
scans do not count). `current` is 0 if the last sorted item was before yesterday.
In `per_day`, `count` is all scans of the day and `sorted` only the sorted ones,
so a day with `sorted > 0` is a streak day (e.g. for a Mon–Sun checkmark view).
Days without scans are not listed.

## Admin (token of an account with role `admin`)

User tokens get `403` on these routes.

### `POST /api/admin/lids/{compartment}/{open|close}`
`compartment`: `GENERAL_WASTE`, `ORGANIC`, `PAPER`, `PMD`.
Body `{"bin_id": "bin-27"}` → `202 {"status": "queued"}`.
The Pi receives the command within a second; if it does not pick it up within
10 s the command is dropped.

### `GET /api/admin/users`
All users sorted by username (password hashes are never sent):
```json
[{"username": "alice", "role": "user", "created_at": "2026-10-20T09:12:00Z",
  "last_login_at": "2026-10-29T13:05:00Z", "locked": false,
  "current_streak": 3, "longest_streak": 7, "last_sorted_date": "2026-10-28",
  "total_scans": 25, "sorted_scans": 22}]
```
- `locked`: the account is in the 5-minute lock after wrong passwords
- `last_login_at`: last successful login, `null` if the user has not logged
  in since this was added
- `total_scans` / `sorted_scans`: all scans of the user over all time and the
  accepted ones (the web app shows sorted / total as the user's accuracy)

### `POST /api/admin/users`
`{"username": "bob", "password": "…", "role": "user"}` → `201 {"username": "bob", "role": "user"}`

- `role` is `user` (default) or `admin`; same username and password rules as register
- `400` invalid username, password or role, `409` username taken

### `DELETE /api/admin/users/{username}` → `204`
Deletes the account and logs the user out everywhere at once. Their scans stay
in the overall statistics.

- `400` you tried to delete your own account, `404` unknown user,
  `409` the last admin cannot be deleted

### `POST /api/admin/bins/connect`
The admin scans the QR on a bin (`rbin:<bin_id>:<code>`) to connect to it:
`{"bin_id": "bin-27", "code": "…"}` → `200 {"bin_id": "bin-27"}`.
`403` wrong code, `429` too many attempts. The web app's Bins tab then shows
a short summary and the lid controls of that bin; the overview shows all bins.

### `GET /api/admin/stats?bin_id=bin-27&from=2026-10-01&to=2026-10-28`
Statistics of one bin (or of all bins without `bin_id`); same dates as
`/api/me/stats`. `users` is always about all accounts.
```json
{
  "summary": {
    "total": 120, "sorted": 104, "rejected": 16,
    "by_category": {"PMD": 40, "ORGANIC": 25, "PAPER": 22, "GENERAL_WASTE": 17},
    "per_day": [{"date": "2026-10-27", "count": 30, "sorted": 26}],
    "by_reason": {"unsupported_item": 9, "low_confidence": 6, "classifier_unavailable": 1},
    "by_bin": {"bin-27": 120},
    "average_confidence": 0.93,
    "active_users": 14,
    "top_users": [{"username": "alice", "total": 21, "sorted": 20}]
  },
  "users": {"total": 25, "admins": 2, "new": 3}
}
```
`by_reason` counts the rejected scans by reason, `average_confidence` is over
the sorted items only, `active_users` are users with a scan in the period and
`top_users` the 5 with the most sorted items; `users.new` counts the accounts
created in the period.

### `GET /api/admin/activity?limit=20&before=<at>`
The activity log, newest first; `limit` 1–50 (default 20). For the next page
pass the `at` of the last event as `before`.
```json
[{"type": "scan", "at": "2026-10-29T14:03:11Z", "username": "alice", "bin_id": "bin-27",
  "scan_id": "Xk2…", "image_id": "scans/5f1c….jpg", "category": "PMD",
  "confidence": 0.93, "can_sort": true},
 {"type": "login_failed", "at": "2026-10-29T13:58:02Z", "username": "admin",
  "reason": "wrong_password", "ip": "145.126.27.204", "can_sort": false}]
```

| type | Fields |
|---|---|
| `login` / `logout` | `username`, `role` (`ip` for login) |
| `login_failed` | `username` as typed, `reason` (`wrong_password`, `locked`, `not_admin`), `ip` |
| `register` | `username` (self-registration), `ip` |
| `user_created` / `user_deleted` | `username` = the admin, `target` = the account (`role` when created) |
| `scan` | `username`, `bin_id`, `scan_id`, `image_id` (object in Firebase Storage), `category`, `confidence`, `can_sort`, `reason` |
| `lid` | `username` = the admin, `bin_id`, `compartment`, `action` (`open`/`close`) |
| `bin_connect` | `username` = the admin, `bin_id` |

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
