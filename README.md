# Garba Night passes · IIM Ahmedabad

Pass distribution and gate entry for Cultcomm's Garba Night. Mobile first, in the Kutch design (2b) from `design/`.

- **IIMA students and staff** sign in (Google or an email code), see their own entry QR, and send passes to friends up to their group's quota.
- **Friends** get a claim link (by email, or shared on WhatsApp for phone numbers), claim the pass in their name and show the QR. Friends sent a pass by email can also sign in with that email.
- **Exchange guests** (pass exchange with other colleges) are added by Cultcomm and get their own pass.
- **Volunteers** scan QRs at the gate. Every pass gets in once; a second scan shows "Already used" with the gate and time.
- **Cultcomm admins** get a live dashboard (entries by gate, latest entries), search every pass, revoke or undo an entry, import the list from CSV, set quotas and event details, and export everything as CSV.

## How it's built

| Part | Tech | Where |
| --- | --- | --- |
| Backend | [PocketBase](https://pocketbase.io) v0.40 as a Go framework: SQLite, auth (Google OAuth2 + email OTP), realtime, admin UI at `/_/`, plus custom routes under `/api/garba/*` | `backend/` |
| Web app | Preact + Vite, ~30 KB gzipped on first load; the scanner and dashboard load only for Cultcomm. Self-hosted fonts, service worker for weak venue Wi-Fi | `web/` |
| API types | Shared TypeScript shapes of the Go responses | `shared/types.ts` |
| Design handoff | The Claude Design mockups and chat | `design/` |

Everything runs as **one binary** that serves both the API and the web app, with its data in one folder (`pb_data`).

## Run it locally

Needs Go ≥ 1.24 (it fetches a newer toolchain automatically) and Node ≥ 20.

```sh
npm install
npm run build          # web app -> backend/pb_public
npm run seed           # demo people and passes from the mockups
npm run backend        # http://127.0.0.1:8090
```

Without SMTP, sign-in codes and emails are printed in the server log. Demo accounts:
`p25aarav@iima.ac.in` (student), `ishaan.m@spjimr.org` (exchange guest), `p24meera@iima.ac.in` (volunteer), `cultcomm@iima.ac.in` (admin).

For frontend work, run `npm run backend` and `npm run dev` together: Vite on :5173 proxies `/api` to PocketBase.

Tests: `npm run backend:test` (import, quotas, claim, revoke, scanning, including 20 parallel scans of one pass letting it in exactly once). `npm run typecheck` for the web app.

## Deploy on a campus server (recommended)

```sh
npm ci && npm run build && npm run backend:build        # produces backend/garba-server and backend/pb_public
# copy both to the server:
sudo mkdir -p /opt/garba && sudo cp -r backend/garba-server backend/pb_public /opt/garba/
sudo useradd --system garba && sudo chown -R garba:garba /opt/garba
sudo cp deploy/garba.env.example /opt/garba/garba.env   # fill it in
sudo cp deploy/garba.service /etc/systemd/system/ && sudo systemctl enable --now garba
```

- Cross-compile from a laptop with `GOOS=linux GOARCH=amd64 npm run backend:build`.
- HTTPS: if the server has a public DNS name, change `ExecStart` to `serve garba.iima.ac.in` and PocketBase gets a Let's Encrypt certificate itself (ports 80/443). Or put it behind the campus reverse proxy. Camera scanning needs HTTPS.
- Create a superuser for the PocketBase dashboard: `./garba-server superuser upsert you@iima.ac.in 'a-long-password' --dir /opt/garba/pb_data`.
- Backups: the whole state is `/opt/garba/pb_data`. PocketBase can also schedule backups from the dashboard (Settings → Backups).
- Import the list from the admin screen (People → Import), or on the server: `./garba-server import people.csv --dir /opt/garba/pb_data`.

## Or: web app on Vercel, backend on campus

The web app is a static build, so it can be hosted on Vercel while PocketBase runs on campus. The campus server still needs a public HTTPS URL.

1. Import `arnav127/iima-garba` in Vercel. `vercel.json` already sets the build (`npm run build`, output `backend/pb_public`).
2. In Vercel → Settings → Environment Variables set `VITE_PB_URL=https://<your-campus-server>`.
3. On the server set `APP_URL=https://<your-vercel-domain>` so claim links point to the Vercel app, and add `https://<your-vercel-domain>/auth/callback` as a Google redirect URI.

## Configuration

All settings are environment variables (see `deploy/garba.env.example`). They're applied on every start; anything left empty keeps what's set in the PocketBase dashboard.

| Variable | What it does |
| --- | --- |
| `APP_URL` | Public URL of the web app, used in claim links and emails |
| `ADMIN_EMAILS` | Comma-separated emails that are always Cultcomm admins (created if missing) |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | Google sign-in. Redirect URI: `<APP_URL>/auth/callback` |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_TLS` | Email for sign-in codes and pass emails |
| `MAIL_FROM_ADDRESS`, `MAIL_FROM_NAME` | Sender |
| `VITE_PB_URL` (build time) | Only when the web app is hosted separately from PocketBase |

Event details (date, venue, gates) and quotas per group are edited in the app: Admin → Settings.

## The people list (CSV)

```csv
email,name,group,college,role
p25aarav@iima.ac.in,Aarav Shah,pgp1,,
aarav.shah@gmail.com,Aarav Shah,pgp1,,
p24meera@iima.ac.in,Meera Iyer,pgp2,,volunteer
ishaan.m@spjimr.org,Ishaan Mehta,exchange,SPJIMR Mumbai,
```

- `group`: `pgp1`, `pgp2`, `pgpx`, `phd`, `faculty`, `staff`, `exchange` (exchange rows need `college`).
- `role` (optional): `volunteer` or `admin`.
- Sign-in matches on email, so list the address people will sign in with: their Gmail or their @iima.ac.in Google account. Re-importing updates people; nobody gets a second pass.

## How entry works

Each pass has a random secret; the QR holds `GRB1:<secret>`, and only the pass holder ever receives it. Volunteers scan it (or type the printed `GRB-0417` code if a phone screen is cracked). The check-and-mark runs in one transaction, so a pass can only get in once even when two gates scan it at the same moment. Admins can undo an entry scanned by mistake.

Placeholders to confirm with Cultcomm: the date (Sat 17 Oct), venue (LKP), start time (8 PM), number of gates (2), and quotas (PGP1/PGP2/Faculty 4, PGPX/PhD/Staff 2). All of these can be changed in Admin → Settings.
