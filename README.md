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

## Deploy on a campus server

```sh
npm ci && npm run build && npm run backend:build        # backend/garba-server + backend/pb_public
sudo mkdir -p /opt/garba && sudo cp -r backend/garba-server backend/pb_public /opt/garba/
sudo useradd --system garba && sudo chown -R garba:garba /opt/garba
sudo cp deploy/garba.env.example /opt/garba/garba.env   # fill it in
sudo cp deploy/garba.service /etc/systemd/system/ && sudo systemctl enable --now garba
```

- Cross-compile from a laptop with `GOOS=linux GOARCH=amd64 npm run backend:build`.
- **HTTPS is required** (for Google sign-in and the phone camera). If the server has a public DNS name, change `ExecStart` to `serve garba.iima.ac.in` and PocketBase gets a Let's Encrypt certificate itself (ports 80/443). Otherwise put it behind the campus reverse proxy.
- Superuser for the PocketBase dashboard: `./garba-server superuser upsert you@iima.ac.in 'a-long-password' --dir /opt/garba/pb_data`.
- Backups: the whole state is `/opt/garba/pb_data`. Schedule backups in the dashboard (Settings → Backups).
- Exchange list from the server shell: `./garba-server exchange guests.xlsx --dir /opt/garba/pb_data`, or upload it in Admin → Exchange.
- Capacity: one small VM handles thousands of people. A gate scan is one SQLite transaction (well under a millisecond), and QR codes are generated on phones, not the server.

### Google sign-in setup

1. Google Cloud Console → APIs & Services → **OAuth consent screen**: External, app name "Garba Night · IIMA", add your domain. **Publish** it, so any Google account can sign in, not just test users.
2. **Credentials → Create OAuth client ID → Web application.** Authorised redirect URI: `<APP_URL>/auth/callback`.
3. Put the client ID and secret in `garba.env` and restart.

If @iima.ac.in mail is on Microsoft 365 instead of Google Workspace, also fill in `MICROSOFT_CLIENT_ID` and `MICROSOFT_CLIENT_SECRET`. A "Continue with Microsoft" button then appears.

## Or: web app on Vercel, backend on campus

The web app is a static build, so it can be hosted on Vercel while PocketBase runs on campus. The campus server still needs a public HTTPS URL.

1. Import `arnav127/iima-garba` in Vercel. `vercel.json` already sets the build (`npm run build`, output `backend/pb_public`).
2. In Vercel → Settings → Environment Variables, set `VITE_PB_URL=https://<your-campus-server>`.
3. On the server, set `APP_URL=https://<your-vercel-domain>` so pass links point to the Vercel app. Use `https://<your-vercel-domain>/auth/callback` as the Google redirect URI.

## Configuration

Environment variables (see `deploy/garba.env.example`), applied on every start:

| Variable | What it does |
| --- | --- |
| `APP_URL` | Public URL of the web app, used in pass links |
| `MEMBER_DOMAINS` | Email domains that get a pass on sign-in (default `iima.ac.in`) |
| `ADMIN_EMAILS` | Comma-separated emails that are always Cultcomm admins |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | Google sign-in. Redirect URI: `<APP_URL>/auth/callback` |
| `MICROSOFT_CLIENT_ID`, `MICROSOFT_CLIENT_SECRET` | Optional Microsoft sign-in |
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
- Date: Sat 17 Oct
- Venue: LKP
- Start: 8 PM
- Gates: 2
