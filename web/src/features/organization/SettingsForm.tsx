import { useCallback, useEffect, useRef, useState } from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useQueryClient } from "@tanstack/react-query";
import {
  FormProvider,
  useForm,
  useWatch,
  type FieldErrors,
  type FieldPath,
} from "react-hook-form";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { DirtyBar } from "@/components/shared/DirtyBar";
import { SettingsLayout } from "@/components/shared/SettingsLayout";
import { toast } from "@/components/ui/sonner";
import {
  reloadOrganizationSettings,
  retentionImpact,
  type RetentionImpact,
  useSaveOrganizationSettings,
  type OrganizationSettings,
} from "@/features/organization/api";
import { controlId } from "@/features/organization/controls";
import { editsSince } from "@/features/organization/editsSince";
import { RetentionConfirm } from "@/features/organization/components/RetentionConfirm";
import { retentionShortened } from "@/features/organization/retention";
import { DataPrivacySection } from "@/features/organization/sections/DataPrivacy";
import { OrganizationSection } from "@/features/organization/sections/Organization";
import { PlanUsageSection } from "@/features/organization/sections/PlanUsage";
import { SignInSection } from "@/features/organization/sections/SignIn";
import {
  formValues,
  settingsFormSchema,
  settingsInput,
  type SettingsFormOutput,
  type SettingsFormValues,
} from "@/features/organization/settingsFormSchema";
import {
  useSignInVerdict,
  type SignInRefusal,
} from "@/features/organization/signInVerdict";
import {
  SYSTEM_SECTIONS,
  sectionOfField,
} from "@/features/organization/systemSections";
import { useUnsavedSection } from "@/features/settings/unsavedSection";
import { ApiError, failureMessage } from "@/lib/api/errors";

const SERVER_FIELD =
  /^(organization\.(name|shortName|website|contactEmail|frontDeskPhone|frontDeskHours|address|logo\.(light|dark))|signIn\.allowedGoogleDomains|retention\.[A-Za-z]+)$/;

function firstPath(errors: object, prefix = ""): string | null {
  for (const [key, value] of Object.entries(errors)) {
    if (value === null || typeof value !== "object") continue;
    const path = prefix === "" ? key : `${prefix}.${key}`;
    if ("type" in value) return path;
    const inner = firstPath(value, path);
    if (inner !== null) return inner;
  }
  return null;
}

function controlOf(path: string): HTMLElement | null {
  const parts = path.split(".");
  while (parts.length > 0) {
    const found = document.getElementById(controlId(parts.join(".")));
    if (found !== null) return found;
    parts.pop();
  }
  return null;
}

/**
 * SettingsForm is System settings once the document has been read: one form
 * for every section, one DirtyBar for all of them, and the sections kept
 * mounted so that an edit survives a visit to another. Save changes sends the
 * whole document under the revision the form was last reset from, never the
 * newest the page has heard of, so a save made elsewhere is a
 * `STALE_WRITE` and not overwritten; the bar then offers to reload. A
 * sign-in policy that would lock people out pauses Save changes and says
 * why. A rule the server refuses is shown on its field, and the section that
 * holds the first such field is opened. Once saved, the front door reads the
 * centre again, and whatever was typed while the save was in flight stays in
 * the form as an unsaved change.
 */
