import { useEffect, useRef, useState } from 'preact/hooks';
import {
  GROUPS, GROUP_LABEL,
  type AdminPerson, type AdminStats, type ImportResult, type MeResponse, type PassView, type Role, type Settings,
} from '../../../shared/types.ts';
import { Btn, Logo, Mirrors, Rainbow, Sheet, Spinner, Top } from '../components/ui.tsx';
import { AVATAR_COLORS, api, initials, istTime, navigate, onPassesChange, pb, storage, toast, toastError } from '../lib.ts';
import { AccountSheet } from './Home.tsx';

type Tab = 'live' | 'passes' | 'people' | 'settings';
const TABS: [Tab, string][] = [['live', 'Live'], ['passes', 'Passes'], ['people', 'People'], ['settings', 'Settings']];

export function Admin({ me }: { me: MeResponse }) {
  const [tab, setTab] = useState<Tab>(() => storage.get<Tab>('garba:admintab') ?? 'live');
  const [account, setAccount] = useState(false);
  const pick = (t: Tab) => { setTab(t); storage.set('garba:admintab', t); };
  return (
    <div class="screen">
      <Top />
      <div style={{ padding: '12px 22px 0', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div style={{ display: 'flex', flexDirection: 'column' }}>
          <span style={{ font: "600 13px var(--fb)", color: 'var(--muted)' }}>{me.user.name} · Cultcomm admin</span>
          <span style={{ font: '800 32px/1 var(--fd)', fontStretch: '75%' }}>GARBA CONTROL</span>
        </div>
        <button onClick={() => setAccount(true)} aria-label="Account"><Logo size={40} /></button>
      </div>
      <div style={{ margin: '16px 20px 0' }}>
        <div class="pills" role="tablist">
          {TABS.map(([id, label]) => <button key={id} role="tab" aria-selected={tab === id} class={`pill ${tab === id ? 'on' : ''}`} style={{ flex: 1 }} onClick={() => pick(id)}>{label}</button>)}
        </div>
      </div>
      {tab === 'live' && <Live />}
      {tab === 'passes' && <Passes />}
      {tab === 'people' && <People meId={me.user.id} />}
      {tab === 'settings' && <SettingsTab />}
      <div style={{ height: 'calc(40px + var(--safe-b))', flex: 'none' }} />
      {account && <AccountSheet me={me} onClose={() => setAccount(false)} />}
    </div>
  );
}

// ---------- Live ----------

function Live() {
  const [s, setS] = useState<AdminStats | null>(null);
  useEffect(() => {
    let t: ReturnType<typeof setTimeout> | undefined;
    const load = () => api<AdminStats>('/admin/stats').then(setS).catch(toastError);
    load();
    const unsub = onPassesChange(() => { clearTimeout(t); t = setTimeout(load, 400); });
    const poll = setInterval(load, 30_000);
    return () => { unsub(); clearInterval(poll); clearTimeout(t); };
  }, []);
  if (!s) return <Spinner />;
  const pct = s.issued ? Math.round((s.entered / s.issued) * 100) : 0;
  const tiles: [string, number][] = [['Claimed', s.claimed], ['Awaiting claim', s.pendingClaims], ['IIMA people', s.members], ['Exchange guests', s.exchange]];
  const kindLabel = { own: 'IIMA', guest: 'Friends', exchange: 'Exchange' };
  const maxGate = Math.max(1, ...s.byGate.map((g) => g.count));

  return (
    <>
      <div class="card-ink" style={{ margin: '16px 20px 0' }}>
        <Rainbow h={10} />
        <div style={{ padding: '16px 18px', display: 'flex', alignItems: 'flex-end', gap: 14 }}>
          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 4 }}>
            <span style={{ font: "600 11px var(--fb)", letterSpacing: '.12em', color: 'var(--green)' }}>● LIVE · ENTERED</span>
            <span style={{ font: '800 72px/.8 var(--fd)', fontStretch: '75%' }}>{s.entered}</span>
            <span style={{ font: "400 13px var(--fb)", color: 'var(--soft)' }}>of {s.issued} passes issued · {pct}%</span>
          </div>
        </div>
        <div style={{ margin: '0 18px 16px', height: 10, borderRadius: 5, background: '#2b2b2b', overflow: 'hidden' }}>
          <div style={{ width: `${pct}%`, height: '100%', background: 'var(--green)', transition: 'width .4s' }} />
        </div>
        <Mirrors color="var(--pink)" h={12} bg="transparent" />
      </div>

      <div style={{ margin: '14px 20px 0', display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
        {tiles.map(([k, v]) => (
          <div key={k} class="box" style={{ padding: '12px 14px', borderRadius: 16 }}>
            <div style={{ font: '800 34px/.9 var(--fd)', fontStretch: '75%', color: 'var(--pink)' }}>{v}</div>
            <div style={{ font: "600 13px var(--fb)" }}>{k}</div>
          </div>
        ))}
      </div>

      <div class="label" style={{ padding: '22px 22px 8px' }}>BY GATE</div>
      <div style={{ margin: '0 20px', display: 'flex', flexDirection: 'column', gap: 8 }}>
        {s.byGate.length === 0 && <span class="hint">Nobody has entered yet.</span>}
        {s.byGate.map((g) => (
          <div key={g.gate} style={{ display: 'flex', alignItems: 'center', gap: 10, font: "600 14px var(--fb)" }}>
            <span style={{ width: 56 }}>Gate {g.gate}</span>
            <div style={{ flex: 1, height: 14, border: '2px solid var(--ink)', borderRadius: 7, overflow: 'hidden', background: '#fff' }}>
              <div style={{ width: `${(g.count / maxGate) * 100}%`, height: '100%', background: 'var(--yellow)' }} />
            </div>
            <span style={{ width: 40, textAlign: 'right' }}>{g.count}</span>
          </div>
        ))}
      </div>

      <div class="label" style={{ padding: '22px 22px 8px' }}>BY PASS TYPE</div>
      <div style={{ margin: '0 20px', borderTop: '2px solid var(--ink)' }}>
        {s.byKind.map((k) => (
          <div key={k.kind} class="row"><div class="row-main"><span class="row-title">{kindLabel[k.kind]}</span></div><span class="status">{k.entered} / {k.issued}</span></div>
        ))}
      </div>

      <div class="label" style={{ padding: '22px 22px 8px' }}>LATEST ENTRIES</div>
      <div style={{ margin: '0 20px', borderTop: '2px solid var(--ink)' }}>
        {s.recent.length === 0 && <div class="row hint">No entries yet.</div>}
        {s.recent.map((r, i) => (
          <div key={r.code} class="row">
            <div class="av" style={{ background: AVATAR_COLORS[i % 4] }}>{initials(r.name)}</div>
            <div class="row-main"><span class="row-title">{r.name}</span><span class="row-sub">{r.code} · Gate {r.gate}</span></div>
            <span class="status" style={{ color: 'var(--ok-text)' }}>{istTime(r.at)}</span>
          </div>
        ))}
      </div>

      <div style={{ margin: '22px 20px 0', display: 'flex', flexDirection: 'column', gap: 12 }}>
        <Btn onClick={() => navigate('/scan')}>Open gate scanner</Btn>
        <Btn variant="ghost" icon="↓" onClick={downloadCsv}>Download all passes (CSV)</Btn>
      </div>
    </>
  );
}

async function downloadCsv() {
  try {
    const res = await fetch(pb.buildURL('/api/garba/admin/export.csv'), { headers: { Authorization: pb.authStore.token } });
    if (!res.ok) throw new Error('Download failed');
    const url = URL.createObjectURL(await res.blob());
    const a = document.createElement('a');
    a.href = url;
    a.download = `garba-passes-${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  } catch (e) { toastError(e); }
}

// ---------- Passes ----------

const FILTERS: [string, string][] = [['', 'All'], ['entered', 'Entered'], ['waiting', 'Not in yet'], ['unclaimed', 'Unclaimed'], ['guest', 'Friends'], ['exchange', 'Exchange'], ['revoked', 'Revoked']];

function passStatus(p: PassView): { label: string; color: string } {
  if (p.status === 'REVOKED') return { label: 'REVOKED', color: 'var(--muted)' };
  if (p.enteredAt) return { label: 'IN', color: 'var(--ok-text)' };
  if (p.status === 'SENT') return { label: 'SENT', color: 'var(--pink)' };
  return { label: 'READY', color: 'var(--ink)' };
}

function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = useState(value);
  useEffect(() => { const t = setTimeout(() => setV(value), ms); return () => clearTimeout(t); }, [value, ms]);
  return v;
}

function Passes() {
  const [q, setQ] = useState('');
  const [filter, setFilter] = useState('');
  const [list, setList] = useState<PassView[] | null>(null);
  const [sel, setSel] = useState<PassView | null>(null);
  const [busy, setBusy] = useState(false);
  const dq = useDebounced(q, 200);
  const seq = useRef(0);

  const load = () => {
    const n = ++seq.current;
    api<PassView[]>(`/admin/passes?q=${encodeURIComponent(dq)}&filter=${filter}`).then((r) => { if (n === seq.current) setList(r); }).catch(toastError);
  };
  useEffect(load, [dq, filter]);

  async function act(path: string, msg: string) {
    if (!sel) return;
    setBusy(true);
    try {
      await api(`/admin/passes/${sel.id}/${path}`, { body: {} });
      toast(msg);
      setSel(null);
      load();
    } catch (e) { toastError(e); } finally { setBusy(false); }
  }

  return (
    <>
      <div style={{ margin: '16px 20px 0' }}>
        <input class="input" type="search" placeholder="Search name, code, email, phone, sent by" value={q} onInput={(e) => setQ(e.currentTarget.value)} />
      </div>
      <div style={{ margin: '10px 20px 0', display: 'flex', gap: 6, overflowX: 'auto', scrollbarWidth: 'none' }}>
        {FILTERS.map(([id, label]) => (
          <button key={id} class={`pill ${filter === id ? 'on' : ''}`} style={{ border: '2px solid var(--ink)', background: filter === id ? 'var(--ink)' : '#fff' }} onClick={() => setFilter(id)}>{label}</button>
        ))}
      </div>
      <div style={{ margin: '14px 20px 0', borderTop: '2px solid var(--ink)' }}>
        {!list && <Spinner />}
        {list?.length === 0 && <div class="row hint">No passes match.</div>}
        {list?.map((p, i) => {
          const st = passStatus(p);
          return (
            <button key={p.id} class="row" onClick={() => setSel(p)}>
              <div class="av" style={{ background: AVATAR_COLORS[i % 4] }}>{initials(p.holderName)}</div>
              <div class="row-main">
                <span class="row-title">{p.holderName}</span>
                <span class="row-sub">{p.code} · {p.typeLabel}{p.issuerName ? ` · from ${p.issuerName}` : ''}{p.college ? ` · ${p.college}` : ''}</span>
              </div>
              <span class="status" style={{ color: st.color }}>{st.label}</span>
            </button>
          );
        })}
        {list && list.length >= 200 && <div class="row hint">Showing the first 200. Search to narrow down.</div>}
      </div>
      {sel && (
        <Sheet onClose={() => setSel(null)} stripe={sel.enteredAt ? 'var(--green)' : sel.status === 'REVOKED' ? 'var(--muted)' : 'var(--pink)'}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            <span class="label">{sel.code} · {sel.typeLabel} · {passStatus(sel).label}</span>
            <span style={{ font: '800 30px/1 var(--fd)', fontStretch: '75%' }}>{sel.holderName.toUpperCase()}</span>
            <span class="note">{[sel.holderEmail ?? sel.holderContact, sel.college, sel.issuerName && `Sent by ${sel.issuerName}`].filter(Boolean).join(' · ')}</span>
            {sel.enteredAt && <span class="note" style={{ color: 'var(--ok-text)', fontWeight: 600 }}>Entered at Gate {sel.enteredGate} · {istTime(sel.enteredAt)}</span>}
          </div>
          {sel.enteredAt && <Btn variant="ghost" disabled={busy} icon="↩" onClick={() => confirm('Undo this entry? The pass can be scanned again.') && act('undo-entry', 'Entry undone')}>Undo entry (scanned by mistake)</Btn>}
          {!sel.enteredAt && sel.status !== 'REVOKED' && <Btn variant="danger" disabled={busy} icon="✕" onClick={() => confirm(`Revoke ${sel.holderName}'s pass? It will be refused at the gate.`) && act('revoke', 'Pass revoked')}>Revoke pass</Btn>}
          {sel.status === 'REVOKED' && <Btn disabled={busy} icon="↺" onClick={() => act('restore', 'Pass restored')}>Restore pass</Btn>}
        </Sheet>
      )}
    </>
  );
}

