import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Pencil } from "lucide-react";
import { SettingsCard } from "@/components/shared/SettingsCard";
import { ProgressBar } from "@/components/shared/stats/ProgressBar";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import type {
  AiAdminCredits,
  AiTeacherMonth,
  AiTeacherStatus,
} from "@/features/ai-credits/api";
import { byUse, teacherTone } from "@/features/ai-credits/figures";
import { credits } from "@/features/ai/credits";
import { useLocale } from "@/lib/i18n/useLocale";

const PILLS: Record<AiTeacherStatus, "success" | "warning" | "danger"> = {
  on_track: "success",
  near_limit: "warning",
  at_limit: "danger",
  paused: "danger",
};

function TeacherRow({
  teacher,
  onEdit,
}: Readonly<{
  teacher: AiTeacherMonth;
  onEdit: (teacher: AiTeacherMonth, trigger: HTMLElement) => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const used = credits(teacher.used, locale);
  const allowance = credits(teacher.allowance, locale);
  return (
    <li className="flex flex-wrap items-center gap-x-4 gap-y-2.5 border-t px-4 py-3 first:border-t-0">
      <div className="flex min-w-0 flex-[1_1_10rem] items-center gap-2.5">
        <Avatar name={teacher.name} />
        <div className="min-w-0">
          <p className="text-ui truncate leading-normal font-medium">{teacher.name}</p>
          <p className="text-muted-fg truncate text-xs leading-normal">
            {teacher.email}
          </p>
        </div>
      </div>
      <div className="flex min-w-0 flex-[2_1_11.875rem] flex-col gap-1.5">
        <div className="text-meta flex justify-between gap-2 tabular-nums">
          <span className="font-medium">{used}</span>
          <span className="text-muted-fg">
            {teacher.custom
              ? t("aiCredits.teachers.ofCustom", { allowance })
              : t("aiCredits.teachers.of", { allowance })}
          </span>
        </div>
        <ProgressBar
          value={teacher.used}
          max={teacher.allowance}
          size={6}
          tone={teacherTone(teacher.status)}
          label={t("aiCredits.teachers.bar", { name: teacher.name, used, allowance })}
        />
      </div>
      <div className="flex flex-none items-center gap-4">
        <div className="w-31">
          <Badge variant={PILLS[teacher.status]} dot>
            {t(`aiCredits.teachers.status.${teacher.status}`)}
          </Badge>
        </div>
        <button
          type="button"
          data-edit-allowance={teacher.userId}
          aria-label={t("aiCredits.teachers.edit", { name: teacher.name })}
          title={t("aiCredits.teachers.edit", { name: teacher.name })}
          className="text-muted-fg hover:bg-hover hover:text-fg rounded-seg grid size-7.5 flex-none place-items-center"
          onClick={(event) => onEdit(teacher, event.currentTarget)}
        >
          <Pencil aria-hidden="true" className="size-3.75" />
        </button>
      </div>
    </li>
  );
}

/**
 * TeachersCard lists the teachers with their month, the most used of their
 * allowance first: the person, the credits used against the allowance on a
 * 6px bar, their standing as the server reads it and a pencil that opens the
 * allowance. Its header counts the teachers who used AI and those who have
 * not yet.
 */
export function TeachersCard({
  view,
  onEdit,
}: Readonly<{
  view: AiAdminCredits;
  onEdit: (teacher: AiTeacherMonth, trigger: HTMLElement) => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const rows = useMemo(() => byUse(view.teachers, locale), [view.teachers, locale]);
  return (
    <SettingsCard
      title={t("aiCredits.teachers.title")}
      action={
        <span className="text-muted-fg text-meta">
          {t("aiCredits.teachers.summary", { count: view.active, idle: view.idle })}
        </span>
      }
      className="min-w-0 flex-[2_1_35rem]"
      headerClassName="px-4 py-3.5"
    >
      {rows.length === 0 ? (
        <p className="text-muted-fg px-4.5 py-5 text-sm">
          {t("aiCredits.teachers.empty")}
        </p>
      ) : (
        <ul>
          {rows.map((teacher) => (
            <TeacherRow key={teacher.userId} teacher={teacher} onEdit={onEdit} />
          ))}
        </ul>
      )}
    </SettingsCard>
  );
}
