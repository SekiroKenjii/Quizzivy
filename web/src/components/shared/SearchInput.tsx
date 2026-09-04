import { Search } from "lucide-react";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

/** The list search box every admin list draws at the toolbar's right edge. */
export function SearchInput({
  value,
  onChange,
  placeholder,
  className,
}: {
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
  className?: string;
}) {
  return (
    <div className={cn("relative w-72", className)}>
      <Search
        className="text-muted-foreground pointer-events-none absolute top-2.5 left-2.5 size-4"
        aria-hidden="true"
      />
      <Input
        type="search"
        className="pl-9"
        value={value}
        placeholder={placeholder}
        aria-label={placeholder}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
}
