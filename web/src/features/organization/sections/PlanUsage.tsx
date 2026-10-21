import { useQuery } from "@tanstack/react-query";
import type { TFunction } from "i18next";
import { ArrowRight } from "lucide-react";
import {
  Controller,
  get,
  useFormContext,
  useWatch,
  type FieldPath,
} from "react-hook-form";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { ADMIN_PATHS } from "@/app/adminPaths";
import { SettingsCard } from "@/components/shared/SettingsCard";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { credits } from "@/features/ai/credits";
import { formatStorage } from "@/features/media/format";
import {
  usageQuery,
  type OrganizationSettings,
  type Usage,
} from "@/features/organization/api";
import { FIELD_INPUT, controlId } from "@/features/organization/controls";
import { fieldMessage } from "@/features/organization/fieldMessage";
import {
  inUnit,
  isByteLimit,
  typedLimit,
  type ByteLimitValue,
  type ByteUnit,
  type LimitName,
  type UsageLimitDefaults,
} from "@/features/organization/limits";
import type { SettingsFormValues } from "@/features/organization/settingsFormSchema";
import type { Locale } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n/useLocale";
import { cn } from "@/lib/utils";

const GROUPS: readonly { id: string; rows: readonly LimitName[] }[] = [
  { id: "people", rows: ["seats", "teachers"] },
  {
    id: "storage",
    rows: [
      "storageBytes",
      "mediaPerTeacherBytes",
      "importBytes",
      "importPerTeacherBytes",
    ],
  },
  { id: "email", rows: ["emailsPerDay", "emailsPerMonth"] },
  { id: "database", rows: ["databaseBytes", "warnAtPercent"] },
];

function usedOf(name: LimitName, usage: Usage | undefined): number | undefined {
  if (usage === undefined) return undefined;
  switch (name) {
    case "seats":
      return usage.seats.used;
    case "teachers":
      return usage.teachers.used;
    case "storageBytes":
      return usage.storage.bytes;
    case "mediaPerTeacherBytes":
      return usage.media.largestLibrary;
    case "importBytes":
      return usage.imports.openBytes;
    case "emailsPerDay":
      return usage.email.today;
    case "emailsPerMonth":
      return usage.email.month;
    case "databaseBytes":
      return usage.database.bytes;
    default:
      return undefined;
  }
}

const UNITS: readonly ByteUnit[] = ["MB", "GB"];

function reading(name: LimitName, limit: number | null, usage: Usage | undefined) {
  const used = usedOf(name, usage);
  if (used === undefined || limit === null || usage === undefined) {
    return { used, below: false, near: false };
  }
  const below = used > limit;
  return { used, below, near: !below && used * 100 >= limit * usage.warnAtPercent };
}

function shownIn(bytesOrCount: number, value: string | ByteLimitValue): string {
  return typeof value === "string"
    ? String(bytesOrCount)
    : inUnit(bytesOrCount, value.unit);
}

function amount(name: LimitName, value: number, locale: Locale): string {
  if (isByteLimit(name)) return formatStorage(value);
  if (name === "warnAtPercent") return `${value}%`;
  return new Intl.NumberFormat(locale).format(value);
}

function defaultLine(
  name: LimitName,
  defaults: UsageLimitDefaults,
  locale: Locale,
  t: TFunction,
): string {
  const { value, source } = defaults[name];
  if (value === null) return t("systemSettings.plan.defaultNone");
  return t("systemSettings.plan.default", {
    value: amount(name, value, locale),
    source: t(`systemSettings.plan.sources.${source}`),
  });
}

