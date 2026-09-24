import { useEffect } from 'preact/hooks';
import { Spinner } from '../components/ui.tsx';
import { finishSignIn, navigate } from '../lib.ts';
import { homeFor } from '../routes.ts';

/** Google (or Microsoft) redirects back here after sign-in. */
export function AuthCallback() {
  useEffect(() => {
    finishSignIn(new URLSearchParams(location.search))
      .then((me) => navigate(homeFor(me), true))
      .catch((e) => navigate(`/login?error=${encodeURIComponent((e as Error).message)}`, true));
  }, []);
  return <div class="screen"><Spinner /></div>;
}
