# Repository Structure

Struktur repo target:

```text
ptt-fleet/
  AGENTS.md
  PLAN.md
  README.md
  package.json
  bun.lockb
  .env.example

  apps/
    dispatcher-web/
      src/
        app/
        components/
        features/
          auth/
          map/
          ptt/
          sos/
          users/
        lib/
          api.ts
          ws.ts
          maptalks.ts
        main.tsx
      package.json
      vite.config.ts

    android-kotlin/
      app/
        src/main/java/id/nuwiarul/pttfleet/
          auth/
          audio/
          gps/
          ptt/
          sos/
          websocket/

  services/
    api-server/
      cmd/
        server/
          main.go
      internal/
        auth/
        config/
        db/
        devices/
        gps/
        groups/
        httpserver/
        ptt/
        sos/
        users/
        ws/
      migrations/
      go.mod

  packages/
    shared-types/
      src/
        events.ts
        models.ts
      package.json

  infra/
    docker/
      docker-compose.local.yml
      docker-compose.prod.yml
      # local compose runs postgres + redis first;
      # app services are enabled later with profile app
    caddy/
      Caddyfile
    nginx/
      nginx.conf

  docs/
    API.md
    ARCHITECTURE.md
    DATABASE.md
    DEPLOYMENT.md
    ROADMAP.md
    STRUCTURE.md
    WEBSOCKET_PROTOCOL.md
```

## Backend Package Rules

- `cmd/server`: composition root, process startup, graceful shutdown.
- `internal/config`: env parsing and validation.
- `internal/db`: PostgreSQL and Redis clients.
- `internal/httpserver`: Gin router, middleware, route registration.
- `internal/auth`: login, JWT, refresh token, password hash.
- `internal/users`: user service and handlers.
- `internal/groups`: group and membership service.
- `internal/devices`: device registration and last seen.
- `internal/ws`: WebSocket hub, connection manager, event envelope.
- `internal/ptt`: talk lock, talk session, audio relay.
- `internal/gps`: GPS validation, persistence, broadcast.
- `internal/sos`: SOS lifecycle and broadcast.

## Dispatcher Package Rules

- `features/map` owns MapTalks map lifecycle.
- `features/users` owns user list and presence UI.
- `features/sos` owns SOS alert UI.
- `features/ptt` owns PTT console state.
- `lib/ws.ts` owns reconnect and event dispatch.
- Do not recreate the MapTalks map on every GPS update.

## Shared Types

`packages/shared-types` stores TypeScript event/model definitions used by
dispatcher. Backend remains Go-native, but event names and payload shapes must
match `docs/WEBSOCKET_PROTOCOL.md`.
