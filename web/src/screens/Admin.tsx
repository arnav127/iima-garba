import { useEffect, useRef, useState } from 'preact/hooks';
import {
  GROUP_LABEL, MEMBER_GROUPS, TONES, toneOfKind,
  type AdminPerson, type AdminStats, type Group, type ImportResult, type MeResponse, type PassView, type Role, type Settings,
} from '../../../shared/types.ts';
import { Btn, ConfirmBtn, Logo, Mirrors, Rainbow, Sheet, Spinner, Top } from '../components/ui.tsx';
import { api, initials, istTime, navigate, onPassesChange, pb, pbCall, storage, toast, toastError } from '../lib.ts';
import { AccountSheet } from './Home.tsx';

type Tab = 'live' | 'passes' | 'people' | 'exchange' | 'settings';
const TABS: [Tab, string][] = [['live', 'Live'], ['passes', 'Passes'], ['people', 'People'], ['exchange', 'Exchange'], ['settings', 'Settings']];

export function Admin({ me }: { me: MeResponse }) {
  const [tab, setTab] = useState<Tab>(() => storage.get<Tab>('garba:admintab') ?? 'live');
  const [account, setAccount] = useState(false);
  const pick = (t: Tab) => { setTab(t); storage.set('garba:admintab', t); };
  return (
    <div class="screen dash">
      <Top />
      <div style={{ padding: '12px 22px 0', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div style={{ display: 'flex', flexDirection: 'column' }}>
          <span style={{ font: "600 13px var(--fb)", color: 'var(--muted)' }}>{me.user.name} · Cultcomm admin</span>
          <span style={{ font: '800 32px/1 var(--fd)', fontStretch: '75%' }}>GARBA CONTROL</span>
        </div>
        <button onClick={() => setAccount(true)} aria-label="Account"><Logo size={40} /></button>
      </div>
      <div class="dash-tabs" style={{ margin: '16px 20px 0' }}>
        <div class="pills" role="tablist" style={{ gap: 2 }}>
          {TABS.map(([id, label]) => <button key={id} role="tab" aria-selected={tab === id} class={`pill ${tab === id ? 'on' : ''}`} style={{ flex: '1 0 auto', padding: '7px 8px' }} onClick={() => pick(id)}>{label}</button>)}
        </div>
      </div>
      {tab === 'live' && <Live me={me} />}
      {tab === 'passes' && <Passes />}
      {tab === 'people' && <People meId={me.user.id} />}
      {tab === 'exchange' && <Exchange />}
      {tab === 'settings' && <SettingsTab />}
      <div style={{ height: 'calc(40px + var(--safe-b))', flex: 'none' }} />
      {account && <AccountSheet me={me} onClose={() => setAccount(false)} />}
    </div>
  );
}

const ToneDot = ({ color }: { color: string }) => <span class="tone-dot" style={{ background: color }} />;
const kindLabel = { own: 'IIMA members', guest: 'Friends & family', exchange: 'Exchange' };

async function download(path: string, name: string) {
  try {
    const res = await fetch(pb.buildURL(path), { headers: { Authorization: pb.authStore.token } });
    if (!res.ok) throw new Error('Download failed');
    const url = URL.createObjectURL(await res.blob());
    const a = document.createElement('a');
    a.href = url;
    a.download = name;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  } catch (e) { toastError(e); }
}

function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = useState(value);
  useEffect(() => { const t = setTimeout(() => setV(value), ms); return () => clearTimeout(t); }, [value, ms]);
  return v;
}

// ---------- Live ----------

function Live({ me }: { me: MeResponse }) {
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
  const tiles: [string, number, string][] = [
    ['IIMA members', s.members, TONES.student.color], ['Friends & family', s.guests, TONES.guest.color],
    ['Exchange guests', s.exchange, TONES.exchange.color], ['Not in yet', s.issued - s.entered, 'var(--ink)'],
  ];
  const maxGate = Math.max(1, ...s.byGate.map((g) => g.count));

  return (
    <div class="dash-cols">
      <div class="dash-col">
      <div class="card-ink" style={{ margin: '16px 20px 0' }}>
        <Rainbow h={10} />
        <div style={{ padding: '16px 18px', display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span style={{ font: "600 11px var(--fb)", letterSpacing: '.12em', color: 'var(--green)' }}>● LIVE · ENTERED</span>
          <span style={{ font: '800 72px/.8 var(--fd)', fontStretch: '75%' }}>{s.entered}</span>
          <span style={{ font: "400 13px var(--fb)", color: 'var(--soft)' }}>of {s.issued} passes · {pct}%</span>
        </div>
        <div style={{ margin: '0 18px 16px', height: 10, borderRadius: 5, background: '#2b2b2b', overflow: 'hidden' }}>
          <div style={{ width: `${pct}%`, height: '100%', background: 'var(--green)', transition: 'width .4s' }} />
        </div>
        <Mirrors color="var(--pink)" h={12} bg="transparent" />
      </div>

      <div style={{ margin: '14px 20px 0', display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
        {tiles.map(([k, v, c]) => (
          <div key={k} class="box" style={{ padding: '12px 14px', borderRadius: 16 }}>
            <div style={{ font: '800 34px/.9 var(--fd)', fontStretch: '75%', color: c }}>{v}</div>
            <div style={{ font: "600 13px var(--fb)" }}>{k}</div>
          </div>
        ))}
      </div>

      <div style={{ margin: '22px 20px 0', display: 'flex', flexDirection: 'column', gap: 12 }}>
        <Btn onClick={() => navigate('/scan')}>Open gate scanner</Btn>
        {me.pass && <Btn variant="ghost" icon="▣" onClick={() => navigate('/pass')}>Show my pass QR</Btn>}
        <Btn variant="ghost" icon="↓" onClick={() => download('/api/garba/admin/export.csv', `garba-passes-${new Date().toISOString().slice(0, 10)}.csv`)}>Download all passes (CSV)</Btn>
      </div>
      </div>

      <div class="dash-col">
      <div class="label dash-first" style={{ padding: '22px 22px 8px' }}>BY GATE</div>
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

      <div class="label" style={{ padding: '22px 22px 8px' }}>BY PASS TYPE · IN / ISSUED</div>
      <div style={{ margin: '0 20px', borderTop: '2px solid var(--ink)' }}>
        {s.byKind.map((k) => (
          <div key={k.kind} class="row"><ToneDot color={TONES[toneOfKind(k.kind)].color} /><div class="row-main"><span class="row-title">{kindLabel[k.kind]}</span></div><span class="status">{k.entered} / {k.issued}</span></div>
        ))}
      </div>

      <div class="label" style={{ padding: '22px 22px 8px' }}>LATEST ENTRIES</div>
      <div style={{ margin: '0 20px', borderTop: '2px solid var(--ink)' }}>
        {s.recent.length === 0 && <div class="row hint">No entries yet.</div>}
        {s.recent.map((r) => (
          <div key={r.code} class="row">
            <div class="av" style={{ background: TONES[toneOfKind(r.kind)].color }}>{initials(r.name)}</div>
            <div class="row-main"><span class="row-title">{r.name}</span><span class="row-sub">{r.code} · Gate {r.gate}</span></div>
            <span class="status" style={{ color: 'var(--ok-text)' }}>{istTime(r.at)}</span>
          </div>
        ))}
      </div>
      </div>
    </div>
  );
}

// ---------- Passes ----------

const FILTERS: [string, string][] = [['', 'All'], ['entered', 'Entered'], ['waiting', 'Not in yet'], ['own', 'IIMA'], ['guest', 'Guests'], ['exchange', 'Exchange'], ['revoked', 'Cancelled']];

function passStatus(p: PassView): { label: string; color: string } {
  if (p.status === 'REVOKED') return { label: 'CANCELLED', color: 'var(--muted)' };
  if (p.enteredAt) return { label: `IN ${istTime(p.enteredAt)}`, color: 'var(--ok-text)' };
  return { label: 'READY', color: 'var(--ink)' };
}

function PassList({ filter: fixed }: { filter?: string }) {
  const [q, setQ] = useState('');
  const [filter, setFilter] = useState(fixed ?? '');
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
        <input class="input" type="search" placeholder="Search name, code, email, college, added by" value={q} onInput={(e) => setQ(e.currentTarget.value)} />
      </div>
      {!fixed && (
        <div style={{ margin: '10px 20px 0', display: 'flex', gap: 6, overflowX: 'auto', scrollbarWidth: 'none' }}>
          {FILTERS.map(([id, label]) => (
            <button key={id} class={`pill ${filter === id ? 'on' : ''}`} style={{ border: '2px solid var(--ink)', background: filter === id ? 'var(--ink)' : '#fff' }} onClick={() => setFilter(id)}>{label}</button>
          ))}
        </div>
      )}
      <div class="row-grid" style={{ margin: '14px 20px 0', borderTop: '2px solid var(--ink)' }}>
        {!list && <Spinner />}
        {list?.length === 0 && <div class="row hint">No passes match.</div>}
        {list?.map((p) => {
          const st = passStatus(p);
          return (
            <button key={p.id} class="row" onClick={() => setSel(p)}>
              <div class="av" style={{ background: TONES[p.tone].color }}>{initials(p.holderName)}</div>
              <div class="row-main">
                <span class="row-title">{p.holderName}</span>
                <span class="row-sub">{p.code} · {p.typeLabel}{p.issuerName ? ` · of ${p.issuerName}` : ''}{p.college ? ` · ${p.college}` : ''}</span>
              </div>
              <span class="status" style={{ color: st.color }}>{st.label}</span>
            </button>
          );
        })}
        {list && list.length >= 200 && <div class="row hint">Showing the first 200. Search to narrow down.</div>}
      </div>
      {sel && (
        <Sheet onClose={() => setSel(null)} stripe={TONES[sel.tone].color}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            <span class="label">{sel.code} · {sel.typeLabel.toUpperCase()} · {passStatus(sel).label}</span>
            <span style={{ font: '800 30px/1 var(--fd)', fontStretch: '75%' }}>{sel.holderName.toUpperCase()}</span>
            <span class="note">{[sel.holderEmail, sel.college, sel.issuerName && `Guest of ${sel.issuerName}`].filter(Boolean).join(' · ') || 'No email · shown from host’s phone or link'}</span>
            {sel.enteredAt && <span class="note" style={{ color: 'var(--ok-text)', fontWeight: 600 }}>Scanned at {istTime(sel.enteredAt)} · Gate {sel.enteredGate}</span>}
          </div>
          {sel.enteredAt && <ConfirmBtn variant="ghost" disabled={busy} icon="↩" confirmLabel="Tap again to undo · can be scanned again" onConfirm={() => act('undo-entry', 'Entry undone. The pass can be scanned again.')}>Undo entry (scanned by mistake)</ConfirmBtn>}
          {!sel.enteredAt && sel.status !== 'REVOKED' && <ConfirmBtn variant="ghost" disabled={busy} icon="✕" confirmLabel="Tap again to cancel this pass" onConfirm={() => act('revoke', 'Pass cancelled')}>Cancel pass</ConfirmBtn>}
          {sel.status === 'REVOKED' && <Btn disabled={busy} icon="↺" onClick={() => act('restore', 'Pass restored')}>Restore pass</Btn>}
        </Sheet>
      )}
    </>
  );
}

const Passes = () => <PassList />;

// ---------- People ----------

function People({ meId }: { meId: string }) {
  const [q, setQ] = useState('');
  const [list, setList] = useState<AdminPerson[] | null>(null);
  const [sel, setSel] = useState<AdminPerson | null>(null);
  const [limit, setLimit] = useState(0);
  const [email, setEmail] = useState('');
  const [role, setRole] = useState<Role>('volunteer');
  const [busy, setBusy] = useState(false);
  const dq = useDebounced(q, 200);

  const load = () => { api<AdminPerson[]>(`/admin/people?q=${encodeURIComponent(dq)}`).then(setList).catch(toastError); };
  useEffect(load, [dq]);

  const open = (p: AdminPerson) => { setSel(p); setLimit(p.limit); };

  async function update(body: { role?: Role; group?: Group; limit?: number }, msg: string) {
    if (!sel) return;
    setBusy(true);
    try {
      const p = await api<AdminPerson>(`/admin/people/${sel.id}`, { body });
      setSel(p); setLimit(p.limit);
      toast(msg);
      load();
    } catch (e) { toastError(e); } finally { setBusy(false); }
  }

  async function grant(e: Event) {
    e.preventDefault();
    setBusy(true);
    try {
      const p = await api<AdminPerson>('/admin/access', { body: { email, role } });
      toast(`${p.name} is now ${role === 'admin' ? 'an admin' : 'a volunteer'}`);
      setEmail('');
      load();
    } catch (err) { toastError(err); } finally { setBusy(false); }
  }

  return (
    <div class="dash-cols">
      <div class="dash-col">
      <form class="box" style={{ margin: '16px 20px 0', padding: 16, display: 'flex', flexDirection: 'column', gap: 10 }} onSubmit={grant}>
        <span style={{ font: '800 22px/1 var(--fd)', fontStretch: '75%' }}>GATE & ADMIN ACCESS</span>
        <span class="note" style={{ fontSize: 13 }}>Add volunteers before the night, even if they haven't signed in yet. Any Google account works, and each volunteer and admin gets a pass of their own.</span>
        <input class="input" type="email" required placeholder="p24name@iima.ac.in or name@gmail.com" value={email} onInput={(e) => setEmail(e.currentTarget.value)} autoCapitalize="none" spellcheck={false} />
        <div class="pills">
          {(['volunteer', 'admin'] as Role[]).map((r) => <button type="button" key={r} class={`pill ${role === r ? 'on' : ''}`} style={{ flex: 1 }} onClick={() => setRole(r)}>{r === 'volunteer' ? 'Gate volunteer' : 'Cultcomm admin'}</button>)}
        </div>
        <Btn type="submit" disabled={busy || !email.includes('@')}>Give access</Btn>
      </form>
      </div>

      <div class="dash-col">
      <div style={{ margin: '16px 20px 0' }}>
        <input class="input" type="search" placeholder="Search people by name or email" value={q} onInput={(e) => setQ(e.currentTarget.value)} />
      </div>
      <div style={{ margin: '14px 20px 0', borderTop: '2px solid var(--ink)' }}>
        {!list && <Spinner />}
        {list?.length === 0 && <div class="row hint">Nobody found. People appear here after they sign in.</div>}
        {list?.map((p) => (
          <button key={p.id} class="row" onClick={() => open(p)}>
            <div class="av" style={{ background: TONES[p.group === 'faculty' ? 'faculty' : p.group === 'guest' ? 'guest' : p.group === 'exchange' ? 'exchange' : 'student'].color }}>{initials(p.name)}</div>
            <div class="row-main"><span class="row-title">{p.name}</span><span class="row-sub">{p.email} · {GROUP_LABEL[p.group]}{MEMBER_GROUPS.includes(p.group) ? ` · ${p.guests}/${p.limit} guests${p.hasLimit ? ' (custom)' : ''}` : ''}</span></div>
            {p.role !== 'member' && <span class="status" style={{ color: p.role === 'admin' ? 'var(--pink)' : 'var(--blue)' }}>{p.role.toUpperCase()}</span>}
          </button>
        ))}
      </div>
      </div>

      {sel && (
        <Sheet onClose={() => setSel(null)}>
          <div style={{ display: 'flex', flexDirection: 'column' }}>
            <span class="label">{GROUP_LABEL[sel.group]} · {sel.role.toUpperCase()}</span>
            <span style={{ font: '800 30px/1 var(--fd)', fontStretch: '75%' }}>{sel.name.toUpperCase()}</span>
            <span class="note">{sel.email}{MEMBER_GROUPS.includes(sel.group) ? ` · ${sel.guests} guests added` : ''}</span>
          </div>

          {MEMBER_GROUPS.includes(sel.group) && (
            <>
              <div class="label">GUEST LIMIT {sel.hasLimit ? '· CUSTOM' : '· GROUP DEFAULT'}</div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <button type="button" class="btn ghost small" style={{ width: 52, justifyContent: 'center' }} onClick={() => setLimit(Math.max(0, limit - 1))}>−</button>
                <span style={{ flex: 1, textAlign: 'center', font: '800 34px/1 var(--fd)' }}>{limit}</span>
                <button type="button" class="btn ghost small" style={{ width: 52, justifyContent: 'center' }} onClick={() => setLimit(Math.min(50, limit + 1))}>+</button>
              </div>
              <div style={{ display: 'flex', gap: 10 }}>
                <Btn disabled={busy || (limit === sel.limit && sel.hasLimit)} onClick={() => update({ limit }, `Limit set to ${limit}`)} style={{ flex: 1 }}>Save limit</Btn>
                {sel.hasLimit && <Btn variant="ghost" disabled={busy} icon="↺" onClick={() => update({ limit: -1 }, 'Back to the group default')} style={{ flex: 1 }}>Default</Btn>}
              </div>
              {limit < sel.guests && <span class="hint">They've already added {sel.guests}. Lowering the limit doesn't remove anyone.</span>}

              <div class="label">COHORT {sel.groupLocked ? '· SET BY ADMIN' : '· FROM EMAIL'}</div>
              <div class="pills">
                {MEMBER_GROUPS.map((g) => <button key={g} class={`pill ${sel.group === g ? 'on' : ''}`} style={{ flex: 1 }} disabled={busy} onClick={() => sel.group !== g && update({ group: g }, `Moved to ${GROUP_LABEL[g]}`)}>{GROUP_LABEL[g]}</button>)}
              </div>
            </>
          )}

          <div class="label">ACCESS</div>
          <div class="pills">
            {(['member', 'volunteer', 'admin'] as Role[]).map((r) => (
              <button key={r} class={`pill ${sel.role === r ? 'on' : ''}`} style={{ flex: 1 }} disabled={busy || sel.id === meId}
                onClick={() => sel.role !== r && update({ role: r }, 'Access updated')}>{r === 'member' ? 'Pass only' : r === 'volunteer' ? 'Volunteer' : 'Admin'}</button>
            ))}
          </div>
        </Sheet>
      )}
    </div>
  );
}

// ---------- Exchange ----------

function Exchange() {
  const [file, setFile] = useState<File | null>(null);
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<ImportResult | null>(null);
  const [v, setV] = useState(0);

  async function upload() {
    if (!file) return;
    setBusy(true); setResult(null);
    try {
      const form = new FormData();
      form.append('file', file);
      const r = await pbCall(() => pb.send<ImportResult>('/api/garba/admin/exchange', { method: 'POST', body: form }));
      setResult(r);
      setFile(null);
      setV((n) => n + 1);
    } catch (e) { toastError(e); } finally { setBusy(false); }
  }

  return (
    <div class="dash-cols">
      <div class="dash-col">
      <div class="box" style={{ margin: '16px 20px 0', padding: 16, display: 'flex', flexDirection: 'column', gap: 10 }}>
        <span style={{ font: '800 22px/1 var(--fd)', fontStretch: '75%' }}>UPLOAD EXCHANGE GUESTS</span>
        <span class="note" style={{ fontSize: 13 }}>
          Excel (.xlsx) or CSV. First row: <b>Name</b>, <b>Email</b>, <b>College</b>, and optionally <b>Phone</b>. Uploading again updates people instead of duplicating them.
          Guests with an email sign in with Google; everyone gets a pass link you can share.
        </span>
        <label class="btn ghost small" style={{ cursor: 'pointer' }}>
          <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{file ? file.name : 'Choose Excel or CSV file'}</span><span aria-hidden="true">↑</span>
          <input type="file" accept=".xlsx,.csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,text/csv" style={{ display: 'none' }} onChange={(e) => setFile(e.currentTarget.files?.[0] ?? null)} />
        </label>
        <Btn disabled={busy || !file} onClick={upload}>{busy ? 'Uploading…' : 'Upload'}</Btn>
        {result && (
          <div class="note" style={{ fontSize: 13 }}>
            <b>{result.created}</b> added · <b>{result.updated}</b> updated{result.errors.length ? ` · ${result.errors.length} skipped:` : ''}
            {result.errors.slice(0, 20).map((er) => <div key={er.line} class="error">Row {er.line}: {er.reason}</div>)}
          </div>
        )}
        <Btn variant="ghost" icon="↓" onClick={() => download('/api/garba/admin/exchange-links.csv', 'exchange-pass-links.csv')}>Download pass links (CSV)</Btn>
      </div>
      </div>
      <div class="dash-col">
      <div class="label dash-first" style={{ padding: '22px 22px 0' }}>EXCHANGE PASSES</div>
      <PassList key={v} filter="exchange" />
      </div>
    </div>
  );
}

// ---------- Settings ----------

function SettingsTab() {
  const [s, setS] = useState<Settings | null>(null);
  const [prefixes, setPrefixes] = useState('');
  const [busy, setBusy] = useState(false);
  useEffect(() => { api<Settings>('/admin/settings').then((r) => { setS(r); setPrefixes(r.pgp1Prefixes.join(', ')); }).catch(toastError); }, []);
  if (!s) return <Spinner />;

  const ev = (k: keyof Settings['event'], v: string | number) => setS({ ...s, event: { ...s.event, [k]: v } });
  async function save() {
    setBusy(true);
    try {
      const r = await api<Settings>('/admin/settings', { method: 'PUT', body: { ...s, pgp1Prefixes: prefixes.split(/[\s,]+/).filter(Boolean) } });
      setS(r); setPrefixes(r.pgp1Prefixes.join(', '));
      toast('Saved. Everyone sees the new details right away.');
    } catch (e) { toastError(e); } finally { setBusy(false); }
  }
  const text: [keyof Settings['event'], string][] = [['title', 'EVENT NAME'], ['dateLabel', 'DATE (as shown)'], ['timeLabel', 'STARTS'], ['venue', 'VENUE'], ['venueShort', 'VENUE (short)'], ['dressCode', 'DRESS CODE LINE']];
  const limits: [keyof Settings['limits'], string][] = [['pgp1', 'PGP1'], ['student', 'OTHER STUDENTS'], ['faculty', 'FACULTY & STAFF']];

  return (
    <div class="dash-cols">
    <div class="dash-col" style={{ margin: '16px 20px 0', gap: 14 }}>
      <span style={{ font: '800 22px/1 var(--fd)', fontStretch: '75%' }}>GUESTS EACH PERSON CAN ADD</span>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 10 }}>
        {limits.map(([k, label]) => (
          <div key={k} class="field">
            <label class="label" for={`l-${k}`} style={{ fontSize: 11 }}>{label}</label>
            <input id={`l-${k}`} class="input" type="number" min={0} max={50} value={s.limits[k]} onInput={(e) => setS({ ...s, limits: { ...s.limits, [k]: Number(e.currentTarget.value) } })} />
          </div>
        ))}
      </div>
      <span class="hint">On top of their own pass. Change one person's limit from People. Lowering a limit doesn't remove guests already added.</span>
      <div class="field">
        <label class="label" for="prefixes">PGP1 EMAIL PREFIXES</label>
        <input id="prefixes" class="input" value={prefixes} onInput={(e) => setPrefixes(e.currentTarget.value)} placeholder="p26, f26" autoCapitalize="none" spellcheck={false} />
        <span class="hint">@iima.ac.in addresses starting with these are PGP1. Other addresses like p25name are students; addresses without a batch number are faculty & staff. People are re-sorted when they next sign in.</span>
      </div>

    </div>
    <div class="dash-col" style={{ margin: '16px 20px 0', gap: 14 }}>
      <span class="dash-first" style={{ font: '800 22px/1 var(--fd)', fontStretch: '75%', marginTop: 10 }}>EVENT</span>
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
      <Btn disabled={busy} onClick={save}>{busy ? 'Saving…' : 'Save settings'}</Btn>
    </div>
    </div>
  );
}
