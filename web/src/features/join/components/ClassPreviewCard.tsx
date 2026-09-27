import { useTranslation } from "react-i18next";
import { CircleCheck } from "lucide-react";

import { Avatar } from "@/components/ui/avatar";

/**
 * ClassPreviewCard is what a valid code finds: the class name and its teacher,
 * and nothing else the public preview could leak (§6.5).
 */
export function ClassPreviewCard({
  name,
  teacherName,
}: Readonly<{ name: string; teacherName: string }>) {
  const { t } = useTranslation();
  return (
    <div className="shadow-card flex flex-col gap-3 rounded-2xl border p-4">
      <span className="bg-success-soft text-success-ink text-meta inline-flex items-center gap-1.5 self-start rounded-full px-[9px] py-0.5">
        <CircleCheck aria-hidden="true" className="size-[13px]" />
        {t("join.found")}
      </span>
      <p className="text-lg font-semibold break-words">{name}</p>
      <div className="flex items-center gap-2.5">
        <Avatar name={teacherName} size="36" className="size-8.5 font-semibold" />
        <span className="text-base font-medium break-words">{teacherName}</span>
      </div>
    </div>
  );
}
