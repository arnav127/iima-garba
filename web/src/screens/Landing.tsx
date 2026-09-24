import type { EventInfo } from '../../../shared/types.ts';
import { FactGrid, Logo, Mirrors, Rainbow, Zigzag, Btn } from '../components/ui.tsx';
import { navigate, storage } from '../lib.ts';

export function Landing({ event }: { event: EventInfo }) {
  const guestPass = storage.get<string>('garba:link');
  return (
    <div class="screen">
      <div style={{ height: 'var(--safe-t)', background: 'var(--ivory)' }} />
      <Rainbow h={14} />
      <Zigzag color="var(--pink)" h={16} />
      <Mirrors color="var(--yellow)" h={14} />
      <div style={{ padding: '18px 22px 0', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Logo size={52} />
        <span style={{ font: "600 12px var(--fb)", letterSpacing: '.16em', textAlign: 'right', lineHeight: 1.3 }}>CULTCOMM<br />IIM AHMEDABAD</span>
      </div>
      <div style={{ padding: '26px 22px 0' }}>
        <div class="guj">ગરબા રાત</div>
        <h1 class="display" style={{ margin: '10px 0 0' }}>{event.title.toUpperCase().split(' ').map((w, i) => <>{i > 0 && <br />}{w}</>)}</h1>
        <div class="display" style={{ color: 'transparent', WebkitTextStroke: '2px var(--ink)' }}>IIMA</div>
      </div>
      <FactGrid items={[['DATE', event.dateLabel], ['VENUE', event.venueShort], ['FROM', event.timeLabel]]} />
      <div style={{ margin: '14px 22px 0', display: 'flex', gap: 10, alignItems: 'center', font: "400 14px/1.4 var(--fb)", color: 'var(--body)' }}>
        <span class="mirror-dot" />{event.dressCode}
      </div>
      <div class="bottom">
        <Btn onClick={() => navigate('/login')}>Sign in to get your pass</Btn>
        {guestPass && (
          <Btn variant="ghost" onClick={() => navigate(`/p/${guestPass}`)}>Open my saved pass</Btn>
        )}
        <div style={{ textAlign: 'center', font: "400 13px var(--fb)", color: 'var(--muted)' }}>IIMA: sign in with your @iima.ac.in account · Guests: use the Gmail your host added</div>
      </div>
    </div>
  );
}
