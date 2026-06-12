import type { UserSummary } from "@ptt-fleet/shared-types";
import { useEffect, useMemo, useRef, useState } from "react";
import { LineString, Map as MaptalksMap, Marker, TileLayer, VectorLayer } from "maptalks";
import {
  fetchGpsHistoryForDate,
  fetchRouteLine,
  fetchUsers,
  type GpsHistoryPoint,
} from "../lib/api";
import { useAuthStore } from "../features/auth/authStore";

interface HistoryPageProps {
  onBack: () => void;
  onOpenAdmin: () => void;
}

interface UserTrack {
  user: UserSummary;
  points: GpsHistoryPoint[];
  route: Array<[number, number]>;
  error?: string;
}

const routeColors = ["#fbbf24", "#61e6a8", "#60a5fa", "#f472b6", "#fb7185", "#a78bfa"];

export function HistoryPage({ onBack, onOpenAdmin }: HistoryPageProps) {
  const session = useAuthStore((state) => state.session)!;
  const [date, setDate] = useState(todayInputValue());
  const [tracks, setTracks] = useState<UserTrack[]>([]);
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState("");

  useEffect(() => {
    let active = true;
    void Promise.resolve().then(async () => {
      if (!active) return;
      setLoading(true);
      setMessage("");
      setTracks([]);
      try {
        const result = await fetchUsers(session.accessToken);
        const fieldUsers = result.items.filter((user) => user.role === "field_user");
        const loadedTracks = await Promise.all(
          fieldUsers.map(async (user): Promise<UserTrack> => {
            const history = await fetchGpsHistoryForDate(session.accessToken, user.id, date);
            const points = history.items;
            if (points.length < 2) {
              return { user, points, route: [] };
            }
            try {
              return { user, points, route: (await fetchRouteLine(session.accessToken, points)).coordinates };
            } catch (error) {
              return {
                user,
                points,
                route: chronologicalCoordinates(points),
                error: error instanceof Error ? error.message : "Gagal memuat rute.",
              };
            }
          }),
        );
        if (active) setTracks(loadedTracks);
      } catch (error: unknown) {
        if (active) {
          setMessage(error instanceof Error ? error.message : "Gagal memuat history GPS.");
        }
      } finally {
        if (active) setLoading(false);
      }
    });

    return () => {
      active = false;
    };
  }, [date, session.accessToken]);

  const trackedCount = tracks.filter((track) => track.points.length > 0).length;

  return (
    <main className="min-h-screen bg-[#09110f] text-stone-100">
      <header className="flex flex-wrap items-center justify-between gap-4 border-b border-white/8 px-5 py-4">
        <div>
          <p className="text-[10px] font-bold uppercase tracking-[0.2em] text-emerald-300">
            Location history
          </p>
          <h1 className="font-display mt-1 text-2xl">Dispatcher route playback</h1>
        </div>
        <div className="flex gap-2">
          <button className="admin-button" onClick={onBack} type="button">
            Live dispatcher
          </button>
          {session.user.role !== "field_user" ? (
            <button className="admin-button" onClick={onOpenAdmin} type="button">
              Admin
            </button>
          ) : null}
        </div>
      </header>

      <div className="grid min-h-[calc(100vh-73px)] lg:grid-cols-[320px_1fr]">
        <aside className="border-b border-white/8 bg-[#101815] p-4 lg:border-b-0 lg:border-r">
          <label className="block text-[10px] font-bold uppercase tracking-[0.2em] text-stone-500">
            Tanggal
            <input
              className="mt-2 w-full rounded-xl border border-stone-700 bg-stone-900 px-3 py-2 text-sm text-stone-100 outline-none focus:border-emerald-400"
              onChange={(event) => setDate(event.target.value)}
              type="date"
              value={date}
            />
          </label>

          <div className="mt-4 rounded-xl border border-white/8 bg-white/[0.035] p-3">
            <div className="text-[10px] font-bold uppercase tracking-wider text-stone-500">
              Track loaded
            </div>
            <div className="mt-1 font-display text-2xl text-stone-100">
              {loading ? "..." : trackedCount}
            </div>
          </div>

          {message ? (
            <div className="mt-4 rounded-xl border border-red-300/20 bg-red-950/25 p-3 text-sm text-red-200">
              {message}
            </div>
          ) : null}

          <div className="mt-4 max-h-[58vh] space-y-2 overflow-y-auto">
            {tracks.map((track, index) => (
              <TrackSummary
                color={routeColors[index % routeColors.length]}
                key={track.user.id}
                track={track}
              />
            ))}
          </div>
        </aside>

        <HistoryMap tracks={tracks} />
      </div>
    </main>
  );
}

