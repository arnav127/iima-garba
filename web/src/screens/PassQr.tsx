import { useEffect, useRef, useState } from 'preact/hooks';
import { TONES, type EventInfo, type PassView } from '../../../shared/types.ts';
import { Back, Mirrors, QrCode, Rainbow, Zigzag } from '../components/ui.tsx';
import { istTime } from '../lib.ts';
import { QR_STEP, qrPayload, qrStep, serverNow } from '../qr.ts';

const clockFmt = new Intl.DateTimeFormat('en-IN', { hour: 'numeric', minute: '2-digit', second: '2-digit', hour12: true, timeZone: 'Asia/Kolkata' });

/**
 * Entry passes with a rotating QR (new code every 15 s, computed on the phone).
 * Several passes (yours + your guests) can be swiped through at the gate.
 */
export function PassQr({ passes, event, start = 0, onBack, refresh }: {
  passes: PassView[];
  event: EventInfo;
  start?: number;
  onBack: () => void;
  /** Re-fetches the passes so a scan at the gate shows up here. */
  refresh?: () => void;
}) {
  const [now, setNow] = useState(serverNow());
  const [index, setIndex] = useState(Math.min(start, passes.length - 1));
  const track = useRef<HTMLDivElement>(null);

  // Tick every second: live clock + QR refresh.
  useEffect(() => {
    const t = setInterval(() => setNow(serverNow()), 1000);
    return () => clearInterval(t);
  }, []);

  // Keep the screen awake while showing the pass (where supported).
  useEffect(() => {
    let lock: { release: () => Promise<void> } | undefined;
    (navigator as unknown as { wakeLock?: { request: (t: string) => Promise<typeof lock> } }).wakeLock?.request('screen').then((l) => { lock = l; }).catch(() => {});
    return () => { lock?.release().catch(() => {}); };
  }, []);

  // Pick up "scanned" status quickly while the pass is open.
  useEffect(() => {
    if (!refresh) return;
    const t = setInterval(() => { if (document.visibilityState === 'visible') refresh(); }, 5000);
    return () => clearInterval(t);
  }, [refresh]);

  useEffect(() => {
    const el = track.current;
    if (el && start > 0) el.scrollTo({ left: el.clientWidth * start, behavior: 'instant' as ScrollBehavior });
  }, []);

  const onScroll = () => {
    const el = track.current;
    if (el) setIndex(Math.round(el.scrollLeft / el.clientWidth));
  };
  const goTo = (i: number) => track.current?.scrollTo({ left: track.current.clientWidth * i, behavior: 'smooth' });

  const secsLeft = QR_STEP - (Math.floor(now / 1000) % QR_STEP);

  return (
    <div class="screen" style={{ background: 'var(--ink)' }}>
      <div style={{ color: 'var(--ivory)', display: 'flex', alignItems: 'flex-end', justifyContent: 'space-between', paddingRight: 24 }}>
        <div><div class="top" /><Back onClick={onBack} label={passes.length > 1 ? `Entry passes · ${index + 1} of ${passes.length}` : 'Entry pass'} light /></div>
        <span style={{ font: "600 15px ui-monospace, Menlo, monospace", color: 'var(--yellow)', letterSpacing: '.02em' }} aria-label="Live time">{clockFmt.format(now).toUpperCase()}</span>
      </div>

      <div ref={track} onScroll={onScroll} class="carousel">
        {passes.map((p) => <PassCard key={p.id} pass={p} event={event} now={now} secsLeft={secsLeft} />)}
      </div>

      {passes.length > 1 && (
        <div style={{ display: 'flex', justifyContent: 'center', gap: 8, marginTop: 14 }}>
          {passes.map((p, i) => (
            <button key={p.id} aria-label={`Pass ${i + 1}`} onClick={() => goTo(i)}
              style={{ width: i === index ? 22 : 8, height: 8, borderRadius: 4, background: i === index ? TONES[p.tone].color : '#555', transition: 'width .2s' }} />
          ))}
        </div>
      )}

      <div style={{ margin: '14px 22px calc(28px + var(--safe-b))', textAlign: 'center', font: "400 13px/1.45 var(--fb)", color: 'var(--soft)' }}>
        {passes.length > 1 ? 'Swipe to show each person’s pass. ' : ''}The code changes every {QR_STEP} seconds, so screenshots won’t work at the gate. Turn up your brightness.
      </div>
    </div>
  );
}

