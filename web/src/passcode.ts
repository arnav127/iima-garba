// Pass codes like "KP7X-4MQ": 6 random characters + a Luhn mod-30 check character.
// Must match backend/garba/passcode.go.

const ALPHABET = '23456789ABCDEFGHJKMNPQRSTVWXYZ'; // no 0 O 1 I L U

function check(body: string): string {
  const n = ALPHABET.length;
  let factor = 2, sum = 0;
  for (let i = body.length - 1; i >= 0; i--) {
    const addend = factor * ALPHABET.indexOf(body[i]);
    factor = factor === 2 ? 1 : 2;
    sum += Math.floor(addend / n) + (addend % n);
  }
  return ALPHABET[(n - (sum % n)) % n];
}

/** Formats what a volunteer types as they type: upper-case, dash after 4 characters. */
export function formatTyped(input: string): string {
  const raw = input.toUpperCase().replace(/[^0-9A-Z]/g, '').slice(0, 7);
  return raw.length > 4 ? `${raw.slice(0, 4)}-${raw.slice(4)}` : raw;
}

/** complete: 7 characters typed. valid: the check character matches (no typo). badChar: a look-alike like O, 0, I, 1, L, U. */
export function checkTyped(input: string): { complete: boolean; valid: boolean; badChar: string | null } {
  const raw = input.toUpperCase().replace(/[^0-9A-Z]/g, '');
  const badChar = [...raw].find((c) => !ALPHABET.includes(c)) ?? null;
  const complete = raw.length === 7;
  return { complete, badChar, valid: complete && !badChar && check(raw.slice(0, 6)) === raw[6] };
}
