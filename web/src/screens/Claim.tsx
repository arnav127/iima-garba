import { useEffect, useState } from 'preact/hooks';
import type { ClaimResponse } from '../../../shared/types.ts';
import { Btn, Spinner, Top } from '../components/ui.tsx';
import { api, navigate, storage } from '../lib.ts';
import { GuestPassLayout } from './GuestHome.tsx';
import { PassQr } from './PassQr.tsx';

/** Friends open /claim/<token> from the link they were sent. No sign-in needed. */
export function Claim({ token }: { token: string }) {
  const cacheKey = `garba:claim:${token}`;
  const [data, setData] = useState<ClaimResponse | null>(() => storage.get<ClaimResponse>(cacheKey));
  const [error, setError] = useState('');
  const [name, setName] = useState('');
  const [busy, setBusy] = useState(false);
  const [showQr, setShowQr] = useState(false);

  const save = (d: ClaimResponse) => {
    setData(d);
    if (d.claimed) { storage.set(cacheKey, d); storage.set('garba:claim', token); }
  };

  useEffect(() => {
    api<ClaimResponse>(`/claim/${token}`)
      .then((d) => { save(d); setName((n) => n || d.pass.holderName); })
      .catch((e) => { if (!data || e.status !== 0) { setError(e.message); storage.set(cacheKey, null); } });
  }, [token]);

  async function claim() {
    setBusy(true); setError('');
    try {
      save(await api<ClaimResponse>(`/claim/${token}`, { body: { name } }));
    } catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }

  if (error && !data) {
    return (
      <div class="screen">
        <Top />
        <div style={{ padding: '40px 22px 0' }}>
          <div class="guj">માફ કરશો</div>
          <div class="h1" style={{ marginTop: 8 }}>PASS NOT<br />FOUND</div>
          <div class="note" style={{ marginTop: 12 }}>{error} Ask the person who sent it to share a new link.</div>
        </div>
        <div class="bottom"><Btn onClick={() => navigate('/')}>Go to Garba Night</Btn></div>
      </div>
    );
  }
  if (!data) return <div class="screen"><Spinner /></div>;
  if (showQr && data.claimed) return <PassQr pass={data.pass} event={data.event} onBack={() => setShowQr(false)} />;

  const { pass, event, claimed } = data;
  return (
    <GuestPassLayout
      name={claimed ? pass.holderName : name || pass.holderName}
      pass={claimed ? pass : { ...pass, holderName: name || pass.holderName }}
      event={event}
      label={claimed ? 'GUEST PASS' : 'GUEST PASS · NOT CLAIMED'}
      intro={`${pass.issuerName ?? 'A friend'} sent you a pass to ${event.title} at IIM Ahmedabad: ${event.venue}, ${event.dateLabel}, from ${event.timeLabel}.`}
      note="Carry a photo ID. The name must match your pass, and the pass can't be transferred."
      action={claimed
        ? <Btn variant="flat" onClick={() => setShowQr(true)}>Show entry QR</Btn>
        : <Btn variant="flat" disabled={busy || name.trim().length < 2} onClick={claim}>{busy ? 'Claiming…' : 'Claim my pass'}</Btn>}
    >
      {!claimed && (
        <div class="field" style={{ margin: '18px 22px 0' }}>
          <label class="label" for="cname">YOUR NAME AS ON YOUR ID</label>
          <input id="cname" class="input" value={name} onInput={(e) => setName(e.currentTarget.value)} autoComplete="name" />
          {error && <span class="error">{error}</span>}
        </div>
      )}
    </GuestPassLayout>
  );
}
