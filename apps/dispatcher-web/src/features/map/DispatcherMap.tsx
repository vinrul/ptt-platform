import { useEffect, useRef } from "react";
import { Map as MaptalksMap, Marker, TileLayer, VectorLayer } from "maptalks";
import { useRealtimeStore } from "../users/realtimeStore";

export function DispatcherMap() {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<MaptalksMap | null>(null);
  const markerLayerRef = useRef<VectorLayer | null>(null);
  const markersRef = useRef(new globalThis.Map<string, Marker>());
  const hasCenteredRef = useRef(false);
  const locations = useRealtimeStore((state) => state.locations);
  const users = useRealtimeStore((state) => state.users);

  useEffect(() => {
    if (!containerRef.current || mapRef.current) {
      return;
    }
    const markers = markersRef.current;

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
