import { Search } from "lucide-react";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

/** SearchInput is a list-toolbar search field with icon clearance at either density. */
export function SearchInput({
  value,
  onChange,
  placeholder,
  className,
  dense = false,
}: Readonly<{
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
  className?: string;
  dense?: boolean;
}>) {
  return (
    <div className={cn("relative w-72", className)}>
      <Search
        className={cn(
          "text-muted-foreground pointer-events-none absolute top-2.5 left-2.5 size-4",
          dense && "top-2 left-2 size-3.5",
        )}
        aria-hidden="true"
      />
      <Input
        type="search"
        className={cn(
          "pl-9 in-data-[scale=deck]:pl-9",
          dense &&
            "h-8 pl-8 text-xs in-data-[scale=deck]:h-8 in-data-[scale=deck]:pl-8 lg:text-xs",
        )}
        value={value}
        placeholder={placeholder}
        aria-label={placeholder}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
}
