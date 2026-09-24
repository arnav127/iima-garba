import { useState } from 'preact/hooks';
import { GROUP_LABEL, TONES, type MeResponse, type PassView } from '../../../shared/types.ts';
import { Btn, ConfirmBtn, Logo, MiniQr, Mirrors, Rainbow, Sheet, Top } from '../components/ui.tsx';
import { api, appPath, initials, istTime, navigate, setMe, share, signOut, toast, toastError } from '../lib.ts';

export function AccountSheet({ me, onClose }: { me: MeResponse; onClose: () => void }) {
  return (
    <Sheet onClose={onClose}>
      <div style={{ display: 'flex', flexDirection: 'column' }}>
        <span style={{ font: '800 26px/1 var(--fd)', fontStretch: '75%' }}>{me.user.name.toUpperCase()}</span>
        <span class="hint" style={{ fontSize: 13 }}>{me.user.email} · {GROUP_LABEL[me.user.group]}</span>
      </div>
      {me.pass && me.user.role !== 'member' && <Btn icon="▣" onClick={() => { onClose(); navigate('/pass'); }}>Show my pass QR</Btn>}
      {me.pass && appPath() !== '/home' && <Btn variant="ghost" onClick={() => { onClose(); navigate('/home'); }}>My passes</Btn>}
      {(me.user.role === 'volunteer' || me.user.role === 'admin') && appPath() !== '/scan' && <Btn variant="ghost" onClick={() => { onClose(); navigate('/scan'); }}>Gate scanner</Btn>}
      {me.user.role === 'admin' && appPath() !== '/admin' && <Btn variant="ghost" onClick={() => { onClose(); navigate('/admin'); }}>Cultcomm dashboard</Btn>}
      <Btn icon="↩" onClick={() => { onClose(); signOut(); navigate('/', true); }}>Sign out</Btn>
    </Sheet>
  );
}

