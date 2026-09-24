import { useEffect } from 'preact/hooks';
import { Spinner } from '../components/ui.tsx';
import { finishGoogleSignIn, navigate } from '../lib.ts';
import { homeFor } from '../routes.ts';

/** Google redirects back here after sign-in. */
export function AuthCallback() {
  useEffect(() => {
    finishGoogleSignIn(new URLSearchParams(location.search))
      .then((me) => navigate(homeFor(me), true))
      .catch((e) => navigate(`/login?error=${encodeURIComponent((e as Error).message)}`, true));
  }, []);
  return <div class="screen"><Spinner /></div>;
}
