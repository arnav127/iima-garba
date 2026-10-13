import { useEffect } from 'preact/hooks';
import type { PrintBatch, PrintPass } from '../../../shared/types.ts';
import { Logo, QrCode } from '../components/ui.tsx';
import { OkkaiLogo } from '../components/Sponsor.tsx';

const PER_PAGE = 8;

/**
 * Printable A4 sheets of paper passes, 8 to a page (2 x 4) with dashed cut lines.
 * The browser's own print window makes the PDF ("Save as PDF") or prints directly.
 * Laid out in millimetres, and readable in black and white: text and the QR are solid black,
 * colour is only decoration.
 */
export function PrintSheet({ batch, onClose }: { batch: PrintBatch; onClose: () => void }) {
  const live = batch.passes.filter((p) => p.status === 'ACTIVE' && !p.enteredAt);
  const pages: PrintPass[][] = [];
  for (let i = 0; i < live.length; i += PER_PAGE) pages.push(live.slice(i, i + PER_PAGE));
  const skipped = batch.passes.length - live.length;

  // The print window uses the page title as the PDF's file name.
  useEffect(() => {
    const title = document.title;
    document.title = `Garba passes - ${batch.label} - ${live.length}`;
    return () => { document.title = title; };
  }, [batch.batch]);

  return (
    <div class="print-view">
      <div class="print-bar">
        <button class="btn ghost small" style={{ width: 'auto' }} onClick={onClose}><span>← Back</span></button>
        <div style={{ flex: 1, minWidth: 180 }}>
          <div style={{ font: '800 20px/1.1 var(--fd)' }}>{live.length} PASSES · {pages.length} {pages.length === 1 ? 'PAGE' : 'PAGES'}</div>
          <div style={{ font: '400 12px/1.4 var(--fb)', color: 'var(--soft)' }}>
            A4, {PER_PAGE} per page. In the print window pick <b>Save as PDF</b> (or your printer), paper <b>A4</b>, scale <b>100%</b>. Cut along the dashed lines.
            {skipped > 0 && ` ${skipped} already used or cancelled ${skipped === 1 ? 'pass is' : 'passes are'} left out.`}
          </div>
        </div>
        <button class="btn yellow small" style={{ width: 'auto' }} disabled={live.length === 0} onClick={() => window.print()}><span>Print / Save as PDF</span></button>
      </div>
      <div class="print-pages">
        {pages.map((page, i) => (
          <div key={i} class="print-page">
            {page.map((p) => <PaperPass key={p.code} pass={p} batch={batch} />)}
          </div>
        ))}
        {live.length === 0 && <div style={{ color: '#fff', padding: 40, font: '500 15px var(--fb)' }}>Every pass in this run has been used or cancelled.</div>}
      </div>
    </div>
  );
}

function PaperPass({ pass, batch }: { pass: PrintPass; batch: PrintBatch }) {
  const { event } = batch;
  const no = pass.name.match(/(\d+)\s*$/)?.[1] ?? '';
  return (
    <div class="pp">
      <div class="rainbow pp-rainbow" />
      <div class="zigzag pp-zigzag" />
      <div class="pp-body">
        <div class="pp-left">
          <div class="pp-org"><Logo size={26} /><span>CULTCOMM<br />IIM AHMEDABAD</span></div>
          <div class="pp-title">{event.title.toUpperCase()}</div>
          <div class="pp-when">{event.dateLabel} · {event.venueShort} · {event.timeLabel}</div>
          <div class="pp-band"><span>{batch.label.toUpperCase()}</span>{no && <span class="pp-no">No. {no}</span>}</div>
          <div class="pp-note">Entry for <b>one person</b>. Give this pass to the volunteer at the gate. It works once, so a photocopy will be refused.</div>
          <div class="pp-sponsor"><span>STYLED BY</span><OkkaiLogo height={13} /></div>
        </div>
        <div class="pp-right">
          <div class="pp-qr"><QrCode value={pass.qr} size={140} /></div>
          <div class="pp-code">{pass.code}</div>
        </div>
      </div>
      <div class="mirrors pp-mirrors" />
    </div>
  );
}
