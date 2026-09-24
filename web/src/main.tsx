import { render, type ComponentType } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import type { EventInfo, MeResponse } from '../../shared/types.ts';
import { Spinner, Toast } from './components/ui.tsx';
import { api, meStore, navigate, refreshMe, renewSession, routeStore, storage } from './lib.ts';
import { homeFor } from './routes.ts';
import { AuthCallback } from './screens/AuthCallback.tsx';
import { Claim } from './screens/Claim.tsx';
import { GuestHome } from './screens/GuestHome.tsx';
import { Home } from './screens/Home.tsx';
import { Landing } from './screens/Landing.tsx';
import { Login } from './screens/Login.tsx';
import { PassQr } from './screens/PassQr.tsx';
import { Send } from './screens/Send.tsx';
// Fonts are self-hosted (no Google Fonts round trip on campus Wi-Fi); browsers fetch only the Latin/Gujarati subsets they need.
import '@fontsource/anek-gujarati/700.css';
import '@fontsource/anek-gujarati/800.css';
import '@fontsource/mukta-vaani/400.css';
import '@fontsource/mukta-vaani/500.css';
import '@fontsource/mukta-vaani/600.css';
import '@fontsource/mukta-vaani/700.css';
import './styles.css';

const FALLBACK_EVENT: EventInfo = {
  title: 'Garba Night', dateLabel: 'Sat 17 Oct', venue: 'Louis Kahn Plaza', venueShort: 'LKP', timeLabel: '8 PM',
  dressCode: 'Dress code: chaniya choli, kediyu, kurta. Dandiyas provided.', gates: 2,
};

/** Loads a screen's code only when it's first needed (the scanner and dashboard are big and only for Cultcomm). */
function lazy<P>(load: () => Promise<ComponentType<P>>) {
  let cached: ComponentType<P> | null = null;
  return (props: P) => {
    const [C, setC] = useState<ComponentType<P> | null>(() => cached);
    useEffect(() => { if (!C) load().then((c) => { cached = c; setC(() => c); }); }, []);
    return C ? <C {...(props as P & object)} /> : <div class="screen"><Spinner /></div>;
  };
}
const Scan = lazy(() => import('./screens/Scan.tsx').then((m) => m.Scan));
const Admin = lazy(() => import('./screens/Admin.tsx').then((m) => m.Admin));

function Redirect({ to }: { to: string }) {
  useEffect(() => navigate(to, true), [to]);
  return null;
}

const isGuest = (me: MeResponse) => me.user.group === 'exchange' || me.user.group === 'guest';

function App() {
  const path = routeStore.use();
  const me = meStore.use();
  const [event, setEvent] = useState<EventInfo>(() => storage.get<EventInfo>('garba:event') ?? FALLBACK_EVENT);
  const [checked, setChecked] = useState(false);

  useEffect(() => {
    renewSession().then(refreshMe).finally(() => setChecked(true));
    api<EventInfo>('/event').then((e) => { setEvent(e); storage.set('garba:event', e); }).catch(() => {});
    const onVisible = () => { if (document.visibilityState === 'visible') refreshMe(); };
    document.addEventListener('visibilitychange', onVisible);
    return () => document.removeEventListener('visibilitychange', onVisible);
  }, []);

  if (path === '/auth/callback') return <AuthCallback />;
  const claim = path.match(/^\/claim\/([\w-]+)$/);
  if (claim) return <Claim token={claim[1]} />;

  if (!me) {
    if (path === '/login') return <Login />;
    if (path === '/' || !checked) return path === '/' ? <Landing event={event} /> : <div class="screen"><Spinner /></div>;
    return <Redirect to="/login" />;
  }

  switch (path) {
    case '/':
    case '/login':
      return <Redirect to={homeFor(me)} />;
    case '/home':
      if (isGuest(me) && me.pass) return <GuestHome me={me} />;
      return <Home me={me} />;
    case '/send':
      return isGuest(me) ? <Redirect to="/home" /> : <Send me={me} />;
    case '/pass':
      return me.pass ? <PassQr pass={me.pass} event={me.event} onBack={() => navigate('/home')} /> : <Redirect to="/home" />;
    case '/scan':
      return me.user.role === 'member' ? <Redirect to="/home" /> : <Scan me={me} />;
    case '/admin':
      return me.user.role === 'admin' ? <Admin me={me} /> : <Redirect to={homeFor(me)} />;
    default:
      return <Redirect to={homeFor(me)} />;
  }
}

render(<><App /><Toast /></>, document.getElementById('app')!);

if (import.meta.env.PROD && 'serviceWorker' in navigator) {
  addEventListener('load', () => navigator.serviceWorker.register('/sw.js').catch(() => {}));
}