// ---------- People ----------

const SAMPLE = `email,name,group,college,role
p25aarav@iima.ac.in,Aarav Shah,pgp1,,
aarav.shah@gmail.com,Aarav Shah,pgp1,,
p24meera@iima.ac.in,Meera Iyer,pgp2,,volunteer
ishaan.m@spjimr.org,Ishaan Mehta,exchange,SPJIMR Mumbai,`;

function People({ meId }: { meId: string }) {
  const [q, setQ] = useState('');
  const [list, setList] = useState<AdminPerson[] | null>(null);
  const [csv, setCsv] = useState('');
  const [importing, setImporting] = useState(false);
  const [result, setResult] = useState<ImportResult | null>(null);
  const [sel, setSel] = useState<AdminPerson | null>(null);
  const dq = useDebounced(q, 200);

  const load = () => { api<AdminPerson[]>(`/admin/people?q=${encodeURIComponent(dq)}`).then(setList).catch(toastError); };
  useEffect(load, [dq]);

  async function runImport() {
    setImporting(true); setResult(null);
    try {
      const r = await api<ImportResult>('/admin/import', { body: { csv } });
      setResult(r);
      if (!r.errors.length) setCsv('');
      load();
    } catch (e) { toastError(e); } finally { setImporting(false); }
  }

  async function setRole(role: Role) {
    if (!sel) return;
    try {
      await api(`/admin/people/${sel.id}/role`, { body: { role } });
      toast(`${sel.name} is now ${role === 'member' ? 'a member' : `a ${role}`}`);
      setSel(null);
      load();
    } catch (e) { toastError(e); }
  }

  return (
    <>
      <div class="box" style={{ margin: '16px 20px 0', padding: 16, display: 'flex', flexDirection: 'column', gap: 10 }}>
        <span style={{ font: '800 22px/1 var(--fd)', fontStretch: '75%' }}>IMPORT THE LIST</span>
        <span class="note" style={{ fontSize: 13 }}>
          CSV with a header row: <b>email,name,group</b>, plus <b>college</b> for exchange guests and an optional <b>role</b> (volunteer, admin).
          Groups: {GROUPS.join(', ')}. Use the email people sign in with (Gmail or @iima.ac.in). Re-importing updates people; everyone gets their own pass.
        </span>
        <label class="btn ghost small" style={{ cursor: 'pointer' }}>
          <span>Choose CSV file</span><span aria-hidden="true">↑</span>
          <input type="file" accept=".csv,text/csv" style={{ display: 'none' }} onChange={async (e) => { const f = e.currentTarget.files?.[0]; if (f) setCsv(await f.text()); }} />
        </label>
        <textarea class="input" rows={5} placeholder={SAMPLE} value={csv} onInput={(e) => setCsv(e.currentTarget.value)} />
        <Btn disabled={importing || !csv.trim()} onClick={runImport}>{importing ? 'Importing…' : 'Import people'}</Btn>
        {result && (
          <div class="note" style={{ fontSize: 13 }}>
            <b>{result.created}</b> added · <b>{result.updated}</b> updated{result.errors.length ? ` · ${result.errors.length} skipped:` : ''}
            {result.errors.slice(0, 20).map((er) => <div key={er.line} class="error">Line {er.line}: {er.reason}</div>)}
          </div>
        )}
      </div>

      <div style={{ margin: '18px 20px 0' }}>
        <input class="input" type="search" placeholder="Search people" value={q} onInput={(e) => setQ(e.currentTarget.value)} />
      </div>
      <div style={{ margin: '14px 20px 0', borderTop: '2px solid var(--ink)' }}>
        {!list && <Spinner />}
        {list?.length === 0 && <div class="row hint">Nobody here yet. Import the list above.</div>}
        {list?.map((p, i) => (
          <button key={p.id} class="row" onClick={() => setSel(p)} disabled={p.id === meId}>
            <div class="av" style={{ background: AVATAR_COLORS[i % 4] }}>{initials(p.name)}</div>
            <div class="row-main"><span class="row-title">{p.name}</span><span class="row-sub">{p.email} · {GROUP_LABEL[p.group]}{p.college ? ` · ${p.college}` : ''}{p.sent ? ` · ${p.sent} sent` : ''}</span></div>
            {p.role !== 'member' && <span class="status" style={{ color: p.role === 'admin' ? 'var(--pink)' : 'var(--blue)' }}>{p.role.toUpperCase()}</span>}
          </button>
        ))}
      </div>
      {sel && (
        <Sheet onClose={() => setSel(null)}>
          <div style={{ display: 'flex', flexDirection: 'column' }}>
            <span class="label">{GROUP_LABEL[sel.group]} · {sel.role.toUpperCase()}</span>
            <span style={{ font: '800 30px/1 var(--fd)', fontStretch: '75%' }}>{sel.name.toUpperCase()}</span>
            <span class="note">{sel.email}</span>
          </div>
          {(['volunteer', 'admin', 'member'] as Role[]).filter((r) => r !== sel.role).map((r) => (
            <Btn key={r} variant={r === 'member' ? 'ghost' : ''} onClick={() => setRole(r)}>
              {r === 'volunteer' ? 'Make gate volunteer' : r === 'admin' ? 'Make Cultcomm admin' : 'Remove volunteer/admin access'}
            </Btn>
          ))}
        </Sheet>
      )}
    </>
  );
}

