import { cn } from "@/lib/utils";
import type { FormInfoRow } from "./types";
const TONES = {
  neutral: "text-muted-fg",
  success: "text-success-ink",
  warning: "text-warning-ink",
  danger: "text-danger-ink",
  info: "text-info-ink",
};
/** InfoField draws informational rows and independent non-submit actions. */
export function InfoField({ rows }: Readonly<{ rows: readonly FormInfoRow[] }>) {
  return (
    <div className="flex flex-col overflow-hidden rounded-lg border">
      {rows.map(({ icon: Icon, tone = "neutral", text, action }, index) => (
        <div
          key={index}
          className="flex items-center gap-2.5 border-t px-3 py-2.5 text-sm first:border-t-0"
        >
          <Icon aria-hidden="true" className={cn("size-3.75 flex-none", TONES[tone])} />
          <span className="min-w-0 flex-1 leading-[1.45]">{text}</span>
          {action && (
            <button
              type="button"
              className="bg-card hover:bg-muted rounded-seg h-7 border px-2.5 text-xs font-medium whitespace-nowrap"
              onClick={action.onAction}
            >
              {action.label}
            </button>
          )}
        </div>
      ))}
    </div>
  );
}
