# Garba Night passes · IIM Ahmedabad

Pass distribution and gate entry for Cultcomm's Garba Night. Mobile first, in the Kutch design (2b) from `design/`. Built for 3000+ people, and it **never sends an email**.

## How it works

- **Everyone at IIMA** signs in with Google using their **@iima.ac.in** account and gets a pass immediately. No list to upload.
- **Guests (friends & family):** each person can add guests up to a limit.
  - **PGP1** (emails starting `p26` or `f26`): no guests. The pass is for themselves only.
  - **Everyone else on campus:** 3 guests by default.
  - Admins can change the default per group, and set **any one person's limit** individually.
- **Showing guest passes:** all of them live on the host's phone, and the host can **swipe through every QR** at the gate (good for families). If the host adds a guest's Gmail, that guest can sign in with Google and show **only their own** QR. Hosts can also share a private **pass link** on WhatsApp for guests without Google.
- **Exchange guests:** Cultcomm uploads an **Excel sheet** (Name, Email, College, Phone). Guests with an email sign in with Google. Everyone gets a pass link, and admins can download all links as a CSV to share with the partner colleges.
- **Screenshot-proof QR:** the code rotates every 15 seconds and is signed. Phones compute it offline, so it works on weak venue Wi-Fi. The gate accepts codes up to 30 seconds old, so a forwarded screenshot shows **"Old QR · screenshot?"**. The pass also shows a ticking clock and a moving sheen, so volunteers can spot a still image.
- **Scanned once:** after a scan, the pass is used everywhere. Scanning again shows **"Already used · Scanned at 7:41 PM · Gate 3"**, and the holder's phone shows a SCANNED stamp.
- **Colour per pass type,** on the pass and on the scanner result, so volunteers know which ID to check:

  | Colour | Pass type | ID to check |
  | --- | --- | --- |
  | Pink | Students (incl. PGP1) | Student ID |
  | Blue | Faculty & staff | IIMA ID |
  | Orange | Guests | Photo ID |
  | Teal | Exchange | College ID |

- **Volunteers and admins** get a pass of their own too (any Google account; non-IIMA ones show as orange **Cultcomm team**, check photo ID), reachable from **MY QR** on the scanner and **Show my pass QR** on the dashboard.
- **Volunteers** scan with their phone camera. If a QR won't scan (for example, a cracked screen), they can type the pass code; that requires an ID check before they tap **Admit**.
- **Pass codes** look like `KP7X-4MQ`: 6 random characters plus a check character, from an alphabet without look-alikes (no 0/O/1/I/L/U).
  - They're random, so nobody can work out other people's codes. About 730 million are possible, so a made-up code hits a real pass roughly 1 in 150,000 times.
  - The check character catches every single-character typo and 99.8% of swapped neighbours. The scanner flags a typo while it's being typed, before anything is looked up.
- **Cultcomm admins** get a dashboard with five tabs:
  - **Live:** entries by gate and by pass type, and the latest arrivals.
  - **Passes:** search every pass, cancel one, or undo an entry scanned by mistake.
  - **People:** change a person's guest limit, cohort or role, and give volunteers access before the night.
  - **Exchange:** upload the Excel sheet and download pass links.
  - **Settings:** default limits, PGP1 email prefixes, and event details. Plus a CSV export of every pass.

Cohorts are read from the @iima.ac.in address:
- Addresses starting with the PGP1 prefixes (default `p26`, `f26`) are **PGP1**.
- Batch-style addresses like `p25name` or `phd23name` are **Student**.
- Anything else is **Faculty & Staff**.

Admins can move anyone between cohorts from People.

## How it's built

