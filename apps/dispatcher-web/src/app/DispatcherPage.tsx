import type { GroupSummary, PttStateEvent, ServerRealtimeEvent } from "@ptt-fleet/shared-types";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  changePassword,
  ensureAccessToken,
  fetchGroup,
  fetchGroupLocations,
  fetchGroups,
  fetchUsers,
} from "../lib/api";
import { decodeAudioDownlink, encodeAudioUplink } from "../lib/audioEnvelope";
import { BrowserPttAudio } from "../lib/browserPttAudio";
import { createRequestId } from "../lib/requestId";
import { RealtimeClient } from "../lib/ws";
import { PasswordDialog } from "../components/PasswordDialog";
import { useAuthStore } from "../features/auth/authStore";
import { DispatcherMap } from "../features/map/DispatcherMap";
import { UserList } from "../features/users/UserList";
import { useRealtimeStore } from "../features/users/realtimeStore";

export function DispatcherPage({
  onOpenAdmin,
  onOpenHistory,
}: {
  onOpenAdmin: () => void;
  onOpenHistory: () => void;
}) {
  const session = useAuthStore((state) => state.session)!;
  const clearSession = useAuthStore((state) => state.clearSession);
  const users = useRealtimeStore((state) => state.users);
  const presence = useRealtimeStore((state) => state.presence);
  const connectionStatus = useRealtimeStore((state) => state.connectionStatus);
  const sosAlerts = useRealtimeStore((state) => state.sosAlerts);
  const setUsers = useRealtimeStore((state) => state.setUsers);
  const setLocations = useRealtimeStore((state) => state.setLocations);
  const applyEvent = useRealtimeStore((state) => state.applyEvent);
  const setConnectionStatus = useRealtimeStore((state) => state.setConnectionStatus);
  const resetRealtime = useRealtimeStore((state) => state.reset);
  const [groups, setGroups] = useState<GroupSummary[]>([]);
  const [selectedGroup, setSelectedGroup] = useState("");
  const [groupMemberIds, setGroupMemberIds] = useState<Set<string>>(new Set());
  const [targetUserId, setTargetUserId] = useState("");
  const [monitorEnabled, setMonitorEnabled] = useState(false);
  const [microphoneLevel, setMicrophoneLevel] = useState(0);
  const [lastRecordingUrl, setLastRecordingUrl] = useState("");
  const [sentFrameCount, setSentFrameCount] = useState(0);
  const [pttStatus, setPttStatus] = useState("Standby");
  const [activeSpeakerId, setActiveSpeakerId] = useState("");
  const [loadError, setLoadError] = useState("");
  const [changePasswordOpen, setChangePasswordOpen] = useState(false);
  const [mapUserId, setMapUserId] = useState("");
  const [positionRequestPending, setPositionRequestPending] = useState(false);
  const [positionRequestMessage, setPositionRequestMessage] = useState("");
  const [mapRefreshing, setMapRefreshing] = useState(false);
  const realtimeRef = useRef<RealtimeClient | null>(null);
  const activeSessionRef = useRef("");
  const audioSequenceRef = useRef(0n);
  const pttHeldRef = useRef(false);
  const positionRequestUserIdRef = useRef("");
  const positionRequestTimeoutRef = useRef<number | null>(null);
  const audioRef = useRef<BrowserPttAudio | null>(null);
  audioRef.current ??= new BrowserPttAudio(setLoadError);

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

    const handleRealtimeEvent = (event: ServerRealtimeEvent) => {
      applyEvent(event);
      if (event.type === "error") {
        setLoadError(event.payload.message);
        setPttStatus("Standby");
        pttHeldRef.current = false;
        return;
      }
      if (event.type === "gps.request.accepted") {
        setPositionRequestMessage(`Position request sent via ${event.payload.delivery}. Waiting for device...`);
        return;
      }
      if (event.type === "gps.request.failed") {
        clearPositionRequestTimeout();
        setPositionRequestPending(false);
        setPositionRequestMessage(event.payload.message);
        return;
      }
      if (
        event.type === "gps.updated" &&
        event.payload.userId === positionRequestUserIdRef.current
      ) {
        clearPositionRequestTimeout();
        setPositionRequestPending(false);
        setPositionRequestMessage("Fresh position received.");
      }
      if (!event.type.startsWith("ptt.")) return;
      const pttEvent = event as PttStateEvent;

      if (pttEvent.type === "ptt.granted") {
        activeSessionRef.current = pttEvent.payload.sessionId;
        audioSequenceRef.current = 0n;
        setSentFrameCount(0);
        if (!pttHeldRef.current) {
          sendPttStop(pttEvent.payload.sessionId);
          return;
        }
        setPttStatus("Transmitting");
        void audioRef.current
          ?.startCapture(
            (payload) => {
              const sessionId = activeSessionRef.current;
              if (!sessionId) return;
              const sent = realtimeRef.current?.sendBinary(
                encodeAudioUplink(sessionId, audioSequenceRef.current++, payload),
              );
              if (sent) setSentFrameCount((count) => count + 1);
            },
            setMicrophoneLevel,
            (recording) => {
              setLastRecordingUrl((current) => {
                if (current) URL.revokeObjectURL(current);
                return URL.createObjectURL(recording);
              });
            },
          )
          .catch((error: unknown) => {
            setLoadError(error instanceof Error ? error.message : "Unable to start microphone");
            stopBrowserPtt();
          });
        return;
      }
      if (pttEvent.type === "ptt.busy") {
        setPttStatus("Channel busy");
        return;
      }
      if (pttEvent.type === "ptt.started") {
        setActiveSpeakerId(pttEvent.payload.speakerUserId);
        if (pttEvent.payload.speakerUserId !== session.user.id) {
          setPttStatus("Receiving");
        }
        return;
      }
      if (pttEvent.type === "ptt.stopped") {
        setActiveSpeakerId("");
        setPttStatus("Standby");
        if (pttEvent.payload.sessionId === activeSessionRef.current) {
          activeSessionRef.current = "";
          setMicrophoneLevel(0);
          void audioRef.current?.stopCapture();
        }
      }
    };

    const realtime = new RealtimeClient({
      getAccessToken: ensureAccessToken,
      onEvent: handleRealtimeEvent,
      onBinary: (data) => {
        const frame = decodeAudioDownlink(data);
        if (frame) audioRef.current?.playOpus(frame.payload, frame.sequence);
      },
      onStatus: setConnectionStatus,
    });
    realtimeRef.current = realtime;
    realtime.connect();

    return () => {
      active = false;
      clearPositionRequestTimeout();
      realtime.disconnect();
      realtimeRef.current = null;
      void audioRef.current?.release();
    };
  }, [applyEvent, session.accessToken, setConnectionStatus, setUsers]);

  useEffect(
    () => () => {
      if (lastRecordingUrl) URL.revokeObjectURL(lastRecordingUrl);
    },
    [lastRecordingUrl],
  );

  useEffect(() => {
    if (!selectedGroup || connectionStatus !== "connected") {
      return;
    }
    realtimeRef.current?.send({
      type: "group.join",
      requestId: createRequestId(),
      timestamp: new Date().toISOString(),
      payload: { groupId: selectedGroup },
    });
  }, [connectionStatus, selectedGroup]);

  const onlineCount = useMemo(
    () => users.filter((user) => presence[user.id]?.status === "online").length,
    [presence, users],
  );
  const activeSosCount = Object.values(sosAlerts).filter((alert) => alert.status === "open").length;

  useEffect(() => {
    if (activeSosCount === 0) {
      document.title = "PTT Fleet Dispatcher";
      return;
    }
    document.title = `(${activeSosCount}) SOS · PTT Fleet`;
    const audioContext = new AudioContext();
    const oscillator = audioContext.createOscillator();
    const gain = audioContext.createGain();
    oscillator.frequency.value = 880;
    gain.gain.value = 0.08;
    oscillator.connect(gain);
    gain.connect(audioContext.destination);
    oscillator.start();
    oscillator.stop(audioContext.currentTime + 0.35);
    return () => {
      void audioContext.close();
    };
  }, [activeSosCount]);

  function handleLogout() {
    stopBrowserPtt();
    realtimeRef.current?.disconnect();
    resetRealtime();
    clearSession();
  }

  function handleGroupChange(groupId: string) {
    setSelectedGroup(groupId);
    setTargetUserId("");
    setGroupMemberIds(new Set());
    setLocations({});
  }

  const fetchGroupSnapshot = useCallback(async () => {
    if (!selectedGroup) {
      throw new Error("No group selected");
    }
    const [group, locationResult] = await Promise.all([
      fetchGroup(session.accessToken, selectedGroup),
      fetchGroupLocations(session.accessToken, selectedGroup, 24),
    ]);
    const locations = Object.fromEntries(
      locationResult.items.map((location) => [
        location.userId,
        {
          lat: location.lat,
          lng: location.lng,
          speed: location.speed,
          heading: location.heading,
          accuracy: location.accuracy,
          recordedAt: location.recordedAt,
        },
      ]),
    );
    return { group, locations };
  }, [selectedGroup, session.accessToken]);

  useEffect(() => {
    if (!selectedGroup) return;
    let active = true;
    void fetchGroupSnapshot()
      .then(({ group, locations }) => {
        if (!active) return;
        setGroupMemberIds(new Set(group.members.map((member) => member.userId)));
        setLocations(locations);
      })
      .catch((error: unknown) => {
        if (active) {
          setLoadError(error instanceof Error ? error.message : "Unable to load group members");
        }
      });
    return () => {
      active = false;
    };
  }, [fetchGroupSnapshot, selectedGroup, setLocations]);

  async function refreshMapLocations() {
    if (!selectedGroup) return;
    setMapRefreshing(true);
    setLoadError("");
    try {
      const { locations } = await fetchGroupSnapshot();
      setLocations(locations);
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : "Unable to refresh map locations");
    } finally {
      setMapRefreshing(false);
    }
  }

  async function toggleMonitor() {
    if (monitorEnabled) {
      audioRef.current?.disableMonitor();
      setMonitorEnabled(false);
      return;
    }
    try {
      await audioRef.current?.enableMonitor();
      setMonitorEnabled(true);
      setLoadError("");
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : "Unable to enable audio monitor");
    }
  }

  function startBrowserPtt() {
    if (!selectedGroup || connectionStatus !== "connected") return;
    pttHeldRef.current = true;
    setPttStatus(targetUserId ? "Requesting direct talk" : "Requesting broadcast");
    realtimeRef.current?.send({
      type: "ptt.start",
      requestId: createRequestId(),
      timestamp: new Date().toISOString(),
      payload: {
        groupId: selectedGroup,
        ...(targetUserId ? { targetUserId } : {}),
      },
    });
  }

  function stopBrowserPtt() {
    pttHeldRef.current = false;
    setMicrophoneLevel(0);
    void audioRef.current?.stopCapture();
    const sessionId = activeSessionRef.current;
    activeSessionRef.current = "";
    if (sessionId) sendPttStop(sessionId);
    setPttStatus("Standby");
  }

  function sendPttStop(sessionId: string) {
    realtimeRef.current?.send({
      type: "ptt.stop",
      requestId: createRequestId(),
      timestamp: new Date().toISOString(),
      payload: { sessionId },
    });
  }

  function clearPositionRequestTimeout() {
    if (positionRequestTimeoutRef.current !== null) {
      window.clearTimeout(positionRequestTimeoutRef.current);
      positionRequestTimeoutRef.current = null;
    }
    positionRequestUserIdRef.current = "";
  }

  function requestPosition(userId: string) {
    if (!selectedGroup || connectionStatus !== "connected") return;
    clearPositionRequestTimeout();
    positionRequestUserIdRef.current = userId;
    setPositionRequestPending(true);
    setPositionRequestMessage("");
    positionRequestTimeoutRef.current = window.setTimeout(() => {
      setPositionRequestPending(false);
      setPositionRequestMessage("Device has not returned a fresh position yet.");
      positionRequestTimeoutRef.current = null;
      positionRequestUserIdRef.current = "";
    }, 45_000);
    realtimeRef.current?.send({
      type: "gps.request",
      requestId: createRequestId(),
      timestamp: new Date().toISOString(),
      payload: {
        groupId: selectedGroup,
        targetUserId: userId,
      },
    });
  }

  function acknowledgeSos(id: string) {
    realtimeRef.current?.send({
      type: "sos.ack",
      requestId: createRequestId(),
      timestamp: new Date().toISOString(),
      payload: { id },
    });
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
              onChange={(event) => handleGroupChange(event.target.value)}
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
              className="rounded-xl border border-white/10 px-3 py-2 text-xs font-bold uppercase tracking-wider text-stone-300 transition hover:bg-white/5"
              onClick={onOpenHistory}
              type="button"
            >
              History
            </button>
            {session.user.role !== "field_user" ? (
              <button
                className="rounded-xl border border-white/10 px-3 py-2 text-xs font-bold uppercase tracking-wider text-stone-300 transition hover:bg-white/5"
                onClick={onOpenAdmin}
                type="button"
              >
                Admin
              </button>
            ) : null}
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
              <UserList
                onSelectUser={(userId) =>
                  setTargetUserId((current) => (current === userId ? "" : userId))
                }
                presence={presence}
                selectedUserId={targetUserId}
                users={users.filter(
                  (user) => user.role === "field_user" && groupMemberIds.has(user.id),
                )}
              />
            </div>
          </aside>

          <section className="min-h-[460px]">
            <DispatcherMap
              onAcknowledgeSos={acknowledgeSos}
              onCloseUser={() => {
                setMapUserId("");
                clearPositionRequestTimeout();
                setPositionRequestPending(false);
                setPositionRequestMessage("");
              }}
              onRequestPosition={requestPosition}
              onRefreshLocations={() => void refreshMapLocations()}
              onSelectUser={(userId) => {
                setMapUserId(userId);
                clearPositionRequestTimeout();
                setPositionRequestPending(false);
                setPositionRequestMessage("");
              }}
              positionRequestMessage={positionRequestMessage}
              positionRequestPending={positionRequestPending}
              refreshing={mapRefreshing}
              selectedUserId={mapUserId}
            />
          </section>
        </div>

        <footer className="grid gap-3 border-t border-white/8 bg-[#0d1512] px-4 py-3 sm:grid-cols-[1fr_auto_1fr] sm:items-center sm:px-6">
          <div className="flex items-center gap-3 text-xs text-stone-500">
            <span className="rounded-md border border-white/8 px-2 py-1 font-mono text-[10px] text-stone-400">
              LIVE
            </span>
            <span>
              {pttStatus}
              {activeSpeakerId
                ? ` · ${users.find((user) => user.id === activeSpeakerId)?.fullName ?? "speaker"}`
                : ""}
            </span>
          </div>

          <div className="flex flex-wrap items-center justify-center gap-2">
            <div
              aria-label={`Microphone level ${Math.round(microphoneLevel * 100)} percent`}
              className="w-24"
              title="Microphone input level"
            >
              <div className="mb-1 flex justify-between text-[9px] font-bold uppercase tracking-wider text-stone-500">
                <span>Mic</span>
                <span>{microphoneLevel > 0.03 ? "Active" : "Silent"}</span>
              </div>
              <div className="h-2 overflow-hidden rounded-full bg-black/40">
                <div
                  className={`h-full rounded-full transition-[width,background-color] duration-75 ${
                    microphoneLevel > 0.75
                      ? "bg-red-400"
                      : microphoneLevel > 0.35
                        ? "bg-amber-300"
                        : "bg-emerald-400"
                  }`}
                  style={{ width: `${Math.max(2, microphoneLevel * 100)}%` }}
                />
              </div>
              <div className="mt-1 text-[9px] text-stone-600">{sentFrameCount} frames sent</div>
            </div>
            {lastRecordingUrl ? (
              <>
                <audio className="h-9 w-44" controls src={lastRecordingUrl}>
                  <track kind="captions" />
                </audio>
                <a
                  className="rounded-xl border border-white/10 px-3 py-2.5 text-[10px] font-bold uppercase tracking-wider text-stone-400"
                  download={`dispatcher-mic-${new Date().toISOString().replaceAll(":", "-")}.wav`}
                  href={lastRecordingUrl}
                >
                  Save WAV
                </a>
              </>
            ) : null}
            <button
              className="rounded-xl border border-white/10 px-3 py-2 text-xs font-bold uppercase tracking-wider text-stone-300 transition hover:bg-white/5"
              onClick={() => setChangePasswordOpen(true)}
              type="button"
            >
              Change password
            </button>
            <button
              className={`rounded-xl border px-3 py-2.5 text-xs font-bold uppercase tracking-[0.14em] ${
                monitorEnabled
                  ? "border-emerald-300/40 bg-emerald-300/10 text-emerald-200"
                  : "border-white/10 text-stone-400"
              }`}
              onClick={() => void toggleMonitor()}
              type="button"
            >
              Monitor {monitorEnabled ? "on" : "off"}
            </button>
            <button
              className="touch-none rounded-xl border border-amber-300/30 bg-amber-300/10 px-5 py-2.5 text-xs font-bold uppercase tracking-[0.18em] text-amber-100 disabled:opacity-40"
              disabled={!selectedGroup || connectionStatus !== "connected"}
              onContextMenu={(event) => event.preventDefault()}
              onPointerCancel={stopBrowserPtt}
              onPointerDown={startBrowserPtt}
              onPointerLeave={() => {
                if (pttHeldRef.current) stopBrowserPtt();
              }}
              onPointerUp={stopBrowserPtt}
              type="button"
            >
              Hold PTT
            </button>
            {targetUserId ? (
              <button
                className="rounded-xl border border-emerald-300/30 bg-emerald-300/10 px-3 py-2.5 text-[10px] font-bold uppercase tracking-wider text-emerald-200 disabled:opacity-40"
                disabled={positionRequestPending || connectionStatus !== "connected"}
                onClick={() => requestPosition(targetUserId)}
                type="button"
              >
                {positionRequestPending ? "Requesting position" : "Request position"}
              </button>
            ) : null}
            <button
              className="rounded-xl border border-white/10 px-3 py-2.5 text-[10px] font-bold uppercase tracking-wider text-stone-400"
              onClick={() => setTargetUserId("")}
              type="button"
            >
              {targetUserId
                ? `Direct: ${users.find((user) => user.id === targetUserId)?.fullName ?? "user"}`
                : "Broadcast channel"}
            </button>
          </div>

          <div className="text-right text-[11px] uppercase tracking-wider text-stone-600">
            SOS alerts · {activeSosCount} active
          </div>
        </footer>
      </div>
      <PasswordDialog
        description="Password baru akan mencabut seluruh sesi. Anda perlu login kembali setelah berhasil."
        onClose={() => setChangePasswordOpen(false)}
        onSubmit={async (currentPassword, newPassword) => {
          await changePassword(session.accessToken, currentPassword, newPassword);
          handleLogout();
        }}
        open={changePasswordOpen}
        requireCurrentPassword
        submitLabel="Change password"
        title="Change your password"
      />
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