function PassCard({ pass, event, now, secsLeft }: { pass: PassView; event: EventInfo; now: number; secsLeft: number }) {
  const tone = TONES[pass.tone];
  const step = qrStep(now);
  // Recompute only when the step changes (every 15 s).
  const payload = useRef({ step: -1, value: '' });
  if (pass.key && payload.current.step !== step) payload.current = { step, value: qrPayload(pass.key, pass.id, step) };
  const used = !!pass.enteredAt;
  const sub = pass.kind === 'guest' ? `Guest of ${pass.issuerName ?? 'a member'}` : pass.kind === 'exchange' ? pass.college ?? 'Pass exchange' : `Bring your ${tone.id}`;

  return (
    <div class="slide">
      <div style={{ background: 'var(--ivory)', borderRadius: 24, overflow: 'hidden' }}>
        <Rainbow h={16} />
        <Zigzag color={tone.color} h={16} />
        <div style={{ padding: '16px 20px 0', display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
          <div style={{ font: '800 34px/.9 var(--fd)', fontStretch: '75%' }}>{event.title.toUpperCase().split(' ').map((w, i) => <>{i > 0 && <br />}{w}</>)}</div>
          <div style={{ textAlign: 'right', font: "500 13px/1.35 var(--fb)", color: 'var(--body)' }}>{event.dateLabel}<br />{event.venueShort} · {event.timeLabel}</div>
        </div>

        <div style={{ margin: '14px 20px 0', padding: '8px 12px', borderRadius: 12, background: tone.color, color: '#fff', display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 10 }}>
          <span style={{ font: '800 20px/1 var(--fd)', letterSpacing: '.02em' }}>{pass.typeLabel.toUpperCase()}</span>
          <span style={{ font: "600 12px var(--fb)", opacity: .95, textAlign: 'right', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{sub}</span>
        </div>

        <div class={used ? '' : 'qr-live'} style={{ margin: '18px auto 0', width: 211, height: 211, borderRadius: 18, background: '#fff', display: 'grid', placeItems: 'center', border: '3px solid var(--ink)', boxShadow: `6px 6px 0 ${tone.color}`, position: 'relative' }}>
          {used ? (
            <div style={{ textAlign: 'center', padding: 12 }}>
              <div style={{ transform: 'rotate(-6deg)', border: '3px solid var(--bad)', color: 'var(--bad)', borderRadius: 12, padding: '10px 12px', font: '800 26px/1 var(--fd)' }}>
                SCANNED
                <div style={{ font: '700 15px/1.3 var(--fb)', marginTop: 6 }}>{istTime(pass.enteredAt!)} · Gate {pass.enteredGate}</div>
              </div>
              <div class="hint" style={{ marginTop: 14 }}>This pass has been used</div>
            </div>
          ) : pass.key ? (
            <QrCode value={payload.current.value} size={181} />
          ) : <span class="hint">QR unavailable</span>}
        </div>
        {!used && (
          <div style={{ width: 211, margin: '10px auto 0', display: 'flex', alignItems: 'center', gap: 8 }}>
            <div style={{ flex: 1, height: 4, borderRadius: 2, background: 'var(--slot)', overflow: 'hidden' }}>
              <div style={{ width: `${(secsLeft / QR_STEP) * 100}%`, height: '100%', background: tone.color, transition: 'width 1s linear' }} />
            </div>
            <span style={{ font: "600 11px var(--fb)", color: 'var(--muted)', width: 64, textAlign: 'right' }}>new in {secsLeft}s</span>
          </div>
        )}

        <div style={{ padding: '14px 20px 18px', display: 'grid', gridTemplateColumns: '1.4fr 1fr', gap: 6, font: "600 11px var(--fb)", letterSpacing: '.1em', color: 'var(--muted)' }}>
          <div style={{ display: 'flex', flexDirection: 'column', minWidth: 0 }}>NAME<b style={{ font: '700 17px var(--fd)', letterSpacing: 0, color: 'var(--ink)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{pass.holderName}</b></div>
          <div style={{ display: 'flex', flexDirection: 'column' }}>PASS<b style={{ font: '700 17px var(--fd)', letterSpacing: 0, color: 'var(--ink)' }}>{pass.code}</b></div>
        </div>
        <Mirrors color={tone.color} h={14} />
      </div>
    </div>
  );
}
