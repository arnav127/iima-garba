import type { ComponentChildren } from 'preact';
import { useCallback, useEffect, useState } from 'preact/hooks';
import { TONES, type EventInfo, type LinkResponse, type MeResponse, type PassView } from '../../../shared/types.ts';
import { Btn, ConfirmBtn, Logo, Sheet, Spinner, Top, Zigzag } from '../components/ui.tsx';
import { api, firstName, initials, istTime, navigate, storage, toast, toastError } from '../lib.ts';
import { syncClock } from '../qr.ts';
import { AccountSheet } from './Home.tsx';
import { PassQr } from './PassQr.tsx';

/** "You're on the list" layout for guests and exchange guests (signed in or via a pass link). */
export function GuestPassLayout(props: {
  pass: PassView;
  event: EventInfo;
  onShow: () => void;
  headerRight?: ComponentChildren;
  /** How many QRs "Show entry QR" opens (the holder's and their guests'). */
  qrCount?: number;
  children?: ComponentChildren;
}) {
  const { pass, event } = props;
  const tone = TONES[pass.tone];
  const exchange = pass.kind === 'exchange';
  const alum = exchange && /alum/i.test(pass.college ?? ''); // alumni are added through the exchange list
  const entry = pass.enteredAt ? `In · ${istTime(pass.enteredAt)} · Gate ${pass.enteredGate}` : `${event.gates > 1 ? 'Any gate' : 'Gate 1'} · ${event.timeLabel}`;
  return (
    <div class="screen">
      <Top />
      <div style={{ padding: '16px 22px 0', position: 'relative' }}>
        {props.headerRight && <div style={{ position: 'absolute', right: 22, top: 16 }}>{props.headerRight}</div>}
        <div style={{ font: '700 22px var(--fd)', color: 'var(--pink)' }}>પધારો, {firstName(pass.holderName)}!</div>
        <div class="h1" style={{ marginTop: 6 }}>YOU'RE ON<br />THE LIST</div>
        <div class="note" style={{ marginTop: 10 }}>
          {alum
            ? `Welcome back to campus. Cultcomm has a pass for you for ${event.title}: ${event.venue}, ${event.dateLabel}, from ${event.timeLabel}.`
            : exchange
            ? `Cultcomm added you through the pass exchange${pass.college ? ` with ${pass.college}` : ''}.`
            : `${pass.issuerName ?? 'A friend'} added you as their guest for ${event.title} at IIM Ahmedabad: ${event.venue}, ${event.dateLabel}, from ${event.timeLabel}.`}
        </div>
      </div>
      <div class="card-ink" style={{ margin: '20px 20px 0', borderRadius: 22 }}>
        <Zigzag color={tone.color} h={16} />
        <div style={{ padding: '16px 18px', display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between' }}>
            <span style={{ font: "600 11px var(--fb)", letterSpacing: '.12em', color: tone.color }}>{alum ? 'ALUMNI PASS' : exchange ? 'PASS EXCHANGE GUEST' : 'GUEST PASS'}</span>
            <span style={{ font: '500 12px ui-monospace, monospace', color: 'var(--soft)' }}>{pass.code}</span>
          </div>
          <span style={{ font: '800 30px/1 var(--fd)', fontStretch: '75%' }}>{pass.holderName.toUpperCase()}</span>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8, font: "400 11px var(--fb)", color: 'var(--soft)', letterSpacing: '.06em' }}>
            <div style={{ display: 'flex', flexDirection: 'column' }}>DATE<b style={{ font: "600 15px var(--fb)", color: 'var(--ivory)', letterSpacing: 0 }}>{event.dateLabel}</b></div>
            <div style={{ display: 'flex', flexDirection: 'column' }}>ENTRY<b style={{ font: "600 15px var(--fb)", color: 'var(--ivory)', letterSpacing: 0 }}>{entry}</b></div>
          </div>
          <Btn variant="flat" onClick={props.onShow}>{(props.qrCount ?? 1) > 1 ? `Show all ${props.qrCount} entry QRs` : pass.enteredAt ? 'View pass' : 'Show entry QR'}</Btn>
        </div>
      </div>
      <div style={{ margin: '16px 22px 0', display: 'flex', gap: 10, font: "400 14px/1.45 var(--fb)", color: 'var(--body)' }}>
        <span class="mirror-dot" style={{ marginTop: 4 }} />
        Carry {alum ? 'a photo ID' : `your ${tone.id}`}. The name must match your pass, and the pass can't be transferred.
      </div>
      {props.children}
      <div style={{ height: 'calc(40px + var(--safe-b))', flex: 'none' }} />
    </div>
  );
}

export function GuestHome({ me }: { me: MeResponse }) {
  const [account, setAccount] = useState(false);
  return (
    <>
      <GuestPassLayout pass={me.pass!} event={me.event} onShow={() => navigate('/pass')}
        headerRight={<button onClick={() => setAccount(true)} aria-label="Account"><Logo size={40} /></button>} />
      {account && <AccountSheet me={me} onClose={() => setAccount(false)} />}
    </>
  );
}