export function SettingsForm({
  saved,
  active,
}: Readonly<{ saved: OrganizationSettings; active: string }>) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const client = useQueryClient();
  const save = useSaveOrganizationSettings();
  const [error, setError] = useState<string | null>(null);
  const [stale, setStale] = useState(false);
  const [refusal, setRefusal] = useState<SignInRefusal | null>(null);
  const revision = useRef(saved.revision);
  const persisting = useRef(false);
  const lastEdited = useRef("organization.name");
  const pendingFocus = useRef<string | null>(null);

  const form = useForm<SettingsFormValues, unknown, SettingsFormOutput>({
    resolver: zodResolver(settingsFormSchema),
    defaultValues: formValues(saved),
    mode: "onTouched",
  });
  const { reset, subscribe, formState, control } = form;
  const dirty = formState.isDirty;
  const [checking, setChecking] = useState(false);
  const [confirming, setConfirming] = useState<{
    values: SettingsFormOutput;
    sent: unknown;
    impact: RetentionImpact;
  } | null>(null);
  const saving = save.isPending || checking;
  useUnsavedSection("system-settings", dirty);

  const policy = useWatch({ control, name: "signIn" });
  const verdict = useSignInVerdict(saved.signIn, policy, refusal);

  const adopt = useCallback(
    (stored: OrganizationSettings) => {
      revision.current = stored.revision;
      reset(formValues(stored));
    },
    [reset],
  );

  useEffect(() => {
    if (!dirty) adopt(saved);
  }, [dirty, saved, adopt]);

  useEffect(
    () =>
      subscribe({
        formState: { values: true },
        callback: ({ name }) => {
          if (name !== undefined) lastEdited.current = name;
        },
      }),
    [subscribe],
  );

  useEffect(() => {
    const path = pendingFocus.current;
    if (path === null) return;
    pendingFocus.current = null;
    controlOf(path)?.focus();
  }, [active]);

  function showAt(path: string) {
    const section = sectionOfField(path);
    if (section !== undefined && section.id !== active) {
      pendingFocus.current = path;
      void navigate(section.to);
    } else controlOf(path)?.focus();
  }

  function refused(cause: ApiError) {
    const shown: string[] = [];
    for (const [path, message] of Object.entries(cause.details ?? {})) {
      const field = path.replace(/^(signIn\.allowedGoogleDomains)\.\d+$/, "$1");
      if (typeof message !== "string" || !SERVER_FIELD.test(field)) continue;
      form.setError(field as FieldPath<SettingsFormValues>, {
        type: "server",
        message,
      });
      shown.push(field);
    }
    setError(cause.message);
    if (shown[0] !== undefined) showAt(shown[0]);
  }

  function failed(cause: unknown, tried: SettingsFormOutput) {
    if (!(cause instanceof ApiError)) {
      setError(failureMessage(cause, t("api.failed")));
    } else if (cause.code === "STALE_WRITE") {
      setStale(true);
      setError(cause.message);
    } else if (cause.code === "SIGN_IN_POLICY_LOCKOUT") {
      setRefusal({ policy: tried.signIn, message: cause.message });
    } else if (cause.code === "VALIDATION_FAILED") {
      refused(cause);
    } else setError(cause.message);
  }

  async function onValid(values: SettingsFormOutput) {
    setError(null);
    setStale(false);
    setRefusal(null);
    const sent: unknown = JSON.parse(JSON.stringify(form.getValues()));
    if (!retentionShortened(saved.retention, values.retention)) {
      await persist(values, sent);
      return;
    }
    setChecking(true);
    try {
      const impact = await retentionImpact(values.retention);
      setConfirming({ values, sent, impact });
    } catch (cause) {
      setError(failureMessage(cause, t("api.failed")));
    } finally {
      setChecking(false);
    }
  }

  async function persist(values: SettingsFormOutput, sent: unknown) {
    if (persisting.current) return;
    persisting.current = true;
    try {
      const stored = await save.mutateAsync(settingsInput(values, revision.current));
      const typedSince = editsSince(sent, form.getValues());
      adopt(stored);
      for (const { path, value } of typedSince) {
        form.setValue(path as FieldPath<SettingsFormValues>, value as never, {
          shouldDirty: true,
        });
      }
      toast(t("settings.saved"));
    } catch (cause) {
      failed(cause, values);
    } finally {
      persisting.current = false;
    }
  }

  function onInvalid(errors: FieldErrors<SettingsFormValues>) {
    const path = firstPath(errors);
    if (path !== null) showAt(path);
  }

  async function reload() {
    setError(null);
    setStale(false);
    try {
      adopt(await reloadOrganizationSettings(client));
      controlOf(lastEdited.current)?.focus();
    } catch (cause) {
      setError(failureMessage(cause, t("api.failed")));
    }
  }

  function sectionContent(id: string) {
    if (id === "sign-in") return <SignInSection verdict={verdict} saving={saving} />;
    if (id === "data") return <DataPrivacySection saving={saving} />;
    if (id === "plan") return <PlanUsageSection />;
    return <OrganizationSection />;
  }

  const sections = SYSTEM_SECTIONS.map(({ id, label, icon, to }) => ({
    id,
    label: t(label),
    icon,
    to,
    content: sectionContent(id),
  }));

  return (
    <FormProvider {...form}>
      <form
        noValidate
        className="contents"
        onSubmit={(event) => {
          if (event.target !== event.currentTarget) return;
          event.preventDefault();
          if (saving || verdict.reason !== null || !dirty) return;
          void form.handleSubmit(onValid, onInvalid)();
        }}
      >
        <SettingsLayout
          label={t("systemSettings.navigation")}
          active={active}
          sections={sections}
          footer={
            <DirtyBar
              dirty={dirty}
              saving={saving}
              blocked={verdict.reason}
              error={error}
              errorAction={
                stale
                  ? { label: t("systemSettings.reload"), onAction: () => void reload() }
                  : undefined
              }
              onDiscard={() => {
                setError(null);
                setStale(false);
                setRefusal(null);
                adopt(saved);
              }}
              returnFocus={() => controlOf(lastEdited.current)}
            />
          }
        />
      </form>
      <RetentionConfirm
        impact={confirming?.impact ?? null}
        pending={save.isPending}
        onCancel={() => {
          if (!save.isPending) setConfirming(null);
        }}
        onConfirm={() => {
          if (confirming === null) return;
          void persist(confirming.values, confirming.sent).then(() =>
            setConfirming(null),
          );
        }}
      />
    </FormProvider>
  );
}
