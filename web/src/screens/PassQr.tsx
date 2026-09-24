import { useEffect } from 'preact/hooks';
import type { EventInfo, PassView } from '../../../shared/types.ts';
import { Back, Mirrors, QrCode, Rainbow, Zigzag } from '../components/ui.tsx';
import { istTime } from '../lib.ts';

/** Full-screen entry pass with the QR the volunteers scan. */
export function PassQr({ pass, event, onBack }: { pass: PassView; event: EventInfo; onBack: () => void }) {
  // Keep the screen awake while the pass is shown at the gate (where supported).
  useEffect(() => {
    let lock: { release: () => Promise<void> } | undefined;
    (navigator as unknown as { wakeLock?: { request: (t: string) => Promise<typeof lock> } }).wakeLock?.request('screen').then((l) => { lock = l; }).catch(() => {});
    return () => { lock?.release().catch(() => {}); };
  }, []);

  return (
    <div class="screen" style={{ background: 'var(--ink)' }}>
      <div style={{ color: 'var(--ivory)' }}>
        <div class="top" />
        <Back onClick={onBack} label="Entry pass" light />
      </div>
      <div style={{ margin: '18px 22px 0', background: 'var(--ivory)', borderRadius: 24, overflow: 'hidden' }}>
        <Rainbow h={16} />
        <Zigzag color="var(--pink)" h={16} />
        <div style={{ padding: '18px 20px 0', display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
          <div style={{ font: '800 34px/.9 var(--fd)', fontStretch: '75%' }}>{event.title.toUpperCase().split(' ').map((w, i) => <>{i > 0 && <br />}{w}</>)}</div>
          <div style={{ textAlign: 'right', font: "500 13px/1.35 var(--fb)", color: 'var(--body)' }}>{event.dateLabel}<br />{event.venueShort} · {event.timeLabel}</div>
        </div>
        <div style={{ margin: '20px auto 0', width: 211, height: 211, borderRadius: 18, background: '#fff', display: 'grid', placeItems: 'center', border: '3px solid var(--ink)', boxShadow: '6px 6px 0 var(--yellow)', position: 'relative' }}>
          {pass.qr ? <QrCode value={pass.qr} size={175} /> : <span class="hint">QR unavailable</span>}
          {pass.enteredAt && (
            <div style={{ position: 'absolute', inset: -3, borderRadius: 18, background: 'rgba(246,240,228,.9)', display: 'grid', placeItems: 'center', textAlign: 'center' }}>
              <div style={{ transform: 'rotate(-8deg)', border: '3px solid var(--ok)', color: 'var(--ok)', borderRadius: 12, padding: '8px 14px', font: '800 26px/1 var(--fd)' }}>
                ENTERED<div style={{ font: '600 13px var(--fb)', marginTop: 4 }}>Gate {pass.enteredGate} · {istTime(pass.enteredAt)}</div>
              </div>
            </div>
          )}
        </div>
        <div style={{ padding: '22px 20px 20px', display: 'grid', gridTemplateColumns: '1.3fr 1fr', gap: 6, font: "600 11px var(--fb)", letterSpacing: '.1em', color: 'var(--muted)' }}>
          <div style={{ display: 'flex', flexDirection: 'column', minWidth: 0 }}>NAME<b style={{ font: '700 17px var(--fd)', letterSpacing: 0, color: 'var(--ink)' }}>{pass.holderName}</b></div>
          <div style={{ display: 'flex', flexDirection: 'column' }}>TYPE<b style={{ font: '700 17px var(--fd)', letterSpacing: 0, color: 'var(--ink)' }}>{pass.typeLabel}</b></div>
        </div>
        <Mirrors color="var(--yellow)" h={14} />
      </div>
      <div style={{ margin: '16px 22px calc(28px + var(--safe-b))', textAlign: 'center', font: "400 13px/1.4 var(--fb)", color: 'var(--soft)' }}>
        {pass.code} · Turn up your brightness at the gate. Screenshots work too.
      </div>
    </div>
  );
}
