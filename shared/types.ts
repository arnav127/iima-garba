// API shapes of the /api/garba/* routes. Keep in sync with backend/garba/*.go.

export type Group = 'pgp1' | 'pgp2' | 'pgpx' | 'phd' | 'faculty' | 'staff' | 'exchange' | 'guest';
export type Role = 'member' | 'volunteer' | 'admin';
export type PassKind = 'own' | 'guest' | 'exchange';
export type PassStatus = 'SENT' | 'CLAIMED' | 'REVOKED';

/** Groups that can be imported from the roster CSV and given a quota. */
export const GROUPS: Group[] = ['pgp1', 'pgp2', 'pgpx', 'phd', 'faculty', 'staff', 'exchange'];

export const GROUP_LABEL: Record<Group, string> = {
  pgp1: 'PGP1', pgp2: 'PGP2', pgpx: 'PGPX', phd: 'PhD', faculty: 'Faculty', staff: 'Staff', exchange: 'Exchange', guest: 'Guest',
};

/** Used in "4 for PGP1 students". */
export const GROUP_PLURAL: Record<Group, string> = {
  pgp1: 'PGP1 students', pgp2: 'PGP2 students', pgpx: 'PGPX students', phd: 'PhD scholars',
  faculty: 'faculty', staff: 'staff', exchange: 'exchange guests', guest: 'guests',
};

export interface EventInfo {
  title: string;       // "Garba Night"
  dateLabel: string;   // "Sat 17 Oct"
  venue: string;       // "Louis Kahn Plaza"
  venueShort: string;  // "LKP"
  timeLabel: string;   // "8 PM"
  dressCode: string;
  gates: number;
}

export type Quotas = Partial<Record<Group, number>>;

export interface Settings {
  event: EventInfo;
  quotas: Quotas;
}

export interface PassView {
  id: string;
  code: string;            // GRB-0417 / GRB-X-0932
  kind: PassKind;
  status: PassStatus;
  holderName: string;
  holderContact: string;
  typeLabel: string;       // PGP1 / Exchange / Guest
  college: string | null;
  issuerName: string | null;
  enteredAt: number | null;
  enteredGate: number | null;
  /** QR payload. Only sent to the pass holder. */
  qr?: string;
  /** Admin views only. */
  holderEmail?: string | null;
}

export interface Me {
  id: string;
  email: string;
  name: string;
  group: Group;
  role: Role;
  college: string | null;
}

export interface MeResponse {
  user: Me;
  event: EventInfo;
  pass: PassView | null;
  quota: number;
  remaining: number;
  sent: PassView[];
}

export interface ClaimResponse {
  event: EventInfo;
  pass: PassView;
  claimed: boolean;
}

export type ScanOutcome = 'allowed' | 'used' | 'revoked' | 'unclaimed' | 'invalid';

export interface ScanResult {
  outcome: ScanOutcome;
  title: string;
  sub: string;
  name: string;
  meta: string;
  entered: number;
}

export interface AdminStats {
  entered: number;
  issued: number;
  claimed: number;
  pendingClaims: number;
  members: number;
  exchange: number;
  byGate: { gate: number; count: number }[];
  byKind: { kind: PassKind; issued: number; entered: number }[];
  recent: { name: string; code: string; gate: number; at: number }[];
}


export interface AdminPerson {
  id: string;
  email: string;
  name: string;
  group: Group;
  role: Role;
  college: string | null;
  sent: number;
}

export interface SendResponse {
  pass: PassView;
  claimUrl: string;
  me: MeResponse;
}

export interface ImportResult {
  created: number;
  updated: number;
  errors: { line: number; reason: string }[];
}
