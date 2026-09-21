import type { ReactNode } from "react";

/** F-11's panel heading: the small uppercase label G-01's "Tóm tắt" uses. */
export function PanelLabel({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <p className="text-muted-foreground mb-3 text-xs font-medium tracking-wide uppercase">
      {children}
    </p>
  );
}

/** A label/value line under a PanelLabel, as G-01 lists an assignment's facts. */
export function PanelRow({
  label,
  children,
}: Readonly<{ label: string; children: ReactNode }>) {
  return (
    <div className="flex justify-between gap-3 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-medium">{children}</span>
    </div>
  );
}
