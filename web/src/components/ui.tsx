import type { ComponentChildren, JSX } from 'preact';
import { useMemo } from 'preact/hooks';
import { encode } from 'uqr';
import { asset, toastStore } from '../lib.ts';

export const Rainbow = ({ h = 14 }: { h?: number }) => <div class="rainbow" style={{ height: h }} />;

export const Zigzag = ({ color = 'var(--pink)', h = 16 }: { color?: string; h?: number }) => (
  <div class="zigzag" style={{ height: h, '--zz': color } as JSX.CSSProperties} />
);

/** Row of mirror-work dots. `bg` defaults to ink; pass "transparent" to sit on an ink card. */
export const Mirrors = ({ color = 'var(--yellow)', h = 14, bg }: { color?: string; h?: number; bg?: string }) => (
  <div class="mirrors" style={{ height: h, '--mc': color, '--mh': `${h}px`, ...(bg ? { '--mb': bg } : {}) } as JSX.CSSProperties} />
);

export const Logo = ({ size }: { size: number }) => (
  <img class="logo" src={asset('cultcomm.jpg')} alt="Cultcomm" width={size} height={size} style={{ width: size, height: size }} />
);

export const Top = () => <div class="top" />;

export function Back({ onClick, label, light }: { onClick: () => void; label?: string; light?: boolean }) {
  return (
    <button class="back" onClick={onClick} aria-label="Back" style={label ? { display: 'flex', gap: 12, alignItems: 'center', font: "500 15px var(--fb)", color: light ? 'var(--ivory)' : undefined } : undefined}>
      <span style={{ fontSize: 22 }}>←</span>{label}
    </button>
  );
}

export function Btn(props: { children: ComponentChildren; onClick?: () => void; disabled?: boolean; variant?: string; icon?: string; type?: 'submit' | 'button'; style?: JSX.CSSProperties }) {
  return (
    <button type={props.type ?? 'button'} class={`btn ${props.variant ?? ''}`} onClick={props.onClick} disabled={props.disabled} style={props.style}>
      <span>{props.children}</span><span aria-hidden="true">{props.icon ?? '→'}</span>
    </button>
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
