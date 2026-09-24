import { useEffect, useState } from 'preact/hooks';
import { Back, Icon, Top, Zigzag } from '../components/ui.tsx';
import { enabledProviders, navigate, startSignIn, storage, type Provider } from '../lib.ts';

const GoogleG = () => (
  <svg width="20" height="20" viewBox="0 0 48 48" aria-hidden="true" style={{ background: '#fff', borderRadius: 4, padding: 2, boxSizing: 'content-box' }}>
    <path fill="#EA4335" d="M24 9.5c3.5 0 6.6 1.2 9.1 3.6l6.8-6.8C35.8 2.4 30.3 0 24 0 14.6 0 6.6 5.4 2.7 13.3l7.9 6.2C12.5 13.6 17.8 9.5 24 9.5z" />
    <path fill="#4285F4" d="M46.1 24.5c0-1.6-.1-3.1-.4-4.5H24v9h12.4c-.5 2.9-2.2 5.3-4.6 6.9l7.4 5.8c4.3-4 6.9-9.9 6.9-17.2z" />
    <path fill="#FBBC05" d="M10.6 28.5c-.5-1.4-.8-2.9-.8-4.5s.3-3.1.8-4.5l-7.9-6.2C1 16.6 0 20.2 0 24s1 7.4 2.7 10.7l7.9-6.2z" />
    <path fill="#34A853" d="M24 48c6.5 0 11.9-2.1 15.9-5.8l-7.4-5.8c-2.1 1.4-4.8 2.3-8.5 2.3-6.2 0-11.5-4.1-13.4-9.8l-7.9 6.2C6.6 42.6 14.6 48 24 48z" />
  </svg>
);

const MicrosoftM = () => (
  <svg width="20" height="20" viewBox="0 0 22 22" aria-hidden="true" style={{ background: '#fff', borderRadius: 4, padding: 2, boxSizing: 'content-box' }}>
    <path fill="#f25022" d="M1 1h9.5v9.5H1z" /><path fill="#7fba00" d="M11.5 1H21v9.5h-9.5z" />
    <path fill="#00a4ef" d="M1 11.5h9.5V21H1z" /><path fill="#ffb900" d="M11.5 11.5H21V21h-9.5z" />
  </svg>
);

export function Login() {
  const params = new URLSearchParams(location.search);
  const [providers, setProviders] = useState<Provider[]>(['google']);
  const [busy, setBusy] = useState<Provider | ''>('');
  const [error, setError] = useState(params.get('error') ?? '');
  const linkToken = storage.get<string>('garba:link');

  useEffect(() => { enabledProviders().then((p) => p.length && setProviders(p)); }, []);

  async function go(p: Provider) {
    setBusy(p); setError('');
    try {
      await startSignIn(p);
    } catch (e) {
      setError((e as Error).message); setBusy('');
    }
  }

  return (
    <div class="screen">
      <Top />
      <Back onClick={() => navigate('/')} />
      <div style={{ padding: '26px 24px 0' }}>
        <div style={{ font: '700 44px/1 var(--fd)', color: 'var(--pink)' }}>કેમ છો!</div>
        <div class="h1" style={{ marginTop: 8 }}>SIGN IN FOR<br />YOUR PASS</div>
      </div>

      <div style={{ margin: '30px 22px 0', display: 'flex', flexDirection: 'column', gap: 14 }}>
        <button type="button" class="btn" onClick={() => go('google')} disabled={!!busy}>
          <span style={{ display: 'flex', alignItems: 'center', gap: 12 }}><GoogleG />{busy === 'google' ? 'Opening Google…' : 'Continue with Google'}</span>
          <Icon name="arrow" />
        </button>
        {providers.includes('microsoft') && (
          <button type="button" class="btn ghost" onClick={() => go('microsoft')} disabled={!!busy}>
            <span style={{ display: 'flex', alignItems: 'center', gap: 12 }}><MicrosoftM />{busy === 'microsoft' ? 'Opening Microsoft…' : 'Continue with Microsoft'}</span>
            <Icon name="arrow" />
          </button>
        )}
        {error && <div class="error" role="alert">{error}</div>}
      </div>

      <div style={{ margin: '28px 22px 0', display: 'flex', flexDirection: 'column', gap: 14 }}>
        {[
          ['IIMA students, faculty & staff', 'Use your @iima.ac.in account. Your pass is ready the moment you sign in.'],
          ['Friends & family', 'Use the Gmail your host added, or open the pass link they shared with you.'],
          ['Exchange guests', 'Use the email your college gave Cultcomm, or the pass link you were sent.'],
        ].map(([t, d]) => (
          <div key={t} style={{ display: 'flex', gap: 10 }}>
            <span class="mirror-dot" style={{ marginTop: 5 }} />
            <div class="note" style={{ fontSize: 14 }}><b style={{ color: 'var(--ink)' }}>{t}</b><br />{d}</div>
          </div>
        ))}
        {linkToken && <button class="btn ghost small" onClick={() => navigate(`/p/${linkToken}`)}><span>Open my saved pass link</span><Icon name="arrow" /></button>}
      </div>

      <div style={{ marginTop: 'auto', paddingTop: 36 }}>
        <Zigzag color="var(--yellow)" h={44} />
        <div style={{ height: 'var(--safe-b)', background: 'var(--ink)' }} />
      </div>
    </div>
  );
}