function HistoryMap({ tracks }: { tracks: UserTrack[] }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<MaptalksMap | null>(null);
  const layerRef = useRef<VectorLayer | null>(null);

  useEffect(() => {
    if (!containerRef.current || mapRef.current) return;
    const map = new MaptalksMap(containerRef.current, {
      center: [115.2167, -8.65],
      zoom: 11,
      attribution: false,
      zoomControl: false,
      baseLayer: new TileLayer("base", {
        urlTemplate: "https://tile.openstreetmap.org/{z}/{x}/{y}.png",
        subdomains: ["a", "b", "c"],
      }),
    });
    layerRef.current = new VectorLayer("history-routes").addTo(map);
    mapRef.current = map;

    return () => {
      layerRef.current = null;
      map.remove();
      mapRef.current = null;
    };
  }, []);

  useEffect(() => {
    const map = mapRef.current;
    const layer = layerRef.current;
    if (!map || !layer) return;
    layer.clear();

    const visibleRoutes = tracks.filter((track) => track.route.length > 1);
    visibleRoutes.forEach((track, index) => {
      const color = routeColors[index % routeColors.length];
      new LineString(track.route, {
        symbol: {
          lineColor: color,
          lineWidth: 5,
          lineOpacity: 0.9,
        },
      }).addTo(layer);

      const first = track.route[0];
      const latest = track.route.at(-1);
      if (first) {
        new Marker(first, {
          symbol: pointSymbol("#f8fafc", color, 16),
        }).addTo(layer);
      }
      if (latest) {
        new Marker(latest, {
          symbol: pointSymbol(color, "#07110f", 22),
        }).addTo(layer);
      }
    });

    const latest = visibleRoutes.at(-1)?.route.at(-1);
    if (latest) map.animateTo({ center: latest, zoom: 14 });
  }, [tracks]);

  return (
    <section className="relative min-h-[520px] overflow-hidden bg-[#17211d]">
      <div ref={containerRef} className="absolute inset-0" />
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(180deg,rgba(7,17,15,.03),rgba(7,17,15,.22))]" />
    </section>
  );
}

function TrackSummary({ color, track }: { color: string; track: UserTrack }) {
  const latest = useMemo(
    () =>
      [...track.points].sort((left, right) =>
        right.recordedAt.localeCompare(left.recordedAt),
      )[0],
    [track.points],
  );

  return (
    <article className="rounded-xl border border-white/8 bg-white/[0.035] p-3 text-sm">
      <div className="flex items-start gap-3">
        <span className="mt-1 h-3 w-3 rounded-full" style={{ backgroundColor: color }} />
        <div className="min-w-0 flex-1">
          <div className="truncate font-semibold text-stone-200">{track.user.fullName}</div>
          <div className="mt-1 text-[11px] uppercase tracking-wider text-stone-500">
            {track.points.length} titik
            {latest ? ` - ${formatDate(latest.recordedAt)}` : ""}
          </div>
          {track.error ? <div className="mt-2 text-xs text-amber-300">{track.error}</div> : null}
        </div>
      </div>
    </article>
  );
}

function chronologicalCoordinates(points: GpsHistoryPoint[]): Array<[number, number]> {
  return [...points]
    .sort((left, right) => left.recordedAt.localeCompare(right.recordedAt))
    .map((point) => [point.lng, point.lat]);
}

function pointSymbol(fill: string, line: string, size: number) {
  return {
    markerType: "ellipse",
    markerFill: fill,
    markerLineColor: line,
    markerLineWidth: 3,
    markerWidth: size,
    markerHeight: size,
  };
}

function todayInputValue(): string {
  const today = new Date();
  const offset = today.getTimezoneOffset() * 60_000;
  return new Date(today.getTime() - offset).toISOString().slice(0, 10);
}

function formatDate(value: string): string {
  return new Intl.DateTimeFormat("id-ID", {
    dateStyle: "short",
    timeStyle: "short",
  }).format(new Date(value));
}
