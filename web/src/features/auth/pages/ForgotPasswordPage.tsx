import { ArrowLeft, KeyRound, Phone } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { Button } from "@/components/ui/button";
import { AuthLayout } from "@/features/auth/AuthLayout";
import { GoogleMark } from "@/features/auth/components/GoogleMark";
import { config } from "@/lib/config";

/**
 * ForgotPasswordPage explains that a password is reset by a teacher or the
 * centre, never by the account holder: there is no self-service reset. It
 * sends nothing.
 */
export default function ForgotPasswordPage() {
  const { t } = useTranslation();
  const { phone, hours } = config.frontDesk;
  const frontDesk = [phone, hours].filter(Boolean).join(" · ");

  return (
    <AuthLayout>
      <div className="flex flex-col gap-[18px]">
        <span className="bg-muted grid size-11 place-items-center rounded-xl">
          <KeyRound aria-hidden="true" className="size-5" />
        </span>
        <div>
          <h1 className="text-h1">{t("forgot.title")}</h1>
          <p className="text-muted-fg text-body mt-1.5 text-pretty">
            {t("forgot.body")}
          </p>
        </div>
        <ul className="text-ui flex flex-col overflow-hidden rounded-xl border">
          <li className="flex gap-3 px-3.5 py-3">
            <GoogleMark className="mt-0.5 size-[17px] shrink-0" />
            <span>
              <b className="font-semibold">{t("forgot.googleTitle")}</b>{" "}
              {t("forgot.googleBody")}
            </span>
          </li>
          {frontDesk && (
            <li className="flex gap-3 border-t px-3.5 py-3">
              <Phone aria-hidden="true" className="mt-0.5 size-[17px] shrink-0" />
              <span>
                <b className="font-semibold">{t("forgot.frontDesk")}</b> · {frontDesk}
              </span>
            </li>
          )}
        </ul>
        <Button
          asChild
          variant="outline"
          size="xl"
          className="bg-card hover:bg-muted w-full font-medium"
        >
          <Link to="/login">
            <ArrowLeft aria-hidden="true" className="size-[17px]" />
            {t("login.backToSignIn")}
          </Link>
        </Button>
      </div>
    </AuthLayout>
  );
}