/** /p/<token>: a guest's pass without signing in (shared by their host on WhatsApp, or by Cultcomm). */
export function PassLink({ token }: { token: string }) {
  const cacheKey = `garba:p:${token}`;
  const [data, setData] = useState<LinkResponse | null>(() => storage.get<LinkResponse>(cacheKey));
  const [error, setError] = useState('');
  const [showQr, setShowQr] = useState<number | null>(null);
  const [name, setName] = useState('');
  const [busy, setBusy] = useState(false);
  const [sel, setSel] = useState<{ pass: PassView; index: number } | null>(null);

  const load = useCallback(() => {
    api<LinkResponse>(`/p/${token}`)
      .then((d) => { syncClock(d.serverTime); setData(d); storage.set(cacheKey, d); storage.set('garba:link', token); })
      .catch((e) => { if (e.status !== 0) { setError(e.message); setData(null); storage.set(cacheKey, null); storage.set('garba:link', null); } });
  }, [token]);
  useEffect(load, [token]);

  if (error && !data) {
    return (
      <div class="screen">
        <Top />
        <div style={{ padding: '40px 22px 0' }}>
          <div class="guj">માફ કરશો</div>
          <div class="h1" style={{ marginTop: 8 }}>PASS NOT<br />FOUND</div>
          <div class="note" style={{ marginTop: 12 }}>{error}. Ask the person who added you for a new link.</div>
        </div>
        <div class="bottom"><Btn onClick={() => navigate('/')}>Go to Garba Night</Btn></div>
      </div>
    );
  }
  if (!data) return <div class="screen"><Spinner /></div>;
  const guests = data.guests ?? []; // a copy saved by an older version of the app has no guests yet
  const limit = data.limit ?? 0;
  const passes = [data.pass, ...guests];
  if (showQr !== null) return <PassQr passes={passes} start={showQr} event={data.event} onBack={() => setShowQr(null)} refresh={load} />;

  const save = (d: LinkResponse) => { setData(d); storage.set(cacheKey, d); };
  async function add(e: Event) {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    try {
      save(await api<LinkResponse>(`/p/${token}/guests`, { body: { name } }));
      toast(`${firstName(name.trim())} is on the list`);
      setName('');
    } catch (err) { toastError(err); } finally { setBusy(false); }
  }
  async function remove(p: PassView) {
    setBusy(true);
    try {
      save(await api<LinkResponse>(`/p/${token}/guests/${p.id}/remove`, { body: {} }));
      setSel(null);
      toast(`${p.holderName}'s pass removed. You can add someone else.`);
    } catch (err) { toastError(err); } finally { setBusy(false); }
  }

  return (
    <>
      <GuestPassLayout pass={data.pass} event={data.event} onShow={() => setShowQr(0)} qrCount={passes.length}>
        {limit > 0 && (
          <>
            <div class="label" style={{ padding: '24px 22px 8px' }}>YOUR FAMILY & GUESTS · {guests.length} OF {limit}</div>
            {guests.length > 0 && (
              <div style={{ margin: '0 20px', borderTop: '2px solid var(--ink)' }}>
                {guests.map((p, i) => (
                  <button key={p.id} class="row" onClick={() => setSel({ pass: p, index: i + 1 })}>
                    <div class="av" style={{ background: TONES.guest.color }}>{initials(p.holderName)}</div>
                    <div class="row-main"><span class="row-title">{p.holderName}</span><span class="row-sub">{p.code} · their pass is on this link</span></div>
                    <span class="status" style={{ color: p.enteredAt ? 'var(--ok-text)' : 'var(--orange)' }}>{p.enteredAt ? 'IN' : 'READY'}</span>
                  </button>
                ))}
              </div>
            )}
            {data.remaining > 0 ? (
              <form class="box" style={{ margin: '14px 20px 0', padding: 16, display: 'flex', flexDirection: 'column', gap: 10 }} onSubmit={add}>
                <label class="label" for="famname">ADD A FAMILY MEMBER OR GUEST · {data.remaining} LEFT</label>
                <input id="famname" class="input" value={name} onInput={(e) => setName(e.currentTarget.value)} placeholder="Name, as on their ID" autoComplete="off" autoCapitalize="words" required minLength={2} maxLength={80} />
                <Btn type="submit" variant="yellow" icon="plus" disabled={busy || name.trim().length < 2}>{busy ? 'Adding…' : 'Add to my passes'}</Btn>
              </form>
            ) : (
              <div class="hint" style={{ margin: '12px 22px 0', fontSize: 13 }}>You've added all {limit} guests. Remove one to add someone else.</div>
            )}
            <div class="hint" style={{ margin: '10px 22px 0', fontSize: 13, lineHeight: 1.45 }}>Their passes live on this same link. At the gate, open the entry QRs and swipe through everyone's. Keep this link private: anyone who has it can use these passes.</div>
          </>
        )}
      </GuestPassLayout>
      {sel && (
        <Sheet onClose={() => setSel(null)} stripe={TONES.guest.color}>
          <div style={{ display: 'flex', flexDirection: 'column' }}>
            <span class="label">{sel.pass.code} · GUEST</span>
            <span style={{ font: '800 30px/1 var(--fd)', fontStretch: '75%' }}>{sel.pass.holderName.toUpperCase()}</span>
            {sel.pass.enteredAt && <span class="note" style={{ marginTop: 6 }}>Scanned at {istTime(sel.pass.enteredAt)} · Gate {sel.pass.enteredGate}.</span>}
          </div>
          {!sel.pass.enteredAt && <Btn onClick={() => { setShowQr(sel.index); setSel(null); }}>Show their QR</Btn>}
          {!sel.pass.enteredAt && <ConfirmBtn variant="ghost" disabled={busy} icon="x" confirmLabel="Tap again · their QR stops working" onConfirm={() => remove(sel.pass)}>Remove guest</ConfirmBtn>}
          {sel.pass.enteredAt && <Btn variant="ghost" icon="check" onClick={() => setSel(null)}>Done</Btn>}
        </Sheet>
      )}
    </>
  );
}
