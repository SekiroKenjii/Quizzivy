import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { BrandFrame } from "@/components/shared/BrandFrame";
import { OrganizationIdentity } from "@/components/shared/OrganizationIdentity";

/**
 * AuthLayout is the frame of the sign-in pages: the brand panel from 900px
 * with the product line and the organisation's logo and name, then the form column.
 * Join and its confirmation pass `panel={false}` and stay one column at every
 * width, as the deck draws them. `footer` sits under the form.
 */
export function AuthLayout({
  children,
  panel = true,
  footer,
}: Readonly<{ children: ReactNode; panel?: boolean; footer?: ReactNode }>) {
  const { t } = useTranslation();
  return (
    <BrandFrame
      from="auth"
      showPanel={panel}
      panel={
        <div className="flex max-w-[420px] flex-col gap-3.5">
          <p className="text-kpi leading-tight font-semibold tracking-[-0.02em] text-balance">
            {t("auth.panel.headline")}
          </p>
          <OrganizationIdentity variant="panel" />
        </div>
      }
      caption={t("auth.panel.help")}
    >
      <div className="flex flex-col gap-5">{children}</div>
      {footer && <div className="text-muted-fg text-ui mt-5 text-center">{footer}</div>}
    </BrandFrame>
  );
}
