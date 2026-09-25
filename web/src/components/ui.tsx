import type { ComponentChildren, JSX } from 'preact';
import { useEffect, useMemo, useState } from 'preact/hooks';
import { encode } from 'uqr';
import logoUrl from '../assets/cultcomm-160.webp';
import { toastStore } from '../lib.ts';

export const Rainbow = ({ h = 14 }: { h?: number }) => <div class="rainbow" style={{ height: h }} />;

export const Zigzag = ({ color = 'var(--pink)', h = 16 }: { color?: string; h?: number }) => (
  <div class="zigzag" style={{ height: h, '--zz': color } as JSX.CSSProperties} />
);

/** Row of mirror-work dots. `bg` defaults to ink; pass "transparent" to sit on an ink card. */
export const Mirrors = ({ color = 'var(--yellow)', h = 14, bg }: { color?: string; h?: number; bg?: string }) => (
  <div class="mirrors" style={{ height: h, '--mc': color, '--mh': `${h}px`, ...(bg ? { '--mb': bg } : {}) } as JSX.CSSProperties} />
);

// 160 px WebP (6 KB) covers the largest logo (52 px) on 3x screens. Imported so Vite hashes it into
// /assets/: cached for a year and kept offline by the service worker. The 447 px JPEG stays for app icons.
export const Logo = ({ size }: { size: number }) => (
  <img class="logo" src={logoUrl} alt="Cultcomm" width={size} height={size} style={{ width: size, height: size }} />
);

export const Top = () => <div class="top" />;

export function Back({ onClick, label, light }: { onClick: () => void; label?: string; light?: boolean }) {
  return (
    <button class="back" onClick={onClick} aria-label="Back" style={label ? { display: 'flex', gap: 12, alignItems: 'center', font: "500 15px var(--fb)", color: light ? 'var(--ivory)' : undefined } : undefined}>
      <span style={{ fontSize: 22 }}>←</span>{label}
    </button>
  );
}

/**
 * One-colour line icons drawn in the text colour. Unicode arrows like ↩ and ↗ turn into colour
 * emoji on iPhones, so buttons use these instead.
 */
const ICONS = {
  arrow: 'M5 12h14M13 6l6 6-6 6',
  undo: 'M9 14 4 9l5-5M4 9h10.5a5.5 5.5 0 0 1 0 11H11',
  logout: 'M9 20H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h3M16 17l5-5-5-5M21 12H9',
  x: 'M6 6l12 12M18 6 6 18',
  restore: 'M3.5 12a8.5 8.5 0 1 0 2.5-6M3.5 3.5V9H9',
  download: 'M12 4v11M7 10l5 5 5-5M5 20h14',
  upload: 'M12 20V9M7 14l5-5 5 5M5 4h14',
  share: 'M14 4h6v6M20 4l-9 9M18 14v4a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4',
  check: 'M5 12.5l4.5 4.5L19 7',
  plus: 'M12 5v14M5 12h14',
  qr: 'M4 4h6v6H4zM14 4h6v6h-6zM4 14h6v6H4zM14 14h2.5M14 17.5v2.5M17.5 17.5H20M20 14v.01M17 20h.01',
  alert: 'M12 6v8M12 18v.01',
} as const;
export type IconName = keyof typeof ICONS;

