import { useRef, useState } from 'preact/hooks';
import { Back, Btn, Top, Zigzag } from '../components/ui.tsx';
import { afterSignIn, navigate, pb, pbCall, startGoogleSignIn, storage } from '../lib.ts';
import { homeFor } from '../routes.ts';

const GoogleG = () => (
  <svg width="20" height="20" viewBox="0 0 48 48" aria-hidden="true" style={{ background: '#fff', borderRadius: 4, padding: 2, boxSizing: 'content-box' }}>
    <path fill="#EA4335" d="M24 9.5c3.5 0 6.6 1.2 9.1 3.6l6.8-6.8C35.8 2.4 30.3 0 24 0 14.6 0 6.6 5.4 2.7 13.3l7.9 6.2C12.5 13.6 17.8 9.5 24 9.5z" />
    <path fill="#4285F4" d="M46.1 24.5c0-1.6-.1-3.1-.4-4.5H24v9h12.4c-.5 2.9-2.2 5.3-4.6 6.9l7.4 5.8c4.3-4 6.9-9.9 6.9-17.2z" />
    <path fill="#FBBC05" d="M10.6 28.5c-.5-1.4-.8-2.9-.8-4.5s.3-3.1.8-4.5l-7.9-6.2C1 16.6 0 20.2 0 24s1 7.4 2.7 10.7l7.9-6.2z" />
    <path fill="#34A853" d="M24 48c6.5 0 11.9-2.1 15.9-5.8l-7.4-5.8c-2.1 1.4-4.8 2.3-8.5 2.3-6.2 0-11.5-4.1-13.4-9.8l-7.9 6.2C6.6 42.6 14.6 48 24 48z" />
  </svg>
);

export function Login() {
  const params = new URLSearchParams(location.search);
  const [email, setEmail] = useState(storage.get<string>('garba:email') ?? '');
  const [code, setCode] = useState('');
  const [otpId, setOtpId] = useState<string | null>(null);
  const [busy, setBusy] = useState<'' | 'google' | 'otp'>('');
  const [error, setError] = useState(params.get('error') ?? '');
  const otpRef = useRef<HTMLInputElement>(null);

  const cleanEmail = email.trim().toLowerCase();

  async function google() {
    setBusy('google'); setError('');
    try {
      await startGoogleSignIn();
    } catch (e) {
      setError((e as Error).message); setBusy('');
    }
  }

  async function sendCode() {
    setBusy('otp'); setError('');
    try {
      const r = await pbCall(() => pb.collection('users').requestOTP(cleanEmail));
      storage.set('garba:email', cleanEmail);
      setOtpId(r.otpId); setCode('');
      setTimeout(() => otpRef.current?.focus(), 50);
    } catch (e) {
      setError((e as Error).message);
    } finally { setBusy(''); }
  }

  async function verify(c = code) {
    if (c.length !== 6 || !otpId || busy) return;
    setBusy('otp'); setError('');
    try {
      await pbCall(() => pb.collection('users').authWithOTP(otpId, c));
      const me = await afterSignIn();
      navigate(homeFor(me), true);
    } catch (e) {
      setError((e as Error).message.includes('authenticate') ? "That code doesn't match. Check your email and try again." : (e as Error).message);
      setCode('');
      otpRef.current?.focus();
    } finally { setBusy(''); }
  }

  const onCode = (v: string) => {
    const c = v.replace(/\D/g, '').slice(0, 6);
    setCode(c);
    if (c.length === 6) verify(c);
  };

  const boxes = Array.from({ length: 6 }, (_, i) => code[i] ?? '');
  const active = otpId ? Math.min(code.length, 5) : -1;

  return (
    <div class="screen">
      <Top />
      <Back onClick={() => (otpId ? setOtpId(null) : navigate('/'))} />
      <div style={{ padding: '26px 24px 0' }}>
        <div style={{ font: '700 44px/1 var(--fd)', color: 'var(--pink)' }}>કેમ છો!</div>
        <div class="h1" style={{ marginTop: 8 }}>SIGN IN FOR<br />YOUR PASS</div>
      </div>

      <div style={{ margin: '28px 22px 0', display: 'flex', flexDirection: 'column', gap: 10 }}>
        <button type="button" class="btn" onClick={google} disabled={!!busy}>
          <span style={{ display: 'flex', alignItems: 'center', gap: 12 }}><GoogleG />{busy === 'google' ? 'Opening Google…' : 'Continue with Google'}</span>
          <span aria-hidden="true">→</span>
        </button>
        <span class="hint">Gmail or your @iima.ac.in Google account, whichever is on the Cultcomm list</span>
      </div>

      <div style={{ margin: '22px 22px 0', display: 'flex', alignItems: 'center', gap: 12 }}>
        <span style={{ flex: 1, height: 2, background: 'var(--ink)' }} />
        <span class="label">OR GET AN EMAIL CODE</span>
        <span style={{ flex: 1, height: 2, background: 'var(--ink)' }} />
      </div>

      <form
        style={{ margin: '18px 22px 0', display: 'flex', flexDirection: 'column', gap: 18 }}
        onSubmit={(e) => { e.preventDefault(); otpId ? verify() : sendCode(); }}
      >
        <div class="field">
          <label class="label" for="email">EMAIL</label>
          <input
            id="email" class="input" type="email" inputMode="email" autoComplete="email" autoCapitalize="none" spellcheck={false}
            placeholder="you@gmail.com" value={email}
            onInput={(e) => { setEmail(e.currentTarget.value); setOtpId(null); }}
          />
        </div>
        <div class="field">
          <label class="label" for="otp">6-DIGIT OTP</label>
          <div class={`otp ${otpId ? '' : 'off'}`}>
            {boxes.map((d, i) => <div key={i} class={`otp-box ${i === active && !d ? 'on' : ''}`}>{d}</div>)}
            <input
              id="otp" ref={otpRef} value={code} disabled={!otpId} inputMode="numeric" autoComplete="one-time-code" maxLength={6} pattern="[0-9]*"
              aria-label="6-digit code" onInput={(e) => onCode(e.currentTarget.value)}
            />
          </div>
          {otpId && (
            <span class="hint">
              Code sent to {cleanEmail}. <button type="button" style={{ textDecoration: 'underline' }} onClick={sendCode} disabled={!!busy}>Resend</button>
            </span>
          )}
        </div>
        {error && <div class="error" role="alert">{error}</div>}
        <Btn type="submit" variant="ghost" disabled={!!busy || (!otpId && !cleanEmail.includes('@')) || (!!otpId && code.length !== 6)}>
          {busy === 'otp' ? 'Please wait…' : otpId ? 'Verify' : 'Send code'}
        </Btn>
      </form>
      <div style={{ marginTop: 'auto', paddingTop: 36 }}>
        <Zigzag color="var(--yellow)" h={44} />
        <div style={{ height: 'var(--safe-b)', background: 'var(--ink)' }} />
      </div>
    </div>
  );
}
