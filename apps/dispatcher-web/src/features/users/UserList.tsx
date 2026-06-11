import type { UserSummary } from "@ptt-fleet/shared-types";
import type { PresenceEntry } from "./realtimeStore";

interface UserListProps {
  users: UserSummary[];
  presence: Record<string, PresenceEntry>;
  selectedUserId?: string;
  onSelectUser?: (userId: string) => void;
}

export function UserList({
  users,
  presence,
  selectedUserId,
  onSelectUser,
}: UserListProps) {
  const sortedUsers = [...users].sort((left, right) => {
    const leftOnline = presence[left.id]?.status === "online" ? 1 : 0;
    const rightOnline = presence[right.id]?.status === "online" ? 1 : 0;
    return rightOnline - leftOnline || left.fullName.localeCompare(right.fullName);
  });

  return (
    <div className="space-y-1.5">
      {sortedUsers.length === 0 ? (
        <div className="rounded-xl border border-dashed border-stone-700 px-4 py-8 text-center text-sm text-stone-500">
          Belum ada user yang dapat ditampilkan.
        </div>
      ) : (
        sortedUsers.map((user) => {
          const online = presence[user.id]?.status === "online";
          return (
            <article
              key={user.id}
              className={`group flex items-center gap-3 rounded-xl border px-3 py-2.5 transition ${
                selectedUserId === user.id
                  ? "border-amber-300/40 bg-amber-300/8"
                  : "border-transparent hover:border-white/8 hover:bg-white/[0.035]"
              }`}
            >
              <div className="relative grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-stone-800 text-xs font-bold text-stone-300">
                {initials(user.fullName)}
                <span
                  className={`absolute -bottom-1 -right-1 h-3 w-3 rounded-full border-2 border-[#101815] ${
                    online ? "bg-emerald-400" : "bg-stone-600"
                  }`}
                />
              </div>
              <div className="min-w-0 flex-1">
                <div className="truncate text-sm font-semibold text-stone-200">{user.fullName}</div>
                <div className="mt-0.5 flex items-center gap-2 text-[11px] uppercase tracking-wider text-stone-500">
                  <span>{roleLabel(user.role)}</span>
                  <span>·</span>
                  <span className={online ? "text-emerald-400" : ""}>
                    {online ? "Online" : "Offline"}
                  </span>
                </div>
              </div>
              <button
                aria-label={`Talk directly to ${user.fullName}`}
                className="rounded-lg px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-stone-500 transition group-hover:bg-stone-800 group-hover:text-stone-200 disabled:opacity-30"
                disabled={!onSelectUser}
                onClick={() => onSelectUser?.(user.id)}
                type="button"
              >
                Direct
              </button>
            </article>
          );
        })
      )}
    </div>
  );
}

function initials(name: string): string {
  return name
    .split(" ")
    .slice(0, 2)
    .map((part) => part[0])
    .join("")
    .toUpperCase();
}

function roleLabel(role: UserSummary["role"]): string {
  return role.replace("_", " ");
}
