import { useState, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { ImageIcon } from "lucide-react";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
import {
  uploadOrganizationLogo,
  type LogoVariant,
  type OrganizationLogo,
} from "@/features/organization/api";
import {
  LOGO_TYPES,
  LogoRefused,
  logoProblem,
  prepareLogo,
  type LogoProblem,
} from "@/features/organization/logoImage";
import { toast } from "@/components/ui/sonner";
import { ApiError, failureMessage } from "@/lib/api/errors";

/**
 * UploadedLogo is a logo the server now holds: the hash a save names, and the
 * address of the PNG that was sent, for showing it before the save.
 */
export type UploadedLogo = OrganizationLogo & Readonly<{ preview: string }>;

type LogoValues = { light: File | null; dark: File | null };

const INITIAL: LogoValues = { light: null, dark: null };
const ACCEPT = [".svg", ".png", ...LOGO_TYPES].join(",");
const VARIANTS = ["light", "dark"] as const satisfies readonly LogoVariant[];

const FILE_REFUSALS: ReadonlySet<number> = new Set([400, 413, 415]);

type Refusal = Readonly<{ variant: LogoVariant; file: File; message: string }>;
type Stored = Readonly<{ logo: OrganizationLogo; png: File }>;

async function store(file: File, variant: LogoVariant): Promise<Stored> {
  const png = await prepareLogo(file);
  const { sha256, type } = await uploadOrganizationLogo(png, variant);
  return { logo: { sha256, type }, png };
}

function uploaded({ logo, png }: Stored): UploadedLogo {
  return { ...logo, preview: URL.createObjectURL(png) };
}

function problemText(problem: LogoProblem, t: TFunction): string {
  return t(`systemSettings.logo.problem.${problem}`);
}

function refusalFor(cause: unknown, variant: LogoVariant, file: File, t: TFunction) {
  if (cause instanceof LogoRefused)
    return { variant, file, message: problemText(cause.problem, t) };
  if (cause instanceof ApiError && FILE_REFUSALS.has(cause.status))
    return { variant, file, message: cause.message };
  return null;
}

/**
 * LogoDialog is "Replace logo": a required file for the logo on a light
 * ground and an optional one for a dark ground, each a PNG or an SVG at least
 * 256 x 256 and square. An SVG is drawn to a PNG here, because the server
 * takes only PNG (D-019). Saving the dialog uploads the files; nothing applies
 * until the settings are saved, which the page says. A file this browser can
 * tell is unusable is refused on its field before anything is sent, and one
 * the server refuses shows the server's message on its field, until another
 * file is chosen. `onUploaded` is given the logos that were stored.
 */
export function LogoDialog({
  open,
  onOpenChange,
  onUploaded,
  returnFocus,
}: Readonly<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onUploaded: (logos: { light: UploadedLogo; dark: UploadedLogo | null }) => void;
  returnFocus: RefObject<HTMLElement | null>;
}>) {
  const { t } = useTranslation();
  const [pending, setPending] = useState(false);
  const [refusal, setRefusal] = useState<Refusal | null>(null);
  const [error, setError] = useState<string | null>(null);

  const fields: readonly FormField<LogoValues>[] = [
    {
      kind: "file",
      name: "light",
      label: t("systemSettings.logo.light"),
      required: true,
      accept: ACCEPT,
      limits: t("systemSettings.logo.limits"),
    },
    {
      kind: "file",
      name: "dark",
      label: t("systemSettings.logo.dark"),
      accept: ACCEPT,
      limits: t("systemSettings.logo.darkLimits"),
    },
  ];

  function close(opening: boolean) {
    if (opening || pending) return;
    onOpenChange(false);
    setRefusal(null);
    setError(null);
  }

  async function submit({ light, dark }: LogoValues) {
    if (light === null) return;
    setPending(true);
    setRefusal(null);
    setError(null);
    let sending: Pick<Refusal, "variant" | "file"> = { variant: "light", file: light };
    try {
      const first = await store(light, "light");
      let second: Stored | null = null;
      if (dark !== null) {
        sending = { variant: "dark", file: dark };
        second = await store(dark, "dark");
      }
      onUploaded({ light: uploaded(first), dark: second && uploaded(second) });
      toast(t("systemSettings.logo.uploaded"));
      onOpenChange(false);
    } catch (cause) {
      const field = refusalFor(cause, sending.variant, sending.file, t);
      if (field === null) setError(failureMessage(cause, t("api.failed")));
      else setRefusal(field);
    } finally {
      setPending(false);
    }
  }

  return (
    <FormDialog<LogoValues>
      open={open}
      onOpenChange={close}
      title={t("systemSettings.logo.title")}
      icon={ImageIcon}
      initial={INITIAL}
      fields={fields}
      validate={(values) => {
        const problems: Partial<Record<LogoVariant, string>> = {};
        for (const variant of VARIANTS) {
          const file = values[variant];
          if (file === null) continue;
          const problem = logoProblem(file);
          if (problem !== null) problems[variant] = problemText(problem, t);
          else if (refusal?.variant === variant && refusal.file === file)
            problems[variant] = refusal.message;
        }
        return Object.keys(problems).length === 0 ? null : problems;
      }}
      submitLabel={t("systemSettings.logo.save")}
      pending={pending}
      error={error}
      returnFocus={returnFocus}
      onSubmit={(values) => void submit(values)}
    />
  );
}