function UnitSelect({
  name,
  saving,
  label,
}: Readonly<{ name: LimitName; saving: boolean; label: string }>) {
  const { t } = useTranslation();
  const { control } = useFormContext<SettingsFormValues>();
  if (!isByteLimit(name)) {
    return (
      <span className="text-muted-fg text-ui min-w-14">
        {t(`systemSettings.plan.units.${name}`)}
      </span>
    );
  }
  return (
    <Controller
      control={control}
      name={`limits.${name}.unit`}
      render={({ field }) => (
        <Select
          value={field.value}
          onValueChange={(next) => {
            if (!saving) field.onChange(next);
          }}
        >
          <SelectTrigger
            ref={field.ref}
            aria-label={t("systemSettings.plan.unitOf", { name: label })}
            aria-disabled={saving || undefined}
            className="aria-disabled:text-foreground/50 h-9.5 w-auto min-w-14"
            onBlur={field.onBlur}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {UNITS.map((unit) => (
              <SelectItem key={unit} value={unit}>
                {unit}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
    />
  );
}

function LimitRow({
  name,
  defaults,
  usage,
  saving,
}: Readonly<{
  name: LimitName;
  defaults: UsageLimitDefaults;
  usage: Usage | undefined;
  saving: boolean;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const form = useFormContext<SettingsFormValues>();
  const value = useWatch({ control: form.control, name: `limits.${name}` });
  const id = controlId(`limits.${name}`);
  const path = (
    isByteLimit(name) ? `limits.${name}.amount` : `limits.${name}`
  ) as FieldPath<SettingsFormValues>;
  const typed = typeof value === "string" ? value : value.amount;
  const message = fieldMessage(get(form.formState.errors, `limits.${name}`), t);
  const label = t(`systemSettings.plan.rows.${name}.label`);
  const fallback = defaults[name].value;
  const { used, below, near } = reading(name, typedLimit(value) ?? fallback, usage);
  const placeholder =
    fallback === null ? t("systemSettings.plan.noLimit") : shownIn(fallback, value);
  const notes = [`${id}-hint`, used === undefined ? null : `${id}-use`];
  if (below) notes.push(`${id}-below`);
  notes.push(`${id}-default`);
  return (
    <div className="flex flex-wrap items-start gap-x-3.5 gap-y-2 px-4.5 py-3">
      <div className="min-w-52 flex-1">
        <label htmlFor={id} className="text-ui block leading-normal font-medium">
          {label}
        </label>
        <p id={`${id}-hint`} className="text-muted-fg text-meta leading-normal">
          {t(`systemSettings.plan.rows.${name}.hint`)}
        </p>
        {used === undefined ? null : (
          <p
            id={`${id}-use`}
            className={cn(
              "text-meta leading-normal tabular-nums",
              near && "text-warning-ink",
            )}
          >
            {t(`systemSettings.plan.rows.${name}.used`, {
              value: amount(name, used, locale),
            })}
          </p>
        )}
        {below ? (
          <p id={`${id}-below`} className="text-warning-ink text-meta leading-normal">
            {t(`systemSettings.plan.rows.${name}.below`)}
          </p>
        ) : null}
      </div>
      <div className="flex max-w-full flex-col gap-1.5">
        <div className="flex items-center gap-2">
          <Input
            id={id}
            className={cn(FIELD_INPUT, "w-32 tabular-nums")}
            inputMode={isByteLimit(name) ? "decimal" : "numeric"}
            autoComplete="off"
            placeholder={placeholder}
            aria-invalid={message === undefined ? undefined : true}
            aria-describedby={
              message === undefined ? notes.filter(Boolean).join(" ") : `${id}-error`
            }
            {...form.register(path)}
          />
          <UnitSelect name={name} saving={saving} label={label} />
        </div>
        {message === undefined ? null : (
          <p id={`${id}-error`} className="text-danger-ink text-xs leading-normal">
            {message}
          </p>
        )}
        <div className="flex flex-wrap items-center gap-x-2">
          <p id={`${id}-default`} className="text-muted-fg text-xs leading-normal">
            {defaultLine(name, defaults, locale, t)}
          </p>
          {typed.trim() === "" ? null : (
            <Button
              type="button"
              variant="link"
              size="xs"
              className="h-auto px-0"
              aria-label={t("systemSettings.plan.useDefaultFor", { name: label })}
              onClick={() => {
                if (saving) return;
                form.setValue(path, "" as never, {
                  shouldDirty: true,
                  shouldTouch: true,
                  shouldValidate: true,
                });
              }}
            >
              {t("systemSettings.plan.useDefault")}
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}

function CreditsRow({ usage }: Readonly<{ usage: Usage }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const { aiCredits } = usage;
  if (aiCredits === null) return null;
  return (
    <SettingsCard
      title={t("systemSettings.plan.aiCredits")}
      description={t("systemSettings.plan.aiCreditsOf", {
        used: credits(aiCredits.used, locale),
        pool: credits(aiCredits.pool, locale),
      })}
      action={
        <Link
          to={ADMIN_PATHS.ai}
          className="text-muted-fg hover:text-fg flex items-center gap-1 text-sm transition-colors ease-[ease]"
        >
          {t("systemSettings.plan.aiCreditsManage")}
          <ArrowRight aria-hidden="true" className="size-3.5" />
        </Link>
      }
      headerClassName="border-b-0"
    >
      {null}
    </SettingsCard>
  );
}

/**
 * PlanUsageSection is the Plan & usage cards of System settings (CR-002): the
 * centre's usage limits, grouped as people, file storage, email and the
 * database, each a field that joins the page's unsaved changes. A row says
 * what the limit bounds, what is in use now when the usage could be read, and
 * the default an empty field takes and where it comes from, with Use default
 * once a value is typed. A limit below what is in use is allowed and says
 * what it will refuse; nothing is disabled. The month's AI credits are a
 * line with a link to where they are edited, there only while the server
 * reports them. The meters are the Overview's.
 */
export function PlanUsageSection({
  saving,
  defaults,
}: Readonly<{ saving: boolean; defaults: OrganizationSettings["limitDefaults"] }>) {
  const { t } = useTranslation();
  const usage = useQuery(usageQuery());
  return (
    <>
      {GROUPS.map((group) => (
        <SettingsCard
          key={group.id}
          title={t(`systemSettings.plan.groups.${group.id}.title`)}
          description={t(`systemSettings.plan.groups.${group.id}.hint`)}
        >
          <div className="divide-y">
            {group.rows.map((name) => (
              <LimitRow
                key={name}
                name={name}
                defaults={defaults}
                usage={usage.data}
                saving={saving}
              />
            ))}
          </div>
        </SettingsCard>
      ))}
      {usage.data === undefined ? null : <CreditsRow usage={usage.data} />}
      {usage.isError ? (
        <p role="status" className="text-muted-fg text-meta">
          {t("systemSettings.plan.loadFailed")}
        </p>
      ) : null}
    </>
  );
}