| Part | Tech | Where |
| --- | --- | --- |
| Backend | [PocketBase](https://pocketbase.io) v0.40 as a Go framework: SQLite, Google/Microsoft OAuth2, realtime, admin UI at `/_/`, plus custom routes under `/api/garba/*` | `backend/` |
| Web app | Preact + Vite, about 32 KB gzipped on first load. The scanner and dashboard load only for Cultcomm. Self-hosted fonts and a service worker | `web/` |
| Rotating QR | HMAC-SHA256 over pass id + 15 s time step. Same code on phone (`web/src/qr.ts`) and server (`backend/garba/service.go`) | |
| API types | Shared TypeScript shapes of the Go responses | `shared/types.ts` |
| Design handoff | The Claude Design mockups and chat | `design/` |

Everything runs as **one binary** that serves both the API and the web app, with its data in one folder (`pb_data`).

## Run it locally

Needs Go ≥ 1.24 (it fetches a newer toolchain automatically) and Node ≥ 20.

```sh
npm install
npm run build          # web app -> backend/pb_public
npm run seed           # demo people, guests and exchange passes
npm run backend        # http://127.0.0.1:8090
```

Sign-in needs Google keys (see below; `http://localhost:8090/auth/callback` works as a redirect URI). Without them you can still try things: create a superuser (`cd backend && go run . superuser upsert you@x.com 'pass-1234567'`), open the dashboard at `/_/`, and use **impersonate** on a user.

For frontend work, run `npm run backend` and `npm run dev` together. Vite on :5173 proxies `/api` to PocketBase.

Tests: `npm run backend:test` covers:
- cohorts from email, guest limits and per-person overrides
- guests signing in, pass links
- rotating QR: old and forged codes rejected, "Scanned at … · Gate …"
- typed code → ID check → admit
- Excel import
- 20 parallel scans of one pass letting it in exactly once

`npm run typecheck` checks the web app.

## Deploy on the campus server (students.iima.ac.in/garba2026)

Needs Node 20+ and git. Go is installed automatically into `./.tools` if missing, so there's no sudo for the build.

```sh
git clone https://github.com/arnav127/iima-garba.git && cd iima-garba
./start.sh            # first run creates .env and stops: fill in GOOGLE_* and ADMIN_EMAILS
./start.sh            # installs, builds for /garba2026/, starts under PM2, health-checks
pm2 startup           # once: run the command it prints (with sudo) so it survives reboots
```

- **Updates:** `git pull && ./start.sh` rebuilds and reloads with no downtime.
- **Day to day:** `pm2 logs garba2026` for logs; `pm2 restart garba2026` to restart. After editing `.env`, run `pm2 startOrReload ecosystem.config.cjs --update-env`.
- **Settings:** everything lives in `.env` (see `.env.example`): `APP_URL`, `BASE_PATH`, `PORT` (default 8090, bound to 127.0.0.1 only) and `DATA_DIR`.
- **Backups:** back up `DATA_DIR` (default `./data`), or schedule backups in the dashboard (Settings → Backups).
- **Dashboard:** `https://students.iima.ac.in/garba2026/_/`. Set `SUPERUSER_EMAIL` and `SUPERUSER_PASSWORD` in `.env` and `start.sh` creates the login.
- **Exchange list from the shell:** `./backend/garba-server exchange guests.xlsx --dir data`

### Apache

One prefix carries everything (app, API under `/garba2026/api/`, dashboard under `/garba2026/_/`), because PocketBase serves the app and the API from the same process. Paste `deploy/apache-garba2026.conf` inside the existing `<VirtualHost *:443>` for students.iima.ac.in:

```apache
RedirectMatch 301 ^/garba2026$ /garba2026/

ProxyPass        /garba2026/api/realtime http://127.0.0.1:8090/api/realtime flushpackets=on timeout=3600
ProxyPassReverse /garba2026/api/realtime http://127.0.0.1:8090/api/realtime

ProxyPass        /garba2026/ http://127.0.0.1:8090/ timeout=120
ProxyPassReverse /garba2026/ http://127.0.0.1:8090/

<Location /garba2026/>
    RequestHeader set X-Forwarded-Proto "https"
    RequestHeader set X-Forwarded-Prefix "/garba2026"
</Location>
```

```sh
sudo a2enmod proxy proxy_http headers && sudo apachectl configtest && sudo systemctl reload apache2
```

- **Keep the realtime line above the general `ProxyPass`.** It stops Apache from buffering the live dashboard updates.
- **HTTPS is required** for Google sign-in and the phone camera; students.iima.ac.in already has it.
- **Capacity:** one small VM handles thousands of people. A gate scan is one SQLite transaction (well under a millisecond), and QR codes are generated on phones, not the server.
- **Hosting at a different path:** change `BASE_PATH` and `APP_URL` in `.env`, then run `./start.sh` again.

### Gate day: keeping the line fast

The server isn't the bottleneck. In a load test through Apache with 3,000 passes and 20 phones scanning at once, it handled **~1,070 scans per second** (median 14 ms, 99% under 70 ms, 0 errors). What sets the pace is people, phones and the network:

- **Throughput per volunteer:** expect about 3–5 seconds per person (raise the phone, read the QR, glance at the ID), so 12–20 people a minute. For 3,000 people in 30–45 minutes, plan **2–3 scanning phones per gate**. More phones scale linearly.
- **Put scanner phones on Wi-Fi.** Mobile data near 3,000 people slows down. Each scan is one small request, and the scanner gives up after 6 seconds with a red NETWORK badge rather than freezing. If a reply was lost after the person was admitted, scanning them again shows green ("You let them in 5s ago"), not "Already used".
- **The scanner scans continuously.** Volunteers don't need to tap anything between people. Each result beeps (high = in, low double = no), vibrates and flashes the whole screen green, red or orange, so volunteers can watch the person and their ID. Tap the screen once after opening the scanner so sound is allowed.
- **Tell guests to open their pass before the queue** (on signage and in the WhatsApp message). It opens instantly, even offline, but people fumbling for it is the real delay. Families: the host swipes through each QR.
- **Keep the scanner screen on:** the scanner screen stays awake by itself. Brighter screens on the guests' side scan faster.

### Google sign-in setup

1. Google Cloud Console → APIs & Services → **OAuth consent screen**: External, app name "Garba Night · IIMA", add your domain. **Publish** it, so any Google account can sign in, not just test users.
2. **Credentials → Create OAuth client ID → Web application.**
   - Authorised JavaScript origin: `https://students.iima.ac.in`
   - Authorised redirect URI: `https://students.iima.ac.in/garba2026/auth/callback`
3. Put the client ID and secret in `.env` and run `./start.sh` again.

If @iima.ac.in mail is on Microsoft 365 instead of Google Workspace, also fill in `MICROSOFT_CLIENT_ID` and `MICROSOFT_CLIENT_SECRET`. A "Continue with Microsoft" button then appears.

## Or: web app on Vercel, backend on campus

The web app is a static build, so it can be hosted on Vercel while PocketBase runs on campus. The campus server still needs a public HTTPS URL.

1. Import `arnav127/iima-garba` in Vercel. `vercel.json` already sets the build (`npm run build`, output `backend/pb_public`).
2. In Vercel → Settings → Environment Variables, set `VITE_PB_URL=https://<your-campus-server>`.
3. On the server, set `APP_URL=https://<your-vercel-domain>` so pass links point to the Vercel app. Use `https://<your-vercel-domain>/auth/callback` as the Google redirect URI.

## Configuration

Environment variables in `.env` (see `.env.example`), applied on every start:

| Variable | What it does |
| --- | --- |
| `APP_URL` | Public URL of the web app, used in pass links |
| `MEMBER_DOMAINS` | Email domains that get a pass on sign-in (default `iima.ac.in`) |
| `ADMIN_EMAILS` | Comma-separated emails that are always Cultcomm admins |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | Google sign-in. Redirect URI: `<APP_URL>/auth/callback` |
| `MICROSOFT_CLIENT_ID`, `MICROSOFT_CLIENT_SECRET` | Optional Microsoft sign-in |
| `BASE_PATH`, `PORT`, `DATA_DIR`, `TRUSTED_PROXY_HEADERS` | Where it runs; see `.env.example` |
| `VITE_PB_URL` (build time) | Only when the web app is hosted separately from PocketBase |

Edit these in the app under Admin → Settings:
- Guest limits: PGP1 0, other students 3, faculty & staff 3.
- PGP1 email prefixes.
- Event details: date, venue, time, number of gates.

## Exchange sheet format

| Name | Email | College | Phone |
| --- | --- | --- | --- |
| Ishaan Mehta | ishaan.m@spjimr.org | SPJIMR Mumbai | +91 98200 00000 |
| Tara Singh | | XLRI Jamshedpur | |

- Headings are matched loosely: "Full Name", "Email ID", "Institute" and "Mobile" all work.
- Email is optional. Without one, the guest uses their pass link.
- Re-uploading updates rows instead of duplicating them.

Placeholders to confirm with Cultcomm:
- Date: Fri 16 Oct
- Venue: Football Ground
- Start: 8 PM
- Gates: 2
