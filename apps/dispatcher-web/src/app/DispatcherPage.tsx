import type { GroupSummary } from "@ptt-fleet/shared-types";
import { useEffect, useMemo, useRef, useState } from "react";
import { fetchGroups, fetchUsers } from "../lib/api";
import { RealtimeClient } from "../lib/ws";
import { useAuthStore } from "../features/auth/authStore";
import { DispatcherMap } from "../features/map/DispatcherMap";
import { UserList } from "../features/users/UserList";
import { useRealtimeStore } from "../features/users/realtimeStore";

export function DispatcherPage() {
  const session = useAuthStore((state) => state.session)!;
  const clearSession = useAuthStore((state) => state.clearSession);
  const users = useRealtimeStore((state) => state.users);
  const presence = useRealtimeStore((state) => state.presence);
  const connectionStatus = useRealtimeStore((state) => state.connectionStatus);
  const setUsers = useRealtimeStore((state) => state.setUsers);
  const applyEvent = useRealtimeStore((state) => state.applyEvent);
  const setConnectionStatus = useRealtimeStore((state) => state.setConnectionStatus);
  const resetRealtime = useRealtimeStore((state) => state.reset);
  const [groups, setGroups] = useState<GroupSummary[]>([]);
  const [selectedGroup, setSelectedGroup] = useState("");
  const [loadError, setLoadError] = useState("");
  const realtimeRef = useRef<RealtimeClient | null>(null);

  useEffect(() => {
    let active = true;
    Promise.all([fetchUsers(session.accessToken), fetchGroups(session.accessToken)])
      .then(([userResponse, groupResponse]) => {
        if (!active) return;
        setUsers(userResponse.items);
        setGroups(groupResponse.items);
        setSelectedGroup(groupResponse.items[0]?.id ?? "");
      })
      .catch((error: unknown) => {
        if (active) {
          setLoadError(error instanceof Error ? error.message : "Gagal memuat data dispatcher.");
        }
      });

    const realtime = new RealtimeClient({
      accessToken: session.accessToken,
      onEvent: applyEvent,
      onStatus: setConnectionStatus,
    });
    realtimeRef.current = realtime;
    realtime.connect();

    return () => {
      active = false;
      realtime.disconnect();
      realtimeRef.current = null;
    };
  }, [applyEvent, session.accessToken, setConnectionStatus, setUsers]);

  useEffect(() => {
    if (!selectedGroup || connectionStatus !== "connected") {
      return;
    }
    realtimeRef.current?.send({
      type: "group.join",
      requestId: crypto.randomUUID(),
      timestamp: new Date().toISOString(),
      payload: { groupId: selectedGroup },
    });
  }, [connectionStatus, selectedGroup]);

  const onlineCount = useMemo(
    () => users.filter((user) => presence[user.id]?.status === "online").length,
    [presence, users],
  );

  function handleLogout() {
    realtimeRef.current?.disconnect();
    resetRealtime();
    clearSession();
  }

  return (
    <main className="min-h-screen bg-[#09110f] text-stone-100">
      <div className="grid min-h-screen grid-rows-[auto_1fr_auto]">
        <header className="z-20 flex flex-wrap items-center justify-between gap-4 border-b border-white/8 bg-[#09110f]/95 px-4 py-3 backdrop-blur-xl sm:px-6">
          <div className="flex items-center gap-4">
            <div className="grid h-10 w-10 place-items-center rounded-xl bg-emerald-300 font-display text-lg font-black text-emerald-950">
              PF
            </div>
            <div>
              <div className="font-display text-lg leading-none">PTT Fleet</div>
              <div className="mt-1 text-[10px] font-semibold uppercase tracking-[0.2em] text-stone-500">
                Dispatcher console
              </div>
            </div>
          </div>

          <div className="order-3 flex w-full items-center gap-3 sm:order-none sm:w-auto">
            <span className="text-xs font-semibold uppercase tracking-wider text-stone-500">
              Active channel
            </span>
            <select
              aria-label="Active group"
              className="min-w-0 flex-1 rounded-xl border border-stone-700 bg-stone-900 px-3 py-2 text-sm text-stone-200 outline-none focus:border-emerald-400 sm:w-56"
              onChange={(event) => setSelectedGroup(event.target.value)}
              value={selectedGroup}
            >
              {groups.length === 0 ? <option value="">No groups</option> : null}
              {groups.map((group) => (
                <option key={group.id} value={group.id}>
                  {group.name}
                </option>
              ))}
            </select>
          </div>

          <div className="flex items-center gap-3">
            <ConnectionBadge status={connectionStatus} />
            <button
              className="hidden rounded-xl border border-white/10 px-3 py-2 text-left transition hover:bg-white/5 sm:block"
              onClick={handleLogout}
              type="button"
            >
              <span className="block text-xs font-semibold text-stone-200">{session.user.fullName}</span>
              <span className="mt-0.5 block text-[10px] uppercase tracking-wider text-stone-500">
                {session.user.role.replace("_", " ")} · logout
              </span>
            </button>
          </div>
        </header>

        <div className="grid min-h-0 lg:grid-cols-[320px_1fr]">
          <aside className="flex max-h-[44vh] flex-col border-b border-white/8 bg-[#101815] lg:max-h-none lg:border-b-0 lg:border-r">
            <div className="border-b border-white/8 px-4 py-4">
              <div className="flex items-end justify-between">
                <div>
                  <p className="text-[10px] font-semibold uppercase tracking-[0.2em] text-stone-500">
                    Field presence
                  </p>
                  <h2 className="font-display mt-1 text-xl">Team status</h2>
                </div>
                <div className="rounded-lg bg-emerald-300/10 px-2.5 py-1.5 text-xs font-bold text-emerald-300">
                  {onlineCount}/{users.length} online
                </div>
              </div>
              <div className="relative mt-4">
                <span className="absolute left-3 top-1/2 -translate-y-1/2 text-stone-600">⌕</span>
                <input
                  aria-label="Search users"
                  className="w-full rounded-xl border border-stone-700/80 bg-black/15 py-2.5 pl-9 pr-3 text-sm outline-none placeholder:text-stone-600 focus:border-stone-500"
                  placeholder="Search field unit"
                />
              </div>
            </div>

            {loadError ? (
              <div className="m-4 rounded-xl border border-amber-400/20 bg-amber-950/25 p-3 text-sm text-amber-200">
                {loadError}
              </div>
            ) : null}
            <div className="min-h-0 flex-1 overflow-y-auto p-2">
              <UserList presence={presence} users={users} />
            </div>
          </aside>

          <section className="min-h-[460px]">
            <DispatcherMap />
          </section>
        </div>

        <footer className="grid gap-3 border-t border-white/8 bg-[#0d1512] px-4 py-3 sm:grid-cols-[1fr_auto_1fr] sm:items-center sm:px-6">
          <div className="flex items-center gap-3 text-xs text-stone-500">
            <span className="rounded-md border border-white/8 px-2 py-1 font-mono text-[10px] text-stone-400">
              LIVE
            </span>
            <span>Waiting for operational events</span>
          </div>

          <button
            className="rounded-xl border border-amber-300/20 bg-amber-300/8 px-5 py-2.5 text-xs font-bold uppercase tracking-[0.18em] text-amber-200"
            disabled
            title="Browser PTT is enabled after Android-to-Android audio is stable"
            type="button"
          >
            PTT monitor standby
          </button>

          <div className="text-right text-[11px] uppercase tracking-wider text-stone-600">
            SOS alerts · no active events
          </div>
        </footer>
      </div>
    </main>
  );
}

function ConnectionBadge({ status }: { status: string }) {
  const connected = status === "connected";
  return (
    <div className="flex items-center gap-2 rounded-full border border-white/8 bg-white/[0.025] px-3 py-2">
      <span
        className={`h-2 w-2 rounded-full ${
          connected ? "bg-emerald-400 shadow-[0_0_12px_rgba(52,211,153,.8)]" : "bg-amber-400"
        }`}
      />
      <span className="text-[10px] font-bold uppercase tracking-[0.16em] text-stone-400">
        {status}
      </span>
    </div>
  );
}
