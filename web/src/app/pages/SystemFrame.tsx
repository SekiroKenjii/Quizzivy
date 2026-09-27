import type { ReactNode } from "react";

import { BrandFrame } from "@/components/shared/BrandFrame";
import { config } from "@/lib/config";

/**
 * SystemFrame is the frame of the system pages (not found, no access, error,
 * maintenance): the brand panel from 1024px with the page's drawing and the
 * organisation's name, beside the page's column.
 */
export function SystemFrame({
  art,
  children,
}: Readonly<{ art: ReactNode; children: ReactNode }>) {
  return (
    <BrandFrame
      from="lg"
      panel={
        <div className="text-fg self-center" aria-hidden="true">
          {art}
        </div>
      }
      caption={config.orgName}
    >
      {children}
    </BrandFrame>
  );
}
