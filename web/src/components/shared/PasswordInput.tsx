import { useState, type ComponentProps } from "react";
import { Eye, EyeOff } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

/** PasswordInput preserves native password autofill and provides an explicit visibility toggle. */
export function PasswordInput({
  className,
  ...props
}: Readonly<Omit<ComponentProps<typeof Input>, "type">>) {
  const { t } = useTranslation();
  const [visible, setVisible] = useState(false);
  return (
    <div className="relative">
      <Input
        {...props}
        type={visible ? "text" : "password"}
        className={cn("h-11 pr-12", props.size === "xl" && "pr-11.5", className)}
      />
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className={cn(
          "absolute inset-y-0 right-0 h-11 w-11",
          props.size === "xl" &&
            "text-muted-fg hover:text-fg inset-y-auto top-1/2 right-1.5 size-9 -translate-y-1/2 [&_svg]:size-[17px]",
        )}
        aria-label={t(visible ? "common.hidePassword" : "common.showPassword")}
        aria-controls={props.id}
        aria-pressed={visible}
        disabled={props.disabled}
        onClick={() => setVisible((v) => !v)}
      >
        {visible ? <EyeOff aria-hidden="true" /> : <Eye aria-hidden="true" />}
      </Button>
    </div>
  );
}
