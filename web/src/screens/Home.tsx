import { useState } from 'preact/hooks';
import { GROUP_LABEL, GROUP_PLURAL, type MeResponse, type PassView } from '../../../shared/types.ts';
import { Btn, Logo, MiniQr, Mirrors, Rainbow, Sheet, Top } from '../components/ui.tsx';
import { AVATAR_COLORS, api, initials, istTime, navigate, setMe, share, signOut, toast, toastError } from '../lib.ts';

/** Set by the Send screen so Home opens the share sheet for the pass that was just sent. */
export let justSent: { pass: PassView; claimUrl: string } | null = null;
export const setJustSent = (v: typeof justSent) => { justSent = v; };

function statusOf(p: PassView): { label: string; color: string } {
  if (p.enteredAt) return { label: 'IN', color: 'var(--ok-text)' };
  if (p.status === 'CLAIMED') return { label: 'CLAIMED', color: 'var(--ok-text)' };
  return { label: 'SENT', color: 'var(--pink)' };
}

export function AccountSheet({ me, onClose }: { me: MeResponse; onClose: () => void }) {
  return (
    <Sheet onClose={onClose}>
      <div style={{ display: 'flex', flexDirection: 'column' }}>
        <span style={{ font: '800 26px/1 var(--fd)', fontStretch: '75%' }}>{me.user.name.toUpperCase()}</span>
        <span class="hint" style={{ fontSize: 13 }}>{me.user.email} · {GROUP_LABEL[me.user.group]}</span>
      </div>
      {me.pass && location.pathname !== '/home' && <Btn variant="ghost" onClick={() => { onClose(); navigate('/home'); }}>My pass</Btn>}
      {(me.user.role === 'volunteer' || me.user.role === 'admin') && location.pathname !== '/scan' && <Btn variant="ghost" onClick={() => { onClose(); navigate('/scan'); }}>Gate scanner</Btn>}
      {me.user.role === 'admin' && location.pathname !== '/admin' && <Btn variant="ghost" onClick={() => { onClose(); navigate('/admin'); }}>Cultcomm dashboard</Btn>}
      <Btn icon="↩" onClick={() => { onClose(); signOut(); navigate('/', true); }}>Sign out</Btn>
    </Sheet>
  );
}

