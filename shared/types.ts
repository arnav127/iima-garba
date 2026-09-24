// API shapes of the /api/garba/* routes. Keep in sync with backend/garba/*.go.

/** pgp1 | student | faculty come from the @iima.ac.in address; guest | exchange from the pass they hold. */
export type Group = 'pgp1' | 'student' | 'faculty' | 'exchange' | 'guest';
export type Role = 'member' | 'volunteer' | 'admin';
export type PassKind = 'own' | 'guest' | 'exchange';
export type PassStatus = 'ACTIVE' | 'REVOKED';
/** Colour family shown on passes and at the gate. */
export type Tone = 'student' | 'faculty' | 'guest' | 'exchange';

export const MEMBER_GROUPS: Group[] = ['pgp1', 'student', 'faculty'];

export const GROUP_LABEL: Record<Group, string> = {
  pgp1: 'PGP1', student: 'Student', faculty: 'Faculty & Staff', exchange: 'Exchange', guest: 'Guest',
};

export const TONES: Record<Tone, { color: string; label: string; id: string }> = {
  student: { color: '#e8317a', label: 'Student', id: 'student ID' },
  faculty: { color: '#2a5bd7', label: 'Faculty & Staff', id: 'IIMA ID' },
  guest: { color: '#f5872a', label: 'Guest', id: 'photo ID' },
  exchange: { color: '#1aa7a0', label: 'Exchange', id: 'college ID' },
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

export interface Limits {
  pgp1: number;
  student: number;
  faculty: number;
}

export interface Settings {
  event: EventInfo;
  limits: Limits;
  pgp1Prefixes: string[];
}

export interface PassView {
  id: string;
  code: string;            // KP7X-4MQ: random + check character, typed at the gate if a QR won't scan
  kind: PassKind;
  status: PassStatus;
  holderName: string;
  holderEmail: string | null;
  typeLabel: string;       // PGP1 / Student / Faculty & Staff / Guest / Exchange
  tone: Tone;
  college: string | null;
  issuerName: string | null;
  enteredAt: number | null;
  enteredGate: number | null;
  /** Key for the rotating QR; only sent to people allowed to show the pass. */
  key?: string;
}

export interface Me {
  id: string;
  email: string;
  name: string;
  group: Group;
  role: Role;
  groupLocked: boolean;
  hasLimit: boolean;
  guestLimit: number;
}

export interface MeResponse {
  user: Me;
  event: EventInfo;
  pass: PassView | null;
  guests: PassView[];
  limit: number;
  remaining: number;
  serverTime: number;
}

export interface LinkResponse {
  event: EventInfo;
  pass: PassView;
  serverTime: number;
}

export type ScanOutcome = 'allowed' | 'used' | 'revoked' | 'invalid' | 'expired' | 'check';

export interface ScanResult {
  outcome: ScanOutcome;
  title: string;
  sub: string;
  name: string;
  meta: string;
  tone: Tone | '';
  typeLabel: string;
  passId?: string;
  entered: number;
}

export interface AdminStats {
  entered: number;
  issued: number;
  members: number;
  guests: number;
  exchange: number;
  byGate: { gate: number; count: number }[];
  byKind: { kind: PassKind; issued: number; entered: number }[];
  recent: { name: string; code: string; kind: PassKind; gate: number; at: number }[];
}

export interface AdminPerson extends Me {
  guests: number;
  limit: number;
}

export interface ImportResult {
  created: number;
  updated: number;
  errors: { line: number; reason: string }[];
}

export const toneOfKind = (k: PassKind): Tone => (k === 'own' ? 'student' : k);
