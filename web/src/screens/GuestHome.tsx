import type { ComponentChildren } from 'preact';
import { useState } from 'preact/hooks';
import type { EventInfo, MeResponse, PassView } from '../../../shared/types.ts';
import { Btn, Logo, Top, Zigzag } from '../components/ui.tsx';
import { firstName, istTime, navigate } from '../lib.ts';
import { AccountSheet } from './Home.tsx';

/** "You're on the list" layout shared by exchange guests, friends who signed in, and claim links. */
export function GuestPassLayout(props: {
  name: string;
  intro: ComponentChildren;
  label: string;
  pass: PassView;
  event: EventInfo;
  children?: ComponentChildren;   // above the card (e.g. name field)
  action: ComponentChildren;      // button inside the card
  note: string;
  headerRight?: ComponentChildren;
}) {
  const { pass, event } = props;
  const entry = pass.enteredAt ? `In · ${istTime(pass.enteredAt)}` : `${event.gates > 1 ? 'Any gate' : 'Gate 1'} · ${event.timeLabel}`;
  return (
    <div class="screen">
      <Top />
      <div style={{ padding: '16px 22px 0', position: 'relative' }}>
        {props.headerRight && <div style={{ position: 'absolute', right: 22, top: 16 }}>{props.headerRight}</div>}
        <div style={{ font: '700 22px var(--fd)', color: 'var(--pink)' }}>પધારો, {firstName(props.name)}!</div>
        <div class="h1" style={{ marginTop: 6 }}>YOU'RE ON<br />THE LIST</div>
        <div class="note" style={{ marginTop: 10 }}>{props.intro}</div>
      </div>
      {props.children}
      <div class="card-ink" style={{ margin: '20px 20px 0', borderRadius: 22 }}>
        <Zigzag color="var(--green)" h={16} />
        <div style={{ padding: '16px 18px', display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between' }}>
            <span style={{ font: "600 11px var(--fb)", letterSpacing: '.12em', color: 'var(--green)' }}>{props.label}</span>
            <span style={{ font: '500 12px ui-monospace, monospace', color: 'var(--soft)' }}>{pass.code}</span>
          </div>
          <span style={{ font: '800 30px/1 var(--fd)', fontStretch: '75%' }}>{pass.holderName.toUpperCase()}</span>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8, font: "400 11px var(--fb)", color: 'var(--soft)', letterSpacing: '.06em' }}>
            <div style={{ display: 'flex', flexDirection: 'column' }}>DATE<b style={{ font: "600 15px var(--fb)", color: 'var(--ivory)', letterSpacing: 0 }}>{event.dateLabel}</b></div>
            <div style={{ display: 'flex', flexDirection: 'column' }}>ENTRY<b style={{ font: "600 15px var(--fb)", color: 'var(--ivory)', letterSpacing: 0 }}>{entry}</b></div>
          </div>
          {props.action}
        </div>
      </div>
      <div style={{ margin: '16px 22px 0', display: 'flex', gap: 10, font: "400 14px/1.45 var(--fb)", color: 'var(--body)' }}>
        <span class="mirror-dot" style={{ marginTop: 4 }} />{props.note}
      </div>
      <div style={{ height: 'calc(40px + var(--safe-b))', flex: 'none' }} />
    </div>
  );
}

export function GuestHome({ me }: { me: MeResponse }) {
  const [account, setAccount] = useState(false);
  const pass = me.pass!;
  const exchange = pass.kind === 'exchange';
  return (
    <>
      <GuestPassLayout
        name={me.user.name}
        pass={pass}
        event={me.event}
        label={exchange ? 'PASS EXCHANGE GUEST' : 'GUEST PASS'}
        intro={exchange
          ? `Cultcomm added you through the pass exchange with ${me.user.college ?? 'your college'}.`
          : `${pass.issuerName ?? 'A friend'} sent you a pass to ${me.event.title} at IIM Ahmedabad.`}
        note={exchange
          ? "Carry your college ID. The name must match your pass, and the pass can't be transferred."
          : "Carry a photo ID. The name must match your pass, and the pass can't be transferred."}
        action={<Btn variant="flat" onClick={() => navigate('/pass')}>Show entry QR</Btn>}
        headerRight={<button onClick={() => setAccount(true)} aria-label="Account"><Logo size={40} /></button>}
      />
      {account && <AccountSheet me={me} onClose={() => setAccount(false)} />}
    </>
  );
}
