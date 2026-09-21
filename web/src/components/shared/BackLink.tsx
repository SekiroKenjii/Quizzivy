import type { ReactNode } from "react";
import { Link } from "react-router";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";

/**
 * S-14's way back on a wide screen: a text link above the title that names
 * where it goes, in place of the phone's back arrow in the bar.
 */
export function BackLink({
  to,
  children,
}: Readonly<{ to: string; children: ReactNode }>) {
  return (
    <Button
      asChild
      variant="ghost"
      size="xs"
      className="text-muted-foreground -ml-1 px-1"
    >
      <Link to={to}>
        <ArrowLeft aria-hidden="true" />
        {children}
      </Link>
    </Button>
  );
}
