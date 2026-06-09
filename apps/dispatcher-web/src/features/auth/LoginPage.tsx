import { useState, type FormEvent } from "react";
import { login } from "../../lib/api";
import { useAuthStore } from "./authStore";

export function LoginPage() {
  const setSession = useAuthStore((state) => state.setSession);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setSubmitting(true);

    try {
      const session = await login(username.trim(), password);
      if (session.user.role === "field_user") {
        throw new Error("Akun field user tidak memiliki akses dispatcher.");
      }
      setSession(session);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Login gagal.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="relative min-h-screen overflow-hidden bg-[#07110f] text-stone-100">
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_18%_20%,rgba(48,132,100,0.18),transparent_32%),radial-gradient(circle_at_82%_82%,rgba(205,141,66,0.13),transparent_30%)]" />
      <div className="absolute inset-0 opacity-[0.04] [background-image:linear-gradient(rgba(255,255,255,.8)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,.8)_1px,transparent_1px)] [background-size:42px_42px]" />

      <div className="relative mx-auto grid min-h-screen max-w-7xl items-center gap-14 px-6 py-12 lg:grid-cols-[1.15fr_.85fr] lg:px-12">
        <section className="max-w-2xl">
          <div className="mb-8 flex items-center gap-3 text-xs font-semibold uppercase tracking-[0.28em] text-emerald-300">
            <span className="h-px w-12 bg-emerald-400/70" />
            Fleet operations network
          </div>
          <h1 className="font-display text-5xl leading-[0.96] tracking-[-0.045em] text-stone-50 sm:text-7xl">
            Field teams,
            <span className="block text-emerald-300">one clear channel.</span>
          </h1>
          <p className="mt-8 max-w-xl text-base leading-7 text-stone-400 sm:text-lg">
            Monitor patrol presence, live positions, and urgent field events from one focused
            dispatcher console.
          </p>

          <div className="mt-12 grid max-w-xl grid-cols-3 gap-3">
            {[
              ["PTT", "Group voice"],
              ["GPS", "Live tracking"],
              ["SOS", "Priority alert"],
            ].map(([label, value]) => (
              <div key={label} className="border-l border-stone-700/80 pl-4">
                <div className="font-display text-2xl text-stone-100">{label}</div>
                <div className="mt-1 text-xs uppercase tracking-wider text-stone-500">{value}</div>
              </div>
            ))}
          </div>
        </section>

        <section className="rounded-[2rem] border border-white/10 bg-stone-950/55 p-7 shadow-2xl shadow-black/30 backdrop-blur-xl sm:p-9">
          <div className="mb-8">
            <p className="text-xs font-semibold uppercase tracking-[0.24em] text-amber-300">
              Secure operator access
            </p>
            <h2 className="font-display mt-3 text-3xl text-stone-50">Open dispatcher</h2>
            <p className="mt-2 text-sm leading-6 text-stone-400">
              Gunakan akun super admin, dispatcher, atau supervisor.
            </p>
          </div>

          <form className="space-y-5" onSubmit={handleSubmit}>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold uppercase tracking-wider text-stone-400">
                Username
              </span>
              <input
                autoComplete="username"
                className="w-full rounded-xl border border-stone-700 bg-black/25 px-4 py-3.5 text-stone-100 outline-none transition placeholder:text-stone-600 focus:border-emerald-400 focus:ring-2 focus:ring-emerald-400/15"
                onChange={(event) => setUsername(event.target.value)}
                placeholder="dispatcher1"
                required
                value={username}
              />
            </label>

            <label className="block">
              <span className="mb-2 block text-xs font-semibold uppercase tracking-wider text-stone-400">
                Password
              </span>
              <input
                autoComplete="current-password"
                className="w-full rounded-xl border border-stone-700 bg-black/25 px-4 py-3.5 text-stone-100 outline-none transition placeholder:text-stone-600 focus:border-emerald-400 focus:ring-2 focus:ring-emerald-400/15"
                onChange={(event) => setPassword(event.target.value)}
                placeholder="••••••••"
                required
                type="password"
                value={password}
              />
            </label>

            {error ? (
              <div role="alert" className="rounded-xl border border-red-400/25 bg-red-950/35 px-4 py-3 text-sm text-red-200">
                {error}
              </div>
            ) : null}

            <button
              className="mt-2 flex w-full items-center justify-center gap-3 rounded-xl bg-emerald-300 px-4 py-3.5 text-sm font-bold text-emerald-950 transition hover:bg-emerald-200 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={submitting}
              type="submit"
            >
              <span>{submitting ? "Menghubungkan..." : "Masuk ke console"}</span>
              <span aria-hidden="true">→</span>
            </button>
          </form>

          <div className="mt-7 flex items-center justify-between border-t border-white/8 pt-5 text-xs text-stone-500">
            <span>JWT secured session</span>
            <span className="flex items-center gap-2">
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-400" />
              API localhost:8080
            </span>
          </div>
        </section>
      </div>
    </main>
  );
}
