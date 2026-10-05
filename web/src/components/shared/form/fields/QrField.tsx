import { QRCodeSVG } from "qrcode.react";
/** QrField encodes its payload on paper and displays the caller's code and explanation. */
export function QrField({
  payload,
  value,
  text,
}: Readonly<{ payload: string; value: string; text: string }>) {
  return (
    <div className="flex flex-col items-center gap-2.5 py-2">
      <span className="bg-paper text-paper-fg grid size-50 place-items-center rounded-xl border">
        <QRCodeSVG
          value={payload}
          size={150}
          bgColor="var(--paper)"
          fgColor="var(--paper-fg)"
        />
      </span>
      <span className="text-stat font-mono font-semibold tracking-[.1em]">{value}</span>
      <span className="text-muted-fg text-meta max-w-full text-center wrap-anywhere">
        {text}
      </span>
    </div>
  );
}