export function Home({ me }: { me: MeResponse }) {
  const [sheet, setSheet] = useState<{ pass: PassView; claimUrl?: string } | null>(() => {
    const j = justSent;
    justSent = null;
    return j;
  });
  const [account, setAccount] = useState(false);
  const [busy, setBusy] = useState(false);
  const { user, pass, quota, remaining, sent } = me;
  const canSend = remaining > 0;
  const entered = pass?.enteredAt ? `● ENTERED · GATE ${pass.enteredGate} · ${istTime(pass.enteredAt)}` : '● YOUR PASS · READY';

  async function shareLink(p: PassView, url?: string) {
    try {
      const link = url ?? (await api<{ claimUrl: string }>(`/passes/${p.id}/link`)).claimUrl;
      await share(link, `${user.name} sent you a pass to ${me.event.title} at IIM Ahmedabad (${me.event.dateLabel}). Claim it in your name:`);
    } catch (e) { toastError(e); }
  }

  async function takeBack(p: PassView) {
    if (!confirm(`Take back the pass sent to ${p.holderName}?`)) return;
    setBusy(true);
    try {
      setMe(await api<MeResponse>(`/passes/${p.id}/revoke`, { body: {} }));
      setSheet(null);
      toast('Pass taken back. You can send it to someone else.');
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
              <span style={{ font: "600 11px var(--fb)", letterSpacing: '.12em', color: 'var(--green)' }}>{entered}</span>
              <span style={{ font: '800 26px/1 var(--fd)', fontStretch: '75%' }}>{me.event.title.toUpperCase()}</span>
              <span style={{ font: "400 13px var(--fb)", color: 'var(--soft)' }}>{me.event.dateLabel} · {me.event.venueShort} · Tap for QR</span>
            </div>
            <MiniQr seed={pass.code} />
          </div>
          <Mirrors color="var(--pink)" h={12} bg="transparent" />
        </button>
      )}

      <div class="box" style={{ margin: '18px 20px 0', padding: '16px 18px', display: 'flex', alignItems: 'center', gap: 16 }}>
        <div style={{ font: '800 64px/.8 var(--fd)', color: 'var(--pink)', fontStretch: '75%' }}>{remaining}</div>
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 6 }}>
          <span style={{ font: "600 15px/1.25 var(--fb)" }}>
            {remaining === 1 ? 'pass' : 'passes'} left to share<br />
            <span style={{ color: 'var(--muted)', fontWeight: 400 }}>{quota} for {GROUP_PLURAL[user.group]}</span>
          </span>
          {quota > 0 && (
            <div style={{ display: 'flex', gap: 4 }}>
              {Array.from({ length: quota }, (_, i) => <span key={i} style={{ flex: 1, height: 8, borderRadius: 4, background: i < quota - remaining ? 'var(--pink)' : 'var(--slot)' }} />)}
            </div>
          )}
        </div>
      </div>

      <div style={{ margin: '12px 20px 0' }}>
        <Btn variant="yellow" icon="+" disabled={!canSend} onClick={() => navigate('/send')}>
          {canSend ? 'Send a pass to a friend' : 'All passes shared'}
        </Btn>
      </div>

      <div class="label" style={{ padding: '22px 22px 8px' }}>SENT</div>
      <div style={{ margin: '0 20px', borderTop: '2px solid var(--ink)' }}>
        {sent.length === 0 && <div class="row" style={{ color: 'var(--muted)', font: "400 14px var(--fb)" }}>No passes sent yet. Your friends will show up here.</div>}
        {sent.map((p, i) => {
          const st = statusOf(p);
          return (
            <button key={p.id} class="row" onClick={() => setSheet({ pass: p })}>
              <div class="av" style={{ background: AVATAR_COLORS[i % 4] }}>{initials(p.holderName)}</div>
              <div class="row-main"><span class="row-title">{p.holderName}</span><span class="row-sub">{p.holderContact}</span></div>
              <span class="status" style={{ color: st.color }}>{st.label}</span>
            </button>
          );
        })}
      </div>
      <div style={{ height: 'calc(40px + var(--safe-b))', flex: 'none' }} />

      {sheet && (
        <Sheet onClose={() => setSheet(null)} stripe={sheet.pass.status === 'SENT' ? 'var(--pink)' : 'var(--green)'}>
          <div style={{ display: 'flex', flexDirection: 'column' }}>
            <span class="label">{sheet.claimUrl ? 'PASS SENT' : sheet.pass.code}</span>
            <span style={{ font: '800 30px/1 var(--fd)', fontStretch: '75%' }}>{sheet.pass.holderName.toUpperCase()}</span>
            <span class="note" style={{ marginTop: 6 }}>
              {sheet.pass.status === 'SENT'
                ? `Share the link so ${sheet.pass.holderName.split(' ')[0]} can claim it in their name.${sheet.pass.holderContact.includes('@') ? ' We also emailed it.' : ''} You can take it back until they claim it.`
                : sheet.pass.enteredAt ? `Entered at Gate ${sheet.pass.enteredGate} · ${istTime(sheet.pass.enteredAt)}.` : 'Claimed. They can open their pass from the link or by signing in.'}
            </span>
          </div>
          {sheet.pass.status === 'SENT' && <Btn onClick={() => shareLink(sheet.pass, sheet.claimUrl)} icon="↗">Share claim link</Btn>}
          {sheet.pass.status === 'SENT' && <Btn variant="ghost" disabled={busy} onClick={() => takeBack(sheet.pass)} icon="↩">Take back pass</Btn>}
          {sheet.pass.status !== 'SENT' && <Btn variant="ghost" onClick={() => setSheet(null)} icon="✓">Done</Btn>}
        </Sheet>
      )}
      {account && <AccountSheet me={me} onClose={() => setAccount(false)} />}
    </div>
  );
}