export function Home({ me }: { me: MeResponse }) {
  const [sheet, setSheet] = useState<{ pass: PassView; index: number } | null>(null);
  const [account, setAccount] = useState(false);
  const [busy, setBusy] = useState(false);
  const { user, pass, limit, remaining, guests } = me;
  const tone = pass ? TONES[pass.tone] : TONES.student;
  const status = pass?.enteredAt ? `● SCANNED · ${istTime(pass.enteredAt)} · GATE ${pass.enteredGate}` : '● YOUR PASS · READY';
  const all = [pass, ...guests].filter(Boolean).length;

  async function shareLink(p: PassView) {
    try {
      const { link } = await api<{ link: string }>(`/guests/${p.id}/link`);
      await share(link, `${p.holderName.split(' ')[0]}, here's your pass to ${me.event.title} at IIM Ahmedabad (${me.event.dateLabel}, ${me.event.venueShort}, ${me.event.timeLabel}). Open it at the gate:`);
    } catch (e) { toastError(e); }
  }

  async function remove(p: PassView) {
    setBusy(true);
    try {
      setMe(await api<MeResponse>(`/guests/${p.id}/remove`, { body: {} }));
      setSheet(null);
      toast(`${p.holderName}'s pass removed. You can add someone else.`);
    } catch (e) { toastError(e); } finally { setBusy(false); }
  }

  return (
    <div class="screen">
      <Top />
      <div style={{ padding: '12px 22px 0', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div style={{ display: 'flex', flexDirection: 'column' }}>
          <span style={{ font: '600 13px var(--fb)', color: 'var(--muted)' }}>{user.name} · {GROUP_LABEL[user.group]}</span>
          <span style={{ font: '800 32px/1 var(--fd)', fontStretch: '75%' }}>MY PASSES</span>
        </div>
        <button onClick={() => setAccount(true)} aria-label="Account"><Logo size={40} /></button>
      </div>

      {pass && (
        <button onClick={() => navigate('/pass')} class="card-ink" style={{ margin: '16px 20px 0', textAlign: 'left', display: 'block' }}>
          <Rainbow h={10} />
          <div style={{ padding: '16px 18px', display: 'flex', alignItems: 'center', gap: 14 }}>
            <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 2, minWidth: 0 }}>
              <span style={{ font: "600 11px var(--fb)", letterSpacing: '.12em', color: pass.enteredAt ? 'var(--yellow)' : 'var(--green)' }}>{status}</span>
              <span style={{ font: '800 26px/1 var(--fd)', fontStretch: '75%' }}>{me.event.title.toUpperCase()}</span>
              <span style={{ font: "400 13px var(--fb)", color: 'var(--soft)' }}>{me.event.dateLabel} · {me.event.venueShort} · {all > 1 ? `Tap for all ${all} QRs` : 'Tap for QR'}</span>
            </div>
            <MiniQr seed={pass.code} />
          </div>
          <Mirrors color={tone.color} h={12} bg="transparent" />
        </button>
      )}

      {limit === 0 ? (
        <div class="box" style={{ margin: '18px 20px 0', padding: '16px 18px', display: 'flex', alignItems: 'center', gap: 14 }}>
          <span class="mirror-dot" />
          <span style={{ font: "500 15px/1.35 var(--fb)" }}>Your pass is just for you.<br /><span style={{ color: 'var(--muted)', fontWeight: 400, fontSize: 14 }}>{user.group === 'pgp1' ? 'PGP1 passes can’t be shared this year.' : 'Your pass can’t be shared.'} Ask Cultcomm if you need an exception.</span></span>
        </div>
      ) : (
        <>
          <div class="box" style={{ margin: '18px 20px 0', padding: '16px 18px', display: 'flex', alignItems: 'center', gap: 16 }}>
            <div style={{ font: '800 64px/.8 var(--fd)', color: 'var(--pink)', fontStretch: '75%' }}>{remaining}</div>
            <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 6 }}>
              <span style={{ font: "600 15px/1.25 var(--fb)" }}>
                guest {remaining === 1 ? 'pass' : 'passes'} left<br />
                <span style={{ color: 'var(--muted)', fontWeight: 400 }}>{limit} for your friends & family</span>
              </span>
              <div style={{ display: 'flex', gap: 4 }}>
                {Array.from({ length: limit }, (_, i) => <span key={i} style={{ flex: 1, height: 8, borderRadius: 4, background: i < limit - remaining ? 'var(--orange)' : 'var(--slot)' }} />)}
              </div>
            </div>
          </div>
          <div style={{ margin: '12px 20px 0' }}>
            <Btn variant="yellow" icon="+" disabled={remaining === 0} onClick={() => navigate('/add')}>
              {remaining > 0 ? 'Add a friend or family member' : 'All guest passes used'}
            </Btn>
          </div>
        </>
      )}

      {guests.length > 0 && (
        <>
          <div class="label" style={{ padding: '22px 22px 8px' }}>YOUR GUESTS</div>
          <div style={{ margin: '0 20px', borderTop: '2px solid var(--ink)' }}>
            {guests.map((p, i) => (
              <button key={p.id} class="row" onClick={() => setSheet({ pass: p, index: i + (pass ? 1 : 0) })}>
                <div class="av" style={{ background: TONES.guest.color }}>{initials(p.holderName)}</div>
                <div class="row-main">
                  <span class="row-title">{p.holderName}</span>
                  <span class="row-sub">{p.holderEmail ? `Can sign in as ${p.holderEmail}` : 'On your phone · or share a link'}</span>
                </div>
                <span class="status" style={{ color: p.enteredAt ? 'var(--ok-text)' : 'var(--orange)' }}>{p.enteredAt ? 'IN' : 'READY'}</span>
              </button>
            ))}
          </div>
        </>
      )}
      <div style={{ height: 'calc(40px + var(--safe-b))', flex: 'none' }} />

      {sheet && (
        <Sheet onClose={() => setSheet(null)} stripe={TONES.guest.color}>
          <div style={{ display: 'flex', flexDirection: 'column' }}>
            <span class="label">{sheet.pass.code} · GUEST</span>
            <span style={{ font: '800 30px/1 var(--fd)', fontStretch: '75%' }}>{sheet.pass.holderName.toUpperCase()}</span>
            <span class="note" style={{ marginTop: 6 }}>
              {sheet.pass.enteredAt
                ? `Scanned at ${istTime(sheet.pass.enteredAt)} · Gate ${sheet.pass.enteredGate}.`
                : sheet.pass.holderEmail
                  ? `They can sign in with Google as ${sheet.pass.holderEmail} to show their own QR, or you can show it from your phone.`
                  : 'Show their QR from your phone at the gate, or share their pass link so they can open it themselves.'}
            </span>
          </div>
          {!sheet.pass.enteredAt && <Btn onClick={() => navigate(`/pass?i=${sheet.index}`)}>Show their QR</Btn>}
          {!sheet.pass.enteredAt && <Btn variant="ghost" icon="↗" onClick={() => shareLink(sheet.pass)}>Share pass link (WhatsApp)</Btn>}
          {!sheet.pass.enteredAt && <ConfirmBtn variant="ghost" disabled={busy} icon="✕" confirmLabel="Tap again · their QR and link stop working" onConfirm={() => remove(sheet.pass)}>Remove guest</ConfirmBtn>}
          {sheet.pass.enteredAt && <Btn variant="ghost" icon="✓" onClick={() => setSheet(null)}>Done</Btn>}
        </Sheet>
      )}
      {account && <AccountSheet me={me} onClose={() => setAccount(false)} />}
    </div>
  );
}
