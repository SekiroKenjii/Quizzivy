import { useEffect, useRef, useState } from "react";
import { get, useFormContext, useWatch } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { BrandMark } from "@/components/shared/Brand";
import { SettingsCard } from "@/components/shared/SettingsCard";
import type { LogoVariant } from "@/features/organization/api";
import {
  LogoDialog,
  type UploadedLogo,
} from "@/features/organization/components/LogoDialog";
import { SettingsField } from "@/features/organization/components/SettingsField";
import {
  FIELD_GRID,
  FIELD_INPUT,
  controlId,
  describedBy,
} from "@/features/organization/controls";
import { fieldMessage } from "@/features/organization/fieldMessage";
import type { SettingsFormValues } from "@/features/organization/settingsFormSchema";
import { storedLogoUrl } from "@/lib/organization";
import { useResolvedTheme } from "@/lib/theme";
import { cn } from "@/lib/utils";

type TextField =
  | "name"
  | "shortName"
  | "website"
  | "contactEmail"
  | "frontDeskPhone"
  | "frontDeskHours"
  | "address";

const FIELDS: readonly {
  name: TextField;
  label: string;
  hint?: string;
  type?: "email" | "tel" | "url";
  inputMode?: "email" | "tel" | "url";
}[] = [
  { name: "name", label: "name" },
  { name: "shortName", label: "shortName", hint: "shortNameHint" },
  { name: "website", label: "website", inputMode: "url" },
  { name: "contactEmail", label: "contactEmail", type: "email", inputMode: "email" },
  { name: "frontDeskPhone", label: "frontDeskPhone", type: "tel", inputMode: "tel" },
  { name: "frontDeskHours", label: "frontDeskHours" },
  { name: "address", label: "address" },
];

function LogoRow() {
  const { t } = useTranslation();
  const form = useFormContext<SettingsFormValues>();
  const light = useWatch({ control: form.control, name: "organization.logo.light" });
  const dark = useWatch({ control: form.control, name: "organization.logo.dark" });
  const theme = useResolvedTheme();
  const [open, setOpen] = useState(false);
  const opener = useRef<HTMLButtonElement>(null);
  const previews = useRef<string[]>([]);
  useEffect(
    () => () => {
      for (const url of previews.current) URL.revokeObjectURL(url);
    },
    [],
  );

  function adopt(variant: "light" | "dark", logo: UploadedLogo | null) {
    if (logo === null) return;
    previews.current.push(logo.preview);
    form.setValue(`organization.logo.${variant}`, logo, {
      shouldDirty: true,
      shouldTouch: true,
    });
  }

  const onDark = theme === "dark";
  const drawn: { logo: typeof light; variant: LogoVariant } =
    onDark && dark !== null
      ? { logo: dark, variant: "dark" }
      : { logo: light, variant: "light" };
  const shown =
    drawn.logo === null
      ? null
      : (drawn.logo.preview ?? storedLogoUrl(drawn.variant, drawn.logo.sha256));
  const flipped = onDark && drawn.variant === "light";

  return (
    <div className="flex flex-wrap items-center gap-3.5">
      <span
        className={cn(
          "grid size-14 flex-none place-items-center rounded-[14px] border",
          flipped ? "bg-fg" : "bg-card",
        )}
      >
        {shown === null ? (
          <BrandMark height={28} label={false} theme="auto" />
        ) : (
          <img
            src={shown}
            alt={t("systemSettings.logo.alt")}
            className="size-10 object-contain"
          />
        )}
      </span>
      <Button
        ref={opener}
        id={controlId("organization.logo.light")}
        type="button"
        variant="outline"
        className="h-8.5"
        onClick={() => setOpen(true)}
      >
        {t("systemSettings.logo.change")}
      </Button>
      <span className="text-muted-fg text-meta">
        {dark === null
          ? t("systemSettings.logo.hint")
          : t("systemSettings.logo.hintWithDark")}
      </span>
      <LogoDialog
        open={open}
        onOpenChange={setOpen}
        returnFocus={opener}
        onUploaded={(logos) => {
          adopt("light", logos.light);
          adopt("dark", logos.dark);
        }}
      />
    </div>
  );
}

/**
 * OrganizationSection is the Organization card of System settings: the logo,
 * with Replace logo, and who the centre is and how it is reached, the name,
 * the short name, the website and DG-41's contact email, front-desk phone,
 * front-desk hours and address. It edits the page's one form.
 */
export function OrganizationSection() {
  const { t } = useTranslation();
  const form = useFormContext<SettingsFormValues>();
  const errors = form.formState.errors;
  return (
    <SettingsCard
      title={t("systemSettings.organization.title")}
      description={t("systemSettings.organization.hint")}
    >
      <div className="flex flex-col gap-4 p-4.5">
        <LogoRow />
        <div className={FIELD_GRID}>
          {FIELDS.map(({ name, label, hint, type, inputMode }) => {
            const id = controlId(`organization.${name}`);
            const message = fieldMessage(get(errors, `organization.${name}`), t);
            return (
              <SettingsField
                key={name}
                id={id}
                label={t(`systemSettings.organization.${label}`)}
                hint={
                  hint === undefined
                    ? undefined
                    : t(`systemSettings.organization.${hint}`)
                }
                error={message}
              >
                <Input
                  id={id}
                  className={FIELD_INPUT}
                  type={type ?? "text"}
                  inputMode={inputMode}
                  autoComplete="off"
                  aria-invalid={message === undefined ? undefined : true}
                  aria-describedby={describedBy(
                    id,
                    hint !== undefined,
                    message !== undefined,
                  )}
                  {...form.register(`organization.${name}`)}
                />
              </SettingsField>
            );
          })}
        </div>
      </div>
    </SettingsCard>
  );
}
