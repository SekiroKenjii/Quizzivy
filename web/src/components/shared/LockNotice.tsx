import { useTranslation } from "react-i18next";
import { Lock } from "lucide-react";

/** LockNoticeProps supplies the lock label and its explanatory reason. */
export type LockNoticeProps = Readonly<{
  reason: string;
  label?: string | undefined;
}>;

/** LockNotice exposes its reason visually and to assistive technology without adding a tab stop. */
export function LockNotice({ reason, label }: LockNoticeProps) {
  const { t } = useTranslation();
  return (
    <span
      title={reason}
      className="text-muted-fg relative inline-flex items-center gap-1.25 text-xs leading-normal whitespace-nowrap"
    >
      <Lock aria-hidden="true" className="size-[0.8125rem] shrink-0" />
      {label ?? t("display.locked")} <span className="sr-only">{reason}</span>
    </span>
  );
}
