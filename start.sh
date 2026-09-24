#!/usr/bin/env bash
# Garba Night · IIMA — build and (re)start everything on the campus server.
#
#   ./start.sh          first run and every update (after git pull)
#
# What it does: creates .env if missing, installs Go if missing (into ./.tools, no sudo),
# installs npm packages, builds the web app for BASE_PATH, builds the server binary,
# creates the dashboard superuser (if set in .env), and starts/reloads it under PM2.
set -euo pipefail

cd "$(dirname "$0")"
ROOT="$(pwd)"
say() { printf '\n\033[1;35m▸ %s\033[0m\n' "$*"; }
die() { printf '\n\033[1;31m✗ %s\033[0m\n' "$*" >&2; exit 1; }

# ---------- .env ----------
if [ ! -f .env ]; then
  cp .env.example .env
  say "Created .env from .env.example"
  echo "  Fill in GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET and ADMIN_EMAILS in $ROOT/.env, then run ./start.sh again."
  exit 1
fi
set -a; . ./.env; set +a
: "${PORT:=8090}" "${BASE_PATH:=/garba2026/}" "${DATA_DIR:=./data}"
[ -n "${APP_URL:-}" ] || die "APP_URL is empty in .env"
[ -n "${GOOGLE_CLIENT_ID:-}" ] && [ -n "${GOOGLE_CLIENT_SECRET:-}" ] || echo "  ⚠ GOOGLE_CLIENT_ID/SECRET are empty: nobody can sign in until you set them."

# ---------- Node ----------
command -v node >/dev/null || die "Node.js 20+ is needed (e.g. https://github.com/nvm-sh/nvm, then: nvm install 22)"
NODE_MAJOR="$(node -p 'process.versions.node.split(".")[0]')"
[ "$NODE_MAJOR" -ge 20 ] || die "Node.js 20+ is needed (found $(node -v))"

# ---------- Go (installed locally if missing) ----------
export GOTOOLCHAIN=auto   # fetches the exact Go version go.mod asks for
if ! command -v go >/dev/null; then
  if [ ! -x .tools/go/bin/go ]; then
    say "Installing Go into ./.tools (no sudo needed)"
    case "$(uname -m)" in x86_64) ARCH=amd64 ;; aarch64|arm64) ARCH=arm64 ;; *) die "Unsupported CPU $(uname -m)" ;; esac
    GOVER="$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -n1)"
    mkdir -p .tools
    curl -fsSL "https://go.dev/dl/${GOVER}.linux-${ARCH}.tar.gz" | tar -xz -C .tools
  fi
  export PATH="$ROOT/.tools/go/bin:$PATH"
fi
echo "  go: $(go version)"

# ---------- Build ----------
say "Installing npm packages"
npm ci --no-audit --no-fund

say "Building the web app for ${BASE_PATH}"
VITE_BASE="$BASE_PATH" npm run build

say "Building the server"
(cd backend && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o garba-server .)

mkdir -p "$DATA_DIR" logs
DATA_DIR_ABS="$(cd "$DATA_DIR" && pwd)"

if [ -n "${SUPERUSER_EMAIL:-}" ] && [ -n "${SUPERUSER_PASSWORD:-}" ]; then
  say "Creating/updating dashboard superuser $SUPERUSER_EMAIL"
  ./backend/garba-server superuser upsert "$SUPERUSER_EMAIL" "$SUPERUSER_PASSWORD" --dir "$DATA_DIR_ABS"
fi

# ---------- PM2 ----------
PM2=(pm2)
if ! command -v pm2 >/dev/null; then
  say "Installing PM2"
  if ! npm install -g pm2 --no-audit --no-fund; then
    echo "  (no permission for a global install; using npx pm2)"
    PM2=(npx --yes pm2)
  fi
fi

say "Starting with PM2"
"${PM2[@]}" startOrReload ecosystem.config.cjs --update-env
"${PM2[@]}" save >/dev/null

# ---------- Health check ----------
for _ in $(seq 1 20); do
  if curl -fsS "http://127.0.0.1:${PORT}/api/health" >/dev/null 2>&1; then OK=1; break; fi
  sleep 0.5
done
[ "${OK:-}" = 1 ] || die "Server didn't answer on 127.0.0.1:${PORT}. Check: ${PM2[*]} logs garba2026"

say "Garba Night is running on 127.0.0.1:${PORT}"
cat <<EOF
  Public URL:   ${APP_URL}/
  Dashboard:    ${APP_URL}/_/
  Logs:         ${PM2[*]} logs garba2026
  Data:         ${DATA_DIR_ABS}   (back this up)

  First time only:
    • Start on boot:   ${PM2[*]} startup   (run the command it prints, with sudo)
    • Apache:          add the lines from deploy/apache-garba2026.conf, then
                       sudo a2enmod proxy proxy_http headers && sudo systemctl reload apache2
    • Google redirect: ${APP_URL}/auth/callback
EOF