// ---------- Settings ----------

function SettingsTab() {
  const [s, setS] = useState<Settings | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => { api<Settings>('/admin/settings').then(setS).catch(toastError); }, []);
  if (!s) return <Spinner />;

  const ev = (k: keyof Settings['event'], v: string | number) => setS({ ...s, event: { ...s.event, [k]: v } });
  async function save() {
    setBusy(true);
    try {
      setS(await api<Settings>('/admin/settings', { method: 'PUT', body: s }));
      toast('Saved. Everyone sees the new details right away.');
    } catch (e) { toastError(e); } finally { setBusy(false); }
  }
  const text: [keyof Settings['event'], string][] = [['title', 'EVENT NAME'], ['dateLabel', 'DATE (as shown)'], ['timeLabel', 'STARTS'], ['venue', 'VENUE'], ['venueShort', 'VENUE (short)'], ['dressCode', 'DRESS CODE LINE']];

  return (
    <div style={{ margin: '16px 20px 0', display: 'flex', flexDirection: 'column', gap: 14 }}>
      <span style={{ font: '800 22px/1 var(--fd)', fontStretch: '75%' }}>EVENT</span>
      {text.map(([k, label]) => (
        <div key={k} class="field">
          <label class="label" for={`ev-${k}`}>{label}</label>
          <input id={`ev-${k}`} class="input" value={String(s.event[k])} onInput={(e) => ev(k, e.currentTarget.value)} />
        </div>
      ))}
      <div class="field">
        <label class="label" for="ev-gates">ENTRY GATES</label>
        <input id="ev-gates" class="input" type="number" min={1} max={20} value={s.event.gates} onInput={(e) => ev('gates', Number(e.currentTarget.value))} />
      </div>

      <span style={{ font: '800 22px/1 var(--fd)', fontStretch: '75%', marginTop: 10 }}>PASSES EACH PERSON CAN SHARE</span>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
        {GROUPS.filter((g) => g !== 'exchange').map((g) => (
          <div key={g} class="field">
            <label class="label" for={`q-${g}`}>{GROUP_LABEL[g].toUpperCase()}</label>
            <input id={`q-${g}`} class="input" type="number" min={0} max={50} value={s.quotas[g] ?? 0} onInput={(e) => setS({ ...s, quotas: { ...s.quotas, [g]: Number(e.currentTarget.value) } })} />
          </div>
        ))}
      </div>
      <span class="hint">Lowering a quota doesn't take back passes already sent.</span>
      <Btn disabled={busy} onClick={save}>{busy ? 'Saving…' : 'Save settings'}</Btn>
    </div>
  );
}
