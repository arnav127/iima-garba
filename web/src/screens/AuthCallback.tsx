import { useEffect } from 'preact/hooks';
import { Spinner } from '../components/ui.tsx';
import { navigate, refreshMe } from '../lib.ts';
import { homeFor } from '../routes.ts';

/** The platform sign-in comes back here: go to this person's home, or to the login page if they aren't on the list. */
export function SignedIn() {
  useEffect(() => {
    refreshMe().then((me) => navigate(me ? homeFor(me) : '/login', true));
  }, []);
  return <div class="screen"><Spinner /></div>;
}
