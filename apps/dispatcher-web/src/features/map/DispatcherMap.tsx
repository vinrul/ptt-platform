import { useEffect, useRef, useState } from "react";
import { Map as MaptalksMap, Marker, TileLayer, VectorLayer } from "maptalks";
import { fetchReverseGeocode } from "../../lib/api";
import { useAuthStore } from "../auth/authStore";
import { useRealtimeStore } from "../users/realtimeStore";

interface DispatcherMapProps {
  onAcknowledgeSos: (id: string) => void;
  onCloseUser: () => void;
  onRequestPosition: (userId: string) => void;
  onRefreshLocations: () => void;
  onSelectUser: (userId: string) => void;
  positionRequestMessage: string;
  positionRequestPending: boolean;
  refreshing: boolean;
  selectedUserId: string;
}

export function DispatcherMap({
  onAcknowledgeSos,
  onCloseUser,
  onRequestPosition,
  onRefreshLocations,
  onSelectUser,
  positionRequestMessage,
  positionRequestPending,
  refreshing,
  selectedUserId,
}: DispatcherMapProps) {
  const session = useAuthStore((state) => state.session)!;
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<MaptalksMap | null>(null);
  const markerLayerRef = useRef<VectorLayer | null>(null);
  const markersRef = useRef(new globalThis.Map<string, Marker>());
  const sosMarkersRef = useRef(new globalThis.Map<string, Marker>());
  const geocodeCacheRef = useRef(new globalThis.Map<string, string>());
  const hasCenteredRef = useRef(false);
  const lastFocusedSosRef = useRef<string | null>(null);
  const [address, setAddress] = useState("");
  const [addressLoading, setAddressLoading] = useState(false);
  const [addressError, setAddressError] = useState("");
  const locations = useRealtimeStore((state) => state.locations);
  const users = useRealtimeStore((state) => state.users);
  const presence = useRealtimeStore((state) => state.presence);
  const sosAlerts = useRealtimeStore((state) => state.sosAlerts);
  const focusedSosId = useRealtimeStore((state) => state.focusedSosId);

  useEffect(() => {
    if (!containerRef.current || mapRef.current) {
      return;
    }
    const markers = markersRef.current;
    const sosMarkers = sosMarkersRef.current;

    const map = new MaptalksMap(containerRef.current, {
      center: [115.2167, -8.65],
      zoom: 11,
      pitch: 0,
      attribution: false,
      zoomControl: false,
      baseLayer: new TileLayer("base", {
        urlTemplate: "https://tile.openstreetmap.org/{z}/{x}/{y}.png",
        subdomains: ["a", "b", "c"],
      }),
    });
    markerLayerRef.current = new VectorLayer("fleet-markers").addTo(map);
    mapRef.current = map;

    return () => {
      markers.clear();
      sosMarkers.clear();
      markerLayerRef.current = null;
      map.remove();
      mapRef.current = null;
    };
  }, []);

  useEffect(() => {
    const map = mapRef.current;
    const layer = markerLayerRef.current;
    if (!map || !layer) return;

    const activeUserIds = new Set(Object.keys(locations));
    for (const [userId, marker] of markersRef.current) {
      if (!activeUserIds.has(userId)) {
        marker.remove();
        markersRef.current.delete(userId);
      }
    }

    for (const [userId, location] of Object.entries(locations)) {
      const coordinates: [number, number] = [location.lng, location.lat];
      const existing = markersRef.current.get(userId);
      if (existing) {
        existing.setCoordinates(coordinates);
        continue;
      }

      const user = users.find((candidate) => candidate.id === userId);
      const marker = new Marker(coordinates, {
        properties: {
          userId,
          label: user?.fullName ?? userId,
        },
        symbol: {
          markerType: "ellipse",
          markerFill: "#61e6a8",
          markerFillOpacity: 0.95,
          markerLineColor: "#07110f",
          markerLineWidth: 3,
          markerWidth: 24,
          markerHeight: 24,
          shadowBlur: 8,
          shadowColor: "rgba(97, 230, 168, 0.5)",
        },
      }).addTo(layer);
      marker.on("click", () => onSelectUser(userId));
      markersRef.current.set(userId, marker);

      if (!hasCenteredRef.current) {
        hasCenteredRef.current = true;
        map.animateTo({ center: coordinates, zoom: 15 });
      }
    }
  }, [locations, onSelectUser, users]);

  useEffect(() => {
    const map = mapRef.current;
    const layer = markerLayerRef.current;
    if (!map || !layer) return;

    for (const alert of Object.values(sosAlerts)) {
      if (alert.lat === undefined || alert.lng === undefined) continue;
      const coordinates: [number, number] = [alert.lng, alert.lat];
      const existing = sosMarkersRef.current.get(alert.id);
      if (existing) {
        existing.setCoordinates(coordinates);
        existing.updateSymbol({
          markerFill: alert.status === "open" ? "#ef4444" : "#f59e0b",
        });
      } else {
        const marker = new Marker(coordinates, {
          symbol: {
            markerType: "triangle",
            markerFill: alert.status === "open" ? "#ef4444" : "#f59e0b",
            markerLineColor: "#ffffff",
            markerLineWidth: 3,
            markerWidth: 34,
            markerHeight: 34,
          },
        }).addTo(layer);
        sosMarkersRef.current.set(alert.id, marker);
      }
    }

    const focused = focusedSosId ? sosAlerts[focusedSosId] : undefined;
    if (
      focused &&
      focused.lat !== undefined &&
      focused.lng !== undefined &&
      lastFocusedSosRef.current !== focused.id
    ) {
      lastFocusedSosRef.current = focused.id;
      map.animateTo({ center: [focused.lng, focused.lat], zoom: 17 });
    }
  }, [focusedSosId, sosAlerts]);

  const activeSos = Object.values(sosAlerts)
    .filter((alert) => alert.status === "open")
    .sort((left, right) => right.createdAt.localeCompare(left.createdAt));
  const selectedUser = users.find((user) => user.id === selectedUserId);
  const selectedLocation = selectedUserId ? locations[selectedUserId] : undefined;

  useEffect(() => {
    let active = true;
    void Promise.resolve().then(async () => {
      if (!active) return;
      setAddress("");
      setAddressError("");
      setAddressLoading(false);
      if (!selectedLocation) return;

      const key = `${selectedLocation.lat.toFixed(5)},${selectedLocation.lng.toFixed(5)}`;
      const cached = geocodeCacheRef.current.get(key);
      if (cached) {
        setAddress(cached);
        return;
      }

      setAddressLoading(true);
      try {
        const result = await fetchReverseGeocode(
          session.accessToken,
          selectedLocation.lat,
          selectedLocation.lng,
        );
        geocodeCacheRef.current.set(key, result.displayName);
        if (active) setAddress(result.displayName);
      } catch (error: unknown) {
        if (active) {
          setAddressError(error instanceof Error ? error.message : "Gagal memuat alamat.");
        }
      } finally {
        if (active) setAddressLoading(false);
      }
    });

    return () => {
      active = false;
    };
  }, [selectedLocation, session.accessToken]);

  return (
    <div className="relative h-full min-h-[420px] overflow-hidden bg-[#17211d]">
      <div ref={containerRef} className="absolute inset-0" />
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(180deg,rgba(7,17,15,.03),rgba(7,17,15,.22))]" />

      <div className="absolute left-4 top-4 flex items-center gap-2">
        <div className="flex items-center gap-2 rounded-full border border-white/12 bg-[#0a1311]/85 px-3 py-2 text-[11px] font-semibold uppercase tracking-[0.16em] text-stone-300 shadow-xl backdrop-blur">
          <span className="h-2 w-2 animate-pulse rounded-full bg-emerald-400" />
          Live operation map
        </div>
        <button
          aria-label="Refresh live map locations"
          className="grid h-9 w-9 place-items-center rounded-full border border-white/12 bg-[#0a1311]/85 text-sm font-black text-stone-300 shadow-xl backdrop-blur transition hover:bg-white/8 disabled:opacity-50"
          disabled={refreshing}
          onClick={onRefreshLocations}
          title="Refresh live map locations"
          type="button"
        >
          {refreshing ? "..." : "R"}
        </button>
      </div>

      <div className="absolute left-4 top-16 rounded-lg border border-white/10 bg-[#0a1311]/80 px-3 py-2 text-xs text-stone-300 backdrop-blur">
        {Object.keys(locations).length} tracked units
      </div>

      {selectedUser ? (
        <div className="absolute bottom-4 left-4 z-10 max-h-[70%] w-[min(390px,calc(100%-2rem))] overflow-y-auto rounded-2xl border border-emerald-300/25 bg-[#0a1311]/95 p-4 shadow-2xl backdrop-blur">
          <div className="flex items-start justify-between gap-4">
            <div>
              <p className="text-[10px] font-black uppercase tracking-[0.2em] text-emerald-300">
                Unit detail
              </p>
              <h3 className="mt-1 font-display text-xl text-stone-100">
                {selectedUser.fullName}
              </h3>
              <p className="mt-1 text-xs text-stone-400">
                @{selectedUser.username} · {selectedUser.role.replace("_", " ")}
              </p>
            </div>
            <button
              aria-label="Close user detail"
              className="rounded-lg px-2 py-1 text-stone-500 hover:bg-white/8 hover:text-stone-200"
              onClick={onCloseUser}
              type="button"
            >
              ×
            </button>
          </div>

          <div className="mt-4 grid grid-cols-2 gap-2 text-xs">
            <Detail
              label="Presence"
              value={presence[selectedUserId]?.status === "online" ? "Online" : "Offline"}
            />
            <Detail label="Account" value={selectedUser.status} />
            <Detail
              label="Latitude"
              value={selectedLocation ? selectedLocation.lat.toFixed(6) : "No live location"}
            />
            <Detail
              label="Longitude"
              value={selectedLocation ? selectedLocation.lng.toFixed(6) : "No live location"}
            />
            <Detail
              label="Accuracy"
              value={
                selectedLocation?.accuracy !== undefined
                  ? `${selectedLocation.accuracy.toFixed(1)} m`
                  : "-"
              }
            />
            <Detail
              label="Last update"
              value={selectedLocation ? formatDate(selectedLocation.recordedAt) : "-"}
            />
          </div>

          <div className="mt-3 rounded-xl bg-white/[0.035] px-3 py-2 text-xs">
            <div className="text-[9px] font-bold uppercase tracking-wider text-stone-600">
              Address
            </div>
            <div className="mt-1 leading-relaxed text-stone-300">
              {addressLoading ? "Memuat alamat..." : address || "Alamat belum tersedia."}
            </div>
            {addressError ? <div className="mt-2 text-[11px] text-red-300">{addressError}</div> : null}
          </div>

          <button
            className="mt-3 w-full rounded-xl bg-emerald-300 px-4 py-2.5 text-xs font-black uppercase tracking-[0.15em] text-emerald-950 disabled:opacity-50"
            disabled={positionRequestPending || !selectedLocation}
            onClick={() => onRequestPosition(selectedUser.id)}
            type="button"
          >
            {positionRequestPending ? "Requesting position..." : "Request position"}
          </button>
          {positionRequestMessage ? (
            <p className="mt-2 text-xs text-stone-400">{positionRequestMessage}</p>
          ) : null}
        </div>
      ) : null}

      {activeSos.length > 0 ? (
        <div className="absolute right-4 top-4 w-[min(360px,calc(100%-2rem))] rounded-2xl border border-red-300/40 bg-red-950/95 p-4 text-red-50 shadow-2xl shadow-red-950/50 backdrop-blur">
          <div className="flex items-center justify-between gap-3">
            <div>
              <p className="text-[10px] font-black uppercase tracking-[0.22em] text-red-300">
                Emergency alert
              </p>
              <h3 className="mt-1 font-display text-xl">{activeSos[0].message}</h3>
            </div>
            <span className="h-3 w-3 animate-ping rounded-full bg-red-400" />
          </div>
          <p className="mt-2 text-xs text-red-200">
            {users.find((user) => user.id === activeSos[0].userId)?.fullName ??
              activeSos[0].userId}
          </p>
          <button
            className="mt-4 w-full rounded-xl bg-red-100 px-4 py-2.5 text-xs font-black uppercase tracking-[0.15em] text-red-950"
            onClick={() => onAcknowledgeSos(activeSos[0].id)}
            type="button"
          >
            Acknowledge SOS
          </button>
          {activeSos.length > 1 ? (
            <p className="mt-2 text-center text-[11px] text-red-300">
              +{activeSos.length - 1} emergency alerts waiting
            </p>
          ) : null}
        </div>
      ) : null}

      <div className="absolute bottom-4 right-4 rounded-xl border border-white/10 bg-[#0a1311]/88 p-2 shadow-xl backdrop-blur">
        <button
          aria-label="Center operation map"
          className="grid h-9 w-9 place-items-center rounded-lg text-lg text-stone-300 transition hover:bg-white/8"
          onClick={() => mapRef.current?.animateTo({ center: [115.2167, -8.65], zoom: 11 })}
          type="button"
        >
          ◎
        </button>
      </div>
    </div>
  );
}

function Detail({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg bg-white/[0.035] px-3 py-2">
      <div className="text-[9px] font-bold uppercase tracking-wider text-stone-600">{label}</div>
      <div className="mt-1 truncate text-stone-300">{value}</div>
    </div>
  );
}

function formatDate(value: string): string {
  return new Intl.DateTimeFormat("id-ID", {
    dateStyle: "short",
    timeStyle: "medium",
  }).format(new Date(value));
}
