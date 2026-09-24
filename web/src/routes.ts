import type { MeResponse } from '../../shared/types.ts';

/** Where a signed-in person lands: volunteers on the gate scanner, admins on the dashboard, everyone else on their passes. */
export function homeFor(me: MeResponse): string {
  if (me.user.role === 'volunteer') return '/scan';
  if (me.user.role === 'admin') return '/admin';
  return '/home';
}
