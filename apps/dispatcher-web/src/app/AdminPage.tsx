import type { GroupSummary, UserRole, UserSummary } from "@ptt-fleet/shared-types";
import { useEffect, useMemo, useState } from "react";
import { PasswordDialog } from "../components/PasswordDialog";
import { useAuthStore } from "../features/auth/authStore";
import {
  addGroupMember,
  changePassword,
  createGroup,
  createUser,
  fetchAuditLogs,
  fetchDevices,
  fetchGroup,
  fetchGroups,
  fetchUsers,
  removeGroupMember,
  resetUserPassword,
  updateUser,
  type AuditLog,
  type DeviceSummary,
  type GroupDetail,
} from "../lib/api";

type Tab = "users" | "groups" | "devices" | "audit";

export function AdminPage({ onBack }: { onBack: () => void }) {
  const session = useAuthStore((state) => state.session)!;
  const clearSession = useAuthStore((state) => state.clearSession);
  const [tab, setTab] = useState<Tab>("users");
  const [users, setUsers] = useState<UserSummary[]>([]);
  const [groups, setGroups] = useState<GroupSummary[]>([]);
  const [devices, setDevices] = useState<DeviceSummary[]>([]);
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);
  const [selectedGroup, setSelectedGroup] = useState<GroupDetail | null>(null);
  const [message, setMessage] = useState("");
  const [changePasswordOpen, setChangePasswordOpen] = useState(false);
  const [passwordResetUser, setPasswordResetUser] = useState<UserSummary | null>(null);

  async function refresh() {
    setMessage("");
    try {
      const [userResult, groupResult, deviceResult, auditResult] = await Promise.all([
        fetchUsers(session.accessToken),
        fetchGroups(session.accessToken),
        fetchDevices(session.accessToken),
        fetchAuditLogs(session.accessToken),
      ]);
      setUsers(userResult.items);
      setGroups(groupResult.items);
      setDevices(deviceResult.items);
      setAuditLogs(auditResult.items);
      if (selectedGroup) {
        setSelectedGroup(await fetchGroup(session.accessToken, selectedGroup.id));
      }
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Unable to load admin data.");
    }
  }

  useEffect(() => {
    let active = true;
    Promise.all([
      fetchUsers(session.accessToken),
      fetchGroups(session.accessToken),
      fetchDevices(session.accessToken),
      fetchAuditLogs(session.accessToken),
    ])
      .then(([userResult, groupResult, deviceResult, auditResult]) => {
        if (!active) return;
        setUsers(userResult.items);
        setGroups(groupResult.items);
        setDevices(deviceResult.items);
        setAuditLogs(auditResult.items);
      })
      .catch((error: unknown) => {
        if (active) {
          setMessage(error instanceof Error ? error.message : "Unable to load admin data.");
        }
      });
    return () => {
      active = false;
    };
  }, [session.accessToken]);

  function logout() {
    clearSession();
  }

  return (
    <main className="min-h-screen bg-[#09110f] text-stone-100">
      <header className="flex flex-wrap items-center justify-between gap-4 border-b border-white/8 px-5 py-4">
        <div>
          <p className="text-[10px] font-bold uppercase tracking-[0.2em] text-emerald-300">
            Administration
          </p>
          <h1 className="font-display mt-1 text-2xl">Fleet control center</h1>
        </div>
        <div className="flex gap-2">
          <button className="admin-button" onClick={onBack} type="button">
            Live dispatcher
          </button>
          <button className="admin-button" onClick={() => setChangePasswordOpen(true)} type="button">
            Change password
          </button>
          <button className="admin-button" onClick={logout} type="button">
            Logout
          </button>
        </div>
      </header>

      <nav className="flex gap-2 overflow-x-auto border-b border-white/8 px-5 py-3">
        {(["users", "groups", "devices", "audit"] as Tab[]).map((item) => (
          <button
            className={`rounded-xl px-4 py-2 text-xs font-bold uppercase tracking-wider ${
              tab === item ? "bg-emerald-300 text-emerald-950" : "bg-white/5 text-stone-400"
            }`}
            key={item}
            onClick={() => setTab(item)}
            type="button"
          >
            {item}
          </button>
        ))}
      </nav>

      {message ? (
        <div className="mx-5 mt-4 rounded-xl border border-amber-400/20 bg-amber-950/30 p-3 text-sm text-amber-200">
          {message}
        </div>
      ) : null}

      <section className="p-5">
        {tab === "users" ? (
          <UsersAdmin
            currentRole={session.user.role}
            currentUserId={session.user.id}
            onChanged={refresh}
            onResetPassword={setPasswordResetUser}
            token={session.accessToken}
            users={users}
          />
        ) : null}
        {tab === "groups" ? (
          <GroupsAdmin
            currentRole={session.user.role}
            groups={groups}
            onChanged={refresh}
            onSelect={async (groupId) =>
              setSelectedGroup(await fetchGroup(session.accessToken, groupId))
            }
            selectedGroup={selectedGroup}
            token={session.accessToken}
            users={users}
          />
        ) : null}
        {tab === "devices" ? <DevicesAdmin devices={devices} /> : null}
        {tab === "audit" ? <AuditAdmin logs={auditLogs} /> : null}
      </section>
      <PasswordDialog
        description="Password baru akan mencabut seluruh sesi. Anda perlu login kembali setelah berhasil."
        onClose={() => setChangePasswordOpen(false)}
        onSubmit={async (currentPassword, newPassword) => {
          await changePassword(session.accessToken, currentPassword, newPassword);
          clearSession();
        }}
        open={changePasswordOpen}
        requireCurrentPassword
        submitLabel="Change password"
        title="Change your password"
      />
      <PasswordDialog
        description={
          passwordResetUser
            ? `Tetapkan password baru untuk ${passwordResetUser.fullName}. Semua sesi user tersebut akan dicabut.`
            : ""
        }
        onClose={() => setPasswordResetUser(null)}
        onSubmit={async (_, newPassword) => {
          if (!passwordResetUser) return;
          await resetUserPassword(session.accessToken, passwordResetUser.id, newPassword);
          const resetOwnPassword = passwordResetUser.id === session.user.id;
          setPasswordResetUser(null);
          if (resetOwnPassword) {
            clearSession();
          } else {
            await refresh();
            setMessage(`Password ${passwordResetUser.fullName} berhasil direset.`);
          }
        }}
        open={passwordResetUser !== null}
        submitLabel="Reset password"
        title="Reset user password"
      />
    </main>
  );
}

