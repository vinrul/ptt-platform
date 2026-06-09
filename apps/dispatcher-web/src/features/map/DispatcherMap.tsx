import { useEffect, useRef } from "react";
import { Map as MaptalksMap, Marker, TileLayer, VectorLayer } from "maptalks";
import { useRealtimeStore } from "../users/realtimeStore";

interface DispatcherMapProps {
  onAcknowledgeSos: (id: string) => void;
}

export function DispatcherMap({ onAcknowledgeSos }: DispatcherMapProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<MaptalksMap | null>(null);
  const markerLayerRef = useRef<VectorLayer | null>(null);
  const markersRef = useRef(new globalThis.Map<string, Marker>());
  const sosMarkersRef = useRef(new globalThis.Map<string, Marker>());
  const hasCenteredRef = useRef(false);
  const lastFocusedSosRef = useRef<string | null>(null);
  const locations = useRealtimeStore((state) => state.locations);
  const users = useRealtimeStore((state) => state.users);
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
      markersRef.current.set(userId, marker);

      if (!hasCenteredRef.current) {
        hasCenteredRef.current = true;
        map.animateTo({ center: coordinates, zoom: 15 });
      }
    }
  }, [locations, users]);

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

  return (
    <div className="relative h-full min-h-[420px] overflow-hidden bg-[#17211d]">
      <div ref={containerRef} className="absolute inset-0" />
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(180deg,rgba(7,17,15,.03),rgba(7,17,15,.22))]" />

      <div className="absolute left-4 top-4 flex items-center gap-2 rounded-full border border-white/12 bg-[#0a1311]/85 px-3 py-2 text-[11px] font-semibold uppercase tracking-[0.16em] text-stone-300 shadow-xl backdrop-blur">
        <span className="h-2 w-2 animate-pulse rounded-full bg-emerald-400" />
        Live operation map
      </div>

      <div className="absolute left-4 top-16 rounded-lg border border-white/10 bg-[#0a1311]/80 px-3 py-2 text-xs text-stone-300 backdrop-blur">
        {Object.keys(locations).length} tracked units
      </div>

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
