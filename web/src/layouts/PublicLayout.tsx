import { Outlet } from "react-router";
import { BrandLockup } from "@/components/shared/Brand";

/** §9: "logo + content, nothing else." */
export default function PublicLayout() {
  return (
    <div className="flex min-h-svh flex-col">
      <header className="flex items-center justify-center border-b px-4 py-3">
        <BrandLockup height={35} />
      </header>
      <main className="flex-1 p-4 pt-10">
        <div className="mx-auto w-full max-w-sm">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