function UsersAdmin({
  currentRole,
  currentUserId,
  onChanged,
  onResetPassword,
  token,
  users,
}: {
  currentRole: UserRole;
  currentUserId: string;
  onChanged: () => Promise<void>;
  onResetPassword: (user: UserSummary) => void;
  token: string;
  users: UserSummary[];
}) {
  const [form, setForm] = useState({
    username: "",
    password: "",
    fullName: "",
    role: "field_user",
  });
  const canManage = currentRole === "super_admin" || currentRole === "dispatcher";

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    await createUser(token, form);
    setForm({ username: "", password: "", fullName: "", role: "field_user" });
    await onChanged();
  }

  return (
    <div className="grid gap-5 xl:grid-cols-[360px_1fr]">
      <div className="admin-card">
        <h2 className="font-display text-xl">Create user</h2>
        {canManage ? (
          <form className="mt-4 grid gap-3" onSubmit={submit}>
            <input
              className="admin-input"
              onChange={(event) => setForm({ ...form, username: event.target.value })}
              placeholder="Username"
              required
              value={form.username}
            />
            <input
              className="admin-input"
              onChange={(event) => setForm({ ...form, fullName: event.target.value })}
              placeholder="Full name"
              required
              value={form.fullName}
            />
            <input
              className="admin-input"
              minLength={8}
              onChange={(event) => setForm({ ...form, password: event.target.value })}
              placeholder="Temporary password"
              required
              type="password"
              value={form.password}
            />
            {currentRole === "super_admin" ? (
              <select
                className="admin-input"
                onChange={(event) => setForm({ ...form, role: event.target.value })}
                value={form.role}
              >
                <option value="field_user">Field user</option>
                <option value="dispatcher">Dispatcher</option>
                <option value="supervisor">Supervisor</option>
                <option value="super_admin">Super admin</option>
              </select>
            ) : null}
            <button className="admin-primary" type="submit">
              Create user
            </button>
          </form>
        ) : (
          <p className="mt-3 text-sm text-stone-500">Supervisor access is read-only.</p>
        )}
      </div>

      <div className="admin-card overflow-x-auto">
        <h2 className="font-display mb-4 text-xl">Users</h2>
        <table className="admin-table">
          <thead>
            <tr>
              <th>User</th>
              <th>Role</th>
              <th>Status</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {users.map((user) => {
              const editable =
                currentRole === "super_admin" ||
                (currentRole === "dispatcher" && user.role === "field_user");
              return (
                <tr key={user.id}>
                  <td>
                    <strong>{user.fullName}</strong>
                    <span>@{user.username}</span>
                  </td>
                  <td>{user.role.replace("_", " ")}</td>
                  <td>{user.status}</td>
                  <td className="text-right">
                    <div className="flex justify-end gap-2">
                      {currentRole === "super_admin" ? (
                        <button
                          className="admin-button"
                          onClick={() => onResetPassword(user)}
                          type="button"
                        >
                          Reset password{user.id === currentUserId ? " (self)" : ""}
                        </button>
                      ) : null}
                      {editable ? (
                        <button
                          className="admin-button"
                          onClick={async () => {
                            await updateUser(token, user.id, {
                              status: user.status === "active" ? "disabled" : "active",
                            });
                            await onChanged();
                          }}
                          type="button"
                        >
                          {user.status === "active" ? "Disable" : "Enable"}
                        </button>
                      ) : null}
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function GroupsAdmin({
  currentRole,
  groups,
  onChanged,
  onSelect,
  selectedGroup,
  token,
  users,
}: {
  currentRole: UserRole;
  groups: GroupSummary[];
  onChanged: () => Promise<void>;
  onSelect: (groupId: string) => Promise<void>;
  selectedGroup: GroupDetail | null;
  token: string;
  users: UserSummary[];
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [memberId, setMemberId] = useState("");
  const canManage = currentRole === "super_admin";
  const availableUsers = useMemo(
    () => users.filter((user) => !selectedGroup?.members.some((member) => member.userId === user.id)),
    [selectedGroup, users],
  );

  return (
    <div className="grid gap-5 xl:grid-cols-[340px_1fr]">
      <div className="admin-card">
        <h2 className="font-display text-xl">Groups</h2>
        <div className="mt-4 grid gap-2">
          {groups.map((group) => (
            <button
              className="rounded-xl border border-white/8 bg-black/15 px-4 py-3 text-left hover:border-emerald-300/40"
              key={group.id}
              onClick={() => void onSelect(group.id)}
              type="button"
            >
              <strong className="block">{group.name}</strong>
              <span className="mt-1 block text-xs text-stone-500">{group.description}</span>
            </button>
          ))}
        </div>
        {canManage ? (
          <form
            className="mt-5 grid gap-3 border-t border-white/8 pt-5"
            onSubmit={async (event) => {
              event.preventDefault();
              await createGroup(token, { name, description });
              setName("");
              setDescription("");
              await onChanged();
            }}
          >
            <input className="admin-input" onChange={(event) => setName(event.target.value)} placeholder="Group name" required value={name} />
            <input className="admin-input" onChange={(event) => setDescription(event.target.value)} placeholder="Description" value={description} />
            <button className="admin-primary" type="submit">Create group</button>
          </form>
        ) : null}
      </div>

      <div className="admin-card">
        <h2 className="font-display text-xl">{selectedGroup?.name ?? "Select a group"}</h2>
        {selectedGroup ? (
          <>
            {canManage ? (
              <div className="mt-4 flex gap-2">
                <select className="admin-input flex-1" onChange={(event) => setMemberId(event.target.value)} value={memberId}>
                  <option value="">Choose user</option>
                  {availableUsers.map((user) => <option key={user.id} value={user.id}>{user.fullName}</option>)}
                </select>
                <button
                  className="admin-primary"
                  disabled={!memberId}
                  onClick={async () => {
                    await addGroupMember(token, selectedGroup.id, memberId);
                    setMemberId("");
                    await onChanged();
                    await onSelect(selectedGroup.id);
                  }}
                  type="button"
                >
                  Assign
                </button>
              </div>
            ) : null}
            <div className="mt-5 grid gap-2">
              {selectedGroup.members.map((member) => (
                <div className="flex items-center justify-between rounded-xl bg-black/15 px-4 py-3" key={member.userId}>
                  <div><strong className="block">{member.fullName}</strong><span className="text-xs text-stone-500">@{member.username}</span></div>
                  {canManage ? (
                    <button className="admin-button" onClick={async () => {
                      await removeGroupMember(token, selectedGroup.id, member.userId);
                      await onChanged();
                      await onSelect(selectedGroup.id);
                    }} type="button">Remove</button>
                  ) : <span className="text-xs text-stone-500">{member.roleInGroup}</span>}
                </div>
              ))}
            </div>
          </>
        ) : null}
      </div>
    </div>
  );
}

function DevicesAdmin({ devices }: { devices: DeviceSummary[] }) {
  return (
    <div className="admin-card overflow-x-auto">
      <h2 className="font-display mb-4 text-xl">Registered devices</h2>
      <table className="admin-table">
        <thead><tr><th>Device</th><th>User</th><th>Platform</th><th>Status</th><th>Last seen</th></tr></thead>
        <tbody>
          {devices.map((device) => (
            <tr key={device.id}>
              <td>{device.deviceName}</td>
              <td>{device.fullName}<span>@{device.username}</span></td>
              <td>{device.platform}</td>
              <td>{device.status}</td>
              <td>{device.lastSeenAt ? new Date(device.lastSeenAt).toLocaleString() : "Never"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function AuditAdmin({ logs }: { logs: AuditLog[] }) {
  return (
    <div className="admin-card overflow-x-auto">
      <h2 className="font-display mb-4 text-xl">Audit trail</h2>
      <table className="admin-table">
        <thead><tr><th>Time</th><th>Actor</th><th>Action</th><th>Entity</th></tr></thead>
        <tbody>
          {logs.map((log) => (
            <tr key={log.id}>
              <td>{new Date(log.createdAt).toLocaleString()}</td>
              <td>{log.actorUsername ?? "system"}</td>
              <td><code>{log.action}</code></td>
              <td>{log.entityType}{log.entityId ? ` · ${log.entityId.slice(0, 8)}` : ""}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
