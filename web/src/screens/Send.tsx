import { useState } from 'preact/hooks';
import type { MeResponse, SendResponse } from '../../../shared/types.ts';
import { Back, Btn, Top } from '../components/ui.tsx';
import { api, navigate, setMe } from '../lib.ts';
import { setJustSent } from './Home.tsx';

export function Send({ me }: { me: MeResponse }) {
  const [name, setName] = useState('');
  const [contact, setContact] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const canSend = me.remaining > 0;

  async function send(e: Event) {
    e.preventDefault();
    if (!canSend || busy) return;
    setBusy(true); setError('');
    try {
      const r = await api<SendResponse>('/passes', { body: { name, contact } });
      setMe(r.me);
      setJustSent({ pass: r.pass, claimUrl: r.claimUrl });
      navigate('/home', true);
    } catch (err) {
      setError((err as Error).message);
    } finally { setBusy(false); }
  }

  return (
    <form class="screen" onSubmit={send}>
      <Top />
      <Back onClick={() => navigate('/home')} />
      <div style={{ padding: '14px 22px 0' }}>
        <div class="h1">SEND A PASS</div>
        <div class="note" style={{ marginTop: 8 }}>They'll get an email to claim it in their name. You can take it back until they claim it.</div>
      </div>
      <div style={{ margin: '24px 22px 0', display: 'flex', flexDirection: 'column', gap: 16 }}>
        <div class="field">
          <label class="label" for="fname">FRIEND'S NAME</label>
          <input id="fname" class="input" value={name} onInput={(e) => setName(e.currentTarget.value)} placeholder="As on their ID" autoComplete="off" autoCapitalize="words" required minLength={2} maxLength={80} />
        </div>
        <div class="field">
          <label class="label" for="fcontact">EMAIL OR PHONE</label>
          <input id="fcontact" class="input" value={contact} onInput={(e) => setContact(e.currentTarget.value)} placeholder="friend@gmail.com" autoComplete="off" autoCapitalize="none" spellcheck={false} required />
          <span class="hint">With a phone number, you'll get a link to share on WhatsApp.</span>
        </div>
      </div>
      <div style={{ margin: '18px 22px 0', padding: '14px 16px', borderRadius: 14, background: '#fff', border: '2px dashed var(--ink)', display: 'flex', alignItems: 'center', gap: 12, font: "500 14px var(--fb)" }}>
        <span style={{ font: '800 32px/.8 var(--fd)', color: 'var(--pink)' }}>{Math.max(me.remaining - 1, 0)}</span>
        {me.remaining - 1 === 1 ? 'pass' : 'passes'} left after this one goes out
      </div>
      {error && <div class="error" role="alert" style={{ margin: '14px 22px 0' }}>{error}</div>}
      <div class="bottom">
        <Btn type="submit" disabled={!canSend || busy}>{!canSend ? 'No passes left' : busy ? 'Sending…' : 'Send pass'}</Btn>
      </div>
    </form>
  );
}
