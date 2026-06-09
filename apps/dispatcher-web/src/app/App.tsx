import { lazy, Suspense, useState } from "react";
import { LoginPage } from "../features/auth/LoginPage";
import { useAuthStore } from "../features/auth/authStore";

const DispatcherPage = lazy(async () => {
  const module = await import("./DispatcherPage");
  return { default: module.DispatcherPage };
});

const AdminPage = lazy(async () => {
  const module = await import("./AdminPage");
  return { default: module.AdminPage };
});

export function App() {
  const session = useAuthStore((state) => state.session);
  const [view, setView] = useState<"dispatcher" | "admin">("dispatcher");
  return session ? (
    <Suspense fallback={<DashboardLoading />}>
      {view === "admin" ? (
        <AdminPage onBack={() => setView("dispatcher")} />
      ) : (
        <DispatcherPage onOpenAdmin={() => setView("admin")} />
      )}
    </Suspense>
  ) : (
    <LoginPage />
  );
}

function DashboardLoading() {
  return (
    <main className="grid min-h-screen place-items-center bg-[#09110f] text-stone-300">
      <div className="flex items-center gap-3 text-xs font-semibold uppercase tracking-[0.2em]">
        <span className="h-2.5 w-2.5 animate-pulse rounded-full bg-emerald-400" />
        Loading workspace
      </div>
    </main>
  );
}