export const Icon = ({ name, size = 20 }: { name: IconName; size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.25" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" style={{ flex: 'none', display: 'block' }}>
    <path d={ICONS[name]} />
  </svg>
);

export function Btn(props: { children: ComponentChildren; onClick?: () => void; disabled?: boolean; variant?: string; icon?: IconName; type?: 'submit' | 'button'; style?: JSX.CSSProperties }) {
  return (
    <button type={props.type ?? 'button'} class={`btn ${props.variant ?? ''}`} onClick={props.onClick} disabled={props.disabled} style={props.style}>
      <span>{props.children}</span><Icon name={props.icon ?? 'arrow'} />
    </button>
  );
}

/**
 * A button that asks "Tap again to …" before acting. Used instead of window.confirm(), which many
 * phone and in-app browsers (WhatsApp, Instagram, installed PWAs) block silently, so the tap did nothing.
 */
export function ConfirmBtn(props: { children: ComponentChildren; confirmLabel: string; onConfirm: () => void; disabled?: boolean; variant?: string; icon?: IconName }) {
  const [armed, setArmed] = useState(false);
  useEffect(() => {
    if (!armed) return;
    const t = setTimeout(() => setArmed(false), 4000);
    return () => clearTimeout(t);
  }, [armed]);
  return (
    <Btn variant={armed ? 'danger' : props.variant} disabled={props.disabled} icon={armed ? 'alert' : props.icon}
      onClick={() => { if (armed) { setArmed(false); props.onConfirm(); } else setArmed(true); }}>
      {armed ? props.confirmLabel : props.children}
    </Btn>
  );
}

/** Real QR code for the pass payload, drawn as crisp SVG modules. */
export function QrCode({ value, size = 175, color = '#151515' }: { value: string; size?: number; color?: string }) {
  const path = useMemo(() => {
    const { data, size: n } = encode(value, { ecc: 'M', border: 0 });
    let d = '';
    for (let y = 0; y < n; y++) for (let x = 0; x < n; x++) if (data[y][x]) d += `M${x} ${y}h1v1h-1z`;
    return { d, n };
  }, [value]);
  return (
    <svg width={size} height={size} viewBox={`0 0 ${path.n} ${path.n}`} shape-rendering="crispEdges" role="img" aria-label="Entry QR code">
      <path d={path.d} fill={color} />
    </svg>
  );
}

/** Small decorative 5x5 QR glyph for the pass card, stable per pass code. */
export function MiniQr({ seed }: { seed: string }) {
  const cells = useMemo(() => {
    let s = [...seed].reduce((a, c) => a * 31 + c.charCodeAt(0), 5) % 233280;
    return Array.from({ length: 25 }, () => ((s = (s * 9301 + 49297) % 233280) / 233280) > 0.45);
  }, [seed]);
  return (
    <div style={{ width: 58, height: 58, borderRadius: 12, background: 'var(--ivory)', display: 'grid', gridTemplateColumns: 'repeat(5,1fr)', gap: 3, padding: 7, flex: 'none' }}>
      {cells.map((on, i) => <div key={i} style={{ background: on ? 'var(--ink)' : 'transparent', borderRadius: 1 }} />)}
    </div>
  );
}

export function Sheet({ onClose, children, stripe = 'var(--pink)' }: { onClose: () => void; children: ComponentChildren; stripe?: string }) {
  return (
    <div class="overlay" onClick={onClose} role="dialog" aria-modal="true">
      <div class="sheet" onClick={(e) => e.stopPropagation()}>
        <Zigzag color={stripe} />
        <div class="sheet-body">{children}</div>
      </div>
    </div>
  );
}

export function Toast() {
  const msg = toastStore.use();
  return msg ? <div class="toast" role="status">{msg}</div> : null;
}

export const Spinner = () => <div style={{ flex: 1, display: 'grid', placeItems: 'center', minHeight: 200 }}><div class="spin" /></div>;

/** Event facts grid on the landing screen. */
export function FactGrid({ items }: { items: [string, string][] }) {
  return (
    <div style={{ margin: '22px 22px 0', border: '2px solid var(--ink)', borderRadius: 18, display: 'grid', gridTemplateColumns: `repeat(${items.length},1fr)`, overflow: 'hidden' }}>
      {items.map(([k, v], i) => (
        <div key={k} style={{ padding: 12, display: 'flex', flexDirection: 'column', borderRight: i < items.length - 1 ? '2px solid var(--ink)' : undefined, minWidth: 0 }}>
          <span style={{ font: '600 11px var(--fb)', letterSpacing: '.1em', color: 'var(--muted)' }}>{k}</span>
          <span style={{ font: '700 17px/1.1 var(--fd)', overflowWrap: 'anywhere' }}>{v}</span>
        </div>
      ))}
    </div>
  );
}
