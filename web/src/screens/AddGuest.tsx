import { useState } from 'preact/hooks';
import type { MeResponse } from '../../../shared/types.ts';
import { Back, Btn, Top } from '../components/ui.tsx';
import { api, navigate, setMe, toast } from '../lib.ts';

export function AddGuest({ me }: { me: MeResponse }) {
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const left = me.remaining;

  async function add(e: Event) {
    e.preventDefault();
    if (left <= 0 || busy) return;
    setBusy(true); setError('');
    try {
      setMe(await api<MeResponse>('/guests', { body: { name, email } }));
      toast(`${name.trim().split(' ')[0]} is on the list`);
      navigate('/home', true);
    } catch (err) {
      setError((err as Error).message);
    } finally { setBusy(false); }
  }

  return (
    <form class="screen" onSubmit={add}>
      <Top />
      <Back onClick={() => navigate('/home')} />
      <div style={{ padding: '14px 22px 0' }}>
        <div class="h1">ADD A GUEST</div>
        <div class="note" style={{ marginTop: 8 }}>A friend or family member coming with you. Their pass shows up on your phone right away; swipe through everyone's QR at the gate.</div>
      </div>
      <div style={{ margin: '24px 22px 0', display: 'flex', flexDirection: 'column', gap: 16 }}>
        <div class="field">
          <label class="label" for="gname">NAME</label>
          <input id="gname" class="input" value={name} onInput={(e) => setName(e.currentTarget.value)} placeholder="As on their ID" autoComplete="off" autoCapitalize="words" required minLength={2} maxLength={80} />
        </div>
        <div class="field">
          <label class="label" for="gemail">THEIR GMAIL <span style={{ letterSpacing: 0, fontWeight: 400 }}>(optional)</span></label>
          <input id="gemail" class="input" type="email" value={email} onInput={(e) => setEmail(e.currentTarget.value)} placeholder="friend@gmail.com" autoComplete="off" autoCapitalize="none" spellcheck={false} />
          <span class="hint">Add it so they can sign in with Google and show their own QR. No email is sent. Leave it empty for family without phones; you can also share a pass link on WhatsApp.</span>
        </div>
      </div>
      <div style={{ margin: '18px 22px 0', padding: '14px 16px', borderRadius: 14, background: '#fff', border: '2px dashed var(--ink)', display: 'flex', alignItems: 'center', gap: 12, font: "500 14px var(--fb)" }}>
        <span style={{ font: '800 32px/.8 var(--fd)', color: 'var(--pink)' }}>{Math.max(left - 1, 0)}</span>
        guest {left - 1 === 1 ? 'pass' : 'passes'} left after this one
      </div>
      {error && <div class="error" role="alert" style={{ margin: '14px 22px 0' }}>{error}</div>}
      <div class="bottom">
        <Btn type="submit" disabled={left <= 0 || busy}>{left <= 0 ? 'No guest passes left' : busy ? 'Adding…' : 'Add guest'}</Btn>
      </div>
    </form>
  );
}
