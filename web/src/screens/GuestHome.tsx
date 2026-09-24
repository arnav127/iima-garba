import type { ComponentChildren } from 'preact';
import { useCallback, useEffect, useState } from 'preact/hooks';
import { TONES, type EventInfo, type LinkResponse, type MeResponse, type PassView } from '../../../shared/types.ts';
import { Btn, Logo, Spinner, Top, Zigzag } from '../components/ui.tsx';
import { api, firstName, istTime, navigate, storage } from '../lib.ts';
import { syncClock } from '../qr.ts';
import { AccountSheet } from './Home.tsx';
import { PassQr } from './PassQr.tsx';

/** "You're on the list" layout for guests and exchange guests (signed in or via a pass link). */
export function GuestPassLayout(props: {
  pass: PassView;
  event: EventInfo;
  onShow: () => void;
  headerRight?: ComponentChildren;
}) {
  const { pass, event } = props;
  const tone = TONES[pass.tone];
  const exchange = pass.kind === 'exchange';
  const entry = pass.enteredAt ? `In · ${istTime(pass.enteredAt)} · Gate ${pass.enteredGate}` : `${event.gates > 1 ? 'Any gate' : 'Gate 1'} · ${event.timeLabel}`;
  return (
    <div class="screen">
      <Top />
      <div style={{ padding: '16px 22px 0', position: 'relative' }}>
        {props.headerRight && <div style={{ position: 'absolute', right: 22, top: 16 }}>{props.headerRight}</div>}
        <div style={{ font: '700 22px var(--fd)', color: 'var(--pink)' }}>પધારો, {firstName(pass.holderName)}!</div>
        <div class="h1" style={{ marginTop: 6 }}>YOU'RE ON<br />THE LIST</div>
        <div class="note" style={{ marginTop: 10 }}>
          {exchange
            ? `Cultcomm added you through the pass exchange${pass.college ? ` with ${pass.college}` : ''}.`
            : `${pass.issuerName ?? 'A friend'} added you as their guest for ${event.title} at IIM Ahmedabad: ${event.venue}, ${event.dateLabel}, from ${event.timeLabel}.`}
        </div>
      </div>
      <div class="card-ink" style={{ margin: '20px 20px 0', borderRadius: 22 }}>
        <Zigzag color={tone.color} h={16} />
        <div style={{ padding: '16px 18px', display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between' }}>
            <span style={{ font: "600 11px var(--fb)", letterSpacing: '.12em', color: tone.color }}>{exchange ? 'PASS EXCHANGE GUEST' : 'GUEST PASS'}</span>
            <span style={{ font: '500 12px ui-monospace, monospace', color: 'var(--soft)' }}>{pass.code}</span>
          </div>
          <span style={{ font: '800 30px/1 var(--fd)', fontStretch: '75%' }}>{pass.holderName.toUpperCase()}</span>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8, font: "400 11px var(--fb)", color: 'var(--soft)', letterSpacing: '.06em' }}>
            <div style={{ display: 'flex', flexDirection: 'column' }}>DATE<b style={{ font: "600 15px var(--fb)", color: 'var(--ivory)', letterSpacing: 0 }}>{event.dateLabel}</b></div>
            <div style={{ display: 'flex', flexDirection: 'column' }}>ENTRY<b style={{ font: "600 15px var(--fb)", color: 'var(--ivory)', letterSpacing: 0 }}>{entry}</b></div>
          </div>
          <Btn variant="flat" onClick={props.onShow}>{pass.enteredAt ? 'View pass' : 'Show entry QR'}</Btn>
        </div>
      </div>
      <div style={{ margin: '16px 22px 0', display: 'flex', gap: 10, font: "400 14px/1.45 var(--fb)", color: 'var(--body)' }}>
        <span class="mirror-dot" style={{ marginTop: 4 }} />
        Carry your {tone.id}. The name must match your pass, and the pass can't be transferred.
      </div>
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
  const [showQr, setShowQr] = useState(false);

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
  if (showQr) return <PassQr passes={[data.pass]} event={data.event} onBack={() => setShowQr(false)} refresh={load} />;
  return <GuestPassLayout pass={data.pass} event={data.event} onShow={() => setShowQr(true)} />;
}
