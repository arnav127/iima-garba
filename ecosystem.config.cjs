// PM2 process file for the campus server.
//   ./start.sh                        builds everything and runs this for you
//   pm2 startOrReload ecosystem.config.cjs --update-env    (after editing .env)
//   pm2 logs garba2026 · pm2 status · pm2 restart garba2026
//
// .cjs because package.json is "type": "module" (PM2 loads process files with require()).

const fs = require('node:fs');
const path = require('node:path');

const root = __dirname;

/** Minimal .env reader (KEY=value, # comments, optional quotes) so no extra packages are needed. */
function readEnv(file) {
  const out = {};
  if (!fs.existsSync(file)) return out;
  for (const line of fs.readFileSync(file, 'utf8').split(/\r?\n/)) {
    const m = line.match(/^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*?)\s*$/);
    if (!m || line.trim().startsWith('#')) continue;
    out[m[1]] = m[2].replace(/^(['"])(.*)\1$/, '$2');
  }
  return out;
}

const env = readEnv(path.join(root, '.env'));
const port = env.PORT || '8090';
const dataDir = path.resolve(root, env.DATA_DIR || './data');

module.exports = {
  apps: [
    {
      name: 'garba2026',
      cwd: root,
      script: path.join(root, 'backend/garba-server'),
      interpreter: 'none', // a compiled Go binary, not a Node script
      args: ['serve', '--http', `127.0.0.1:${port}`, '--dir', dataDir, '--publicDir', path.join(root, 'backend/pb_public')],
      env: { ...env, NODE_ENV: 'production' },
      autorestart: true,
      restart_delay: 2000,
      max_restarts: 50,
      kill_timeout: 10000, // let in-flight scans finish on restart
      max_memory_restart: '1G',
      time: true,
      out_file: path.join(root, 'logs/garba.out.log'),
      error_file: path.join(root, 'logs/garba.err.log'),
      merge_logs: true,
    },
  ],
};
