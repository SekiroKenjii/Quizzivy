import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { ImageIcon, Trash2 } from "lucide-react";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
import { initials } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/sonner";
import { replaceAccountUser } from "@/features/auth/accountPreferences";
import type { User } from "@/features/auth/api";
import {
  AVATAR_MAX_BYTES,
  AVATAR_TYPES,
  deleteAvatar,
  setAvatar,
} from "@/features/settings/api";
import { failureMessage } from "@/lib/api/errors";
import { useAuthStore } from "@/stores/auth";

type PhotoValues = { file: File | null };

const INITIAL: PhotoValues = { file: null };
const ACCEPT = AVATAR_TYPES.join(",");
const ACCEPTED = new Set<string>(AVATAR_TYPES);

function photoProblem(file: File, t: TFunction): string | null {
  if (!ACCEPTED.has(file.type)) return t("settings.photo.wrongType");
  if (file.size > AVATAR_MAX_BYTES) return t("settings.photo.tooLarge");
  return null;
}

/**
 * ProfilePhoto is the photo row of the Profile card: the account's photo, or
 * its initials, with Change photo and, while there is a photo, Remove. Both
 * save at once, outside the form's unsaved changes (DG-158). A file of the
 * wrong type or over 2 MiB is refused before it is sent; the server checks
 * again and its message is shown as written.
 */
export function ProfilePhoto() {
  const { t } = useTranslation();
  const fullName = useAuthStore((s) => s.user?.fullName ?? "");
  const avatarUrl = useAuthStore((s) => s.user?.avatarUrl);
  const [mode, setMode] = useState<"change" | "remove" | null>(null);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const changeButton = useRef<HTMLButtonElement>(null);

  const fields: readonly FormField<PhotoValues>[] = [
    {
      kind: "file",
      name: "file",
      required: true,
      accept: ACCEPT,
      limits: t("settings.photo.limits"),
    },
  ];

  function close(open: boolean) {
    if (open || pending) return;
    setMode(null);
    setError(null);
  }

  function run(work: () => Promise<User>, done: string) {
    setPending(true);
    setError(null);
    replaceAccountUser(work).then(
      () => {
        setPending(false);
        setMode(null);
        toast(t(done));
      },
      (cause: unknown) => {
        setPending(false);
        setError(failureMessage(cause, t("api.failed")));
      },
    );
  }

  return (
    <div className="flex flex-wrap items-center gap-3.5">
      {avatarUrl === undefined ? (
        <span
          aria-hidden="true"
          className="bg-brand-soft text-brand-ink grid size-14 flex-none place-items-center rounded-[14px] text-lg font-semibold"
        >
          {initials(fullName)}
        </span>
      ) : (
        <img
          src={avatarUrl}
          alt={t("settings.photo.alt")}
          className="size-14 flex-none rounded-[14px] object-cover"
        />
      )}
      <Button
        ref={changeButton}
        type="button"
        variant="outline"
        className="h-8.5"
        onClick={() => setMode("change")}
      >
        {t("settings.photo.change")}
      </Button>
      {avatarUrl === undefined ? null : (
        <Button
          type="button"
          variant="ghost"
          className="text-muted-fg hover:text-fg h-8.5 px-2.5 text-sm"
          onClick={() => setMode("remove")}
        >
          {t("settings.photo.remove")}
        </Button>
      )}
      <FormDialog<PhotoValues>
        open={mode === "change"}
        onOpenChange={close}
        title={t("settings.photo.changeTitle")}
        icon={ImageIcon}
        initial={INITIAL}
        fields={fields}
        validate={(values) => {
          if (values.file === null) return null;
          const problem = photoProblem(values.file, t);
          return problem === null ? null : { file: problem };
        }}
        submitLabel={t("settings.photo.save")}
        pending={pending}
        error={mode === "change" ? error : null}
        returnFocus={changeButton}
        onSubmit={(values) => {
          const file = values.file;
          if (file === null) return;
          run(() => setAvatar(file), "settings.photo.updated");
        }}
      />
      <ConfirmDialog
        open={mode === "remove"}
        onOpenChange={close}
        title={t("settings.photo.removeTitle")}
        description={t("settings.photo.removeBody")}
        confirmLabel={t("settings.photo.removeConfirm")}
        icon={Trash2}
        destructive
        pending={pending}
        error={mode === "remove" ? error : null}
        returnFocus={changeButton}
        onConfirm={() => run(deleteAvatar, "settings.photo.removed")}
      />
    </div>
  );
}
