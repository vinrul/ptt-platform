# Database Design

Database utama adalah PostgreSQL. Migration menggunakan goose.

## Extensions

Recommended:

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;
```

UUID bisa dibuat oleh aplikasi atau database. Untuk konsistensi MVP, aplikasi
boleh membuat UUID, dan database tetap menyediakan default `gen_random_uuid()`.

## users

Menyimpan akun.

Columns:

- `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
- `username TEXT NOT NULL UNIQUE`
- `password_hash TEXT NOT NULL`
- `full_name TEXT NOT NULL`
- `role TEXT NOT NULL`
- `status TEXT NOT NULL DEFAULT 'active'`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`

Roles:

- `super_admin`
- `dispatcher`
- `supervisor`
- `field_user`

Status:

- `active`
- `disabled`

## devices

Menyimpan perangkat login.

Columns:

- `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
- `user_id UUID NOT NULL REFERENCES users(id)`
- `device_name TEXT NOT NULL`
- `device_imei TEXT`
- `platform TEXT NOT NULL`
- `status TEXT NOT NULL DEFAULT 'active'`
- `last_seen_at TIMESTAMPTZ`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`

Platforms:

- `android`
- `web`

Indexes:

- `(user_id)`
- `(last_seen_at DESC)`

## groups

Menyimpan channel/grup PTT.

Columns:

- `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
- `name TEXT NOT NULL UNIQUE`
- `description TEXT NOT NULL DEFAULT ''`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`

## group_members

Menyimpan membership user di grup.

Columns:

- `group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE`
- `user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE`
- `role_in_group TEXT NOT NULL DEFAULT 'member'`
- `joined_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- `PRIMARY KEY (group_id, user_id)`

Roles:

- `member`
- `dispatcher`
- `supervisor`

Indexes:

- `(user_id)`

## gps_logs

Menyimpan lokasi user.

Columns:

- `id BIGSERIAL PRIMARY KEY`
- `user_id UUID NOT NULL REFERENCES users(id)`
- `lat DOUBLE PRECISION NOT NULL`
- `lng DOUBLE PRECISION NOT NULL`
- `speed DOUBLE PRECISION`
- `heading DOUBLE PRECISION`
- `accuracy DOUBLE PRECISION`
- `recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()`

Indexes:

- `(user_id, recorded_at DESC)`
- `(recorded_at DESC)`

Validation:

- `lat` between -90 and 90.
- `lng` between -180 and 180.

## sos_events

Menyimpan emergency event.

Columns:

- `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
- `user_id UUID NOT NULL REFERENCES users(id)`
- `lat DOUBLE PRECISION`
- `lng DOUBLE PRECISION`
- `message TEXT NOT NULL DEFAULT ''`
- `status TEXT NOT NULL DEFAULT 'open'`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- `acknowledged_by UUID REFERENCES users(id)`
- `acknowledged_at TIMESTAMPTZ`
- `resolved_at TIMESTAMPTZ`

Status:

- `open`
- `ack`
- `resolved`

Indexes:

- `(status, created_at DESC)`
- `(user_id, created_at DESC)`

## talk_sessions

Menyimpan metadata sesi bicara.

Columns:

- `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
- `group_id UUID NOT NULL REFERENCES groups(id)`
- `speaker_user_id UUID NOT NULL REFERENCES users(id)`
- `started_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- `ended_at TIMESTAMPTZ`
- `duration_ms BIGINT`
- `stop_reason TEXT`

Stop reasons:

- `user_stop`
- `disconnect`
- `timeout`
- `server_error`

Indexes:

- `(group_id, started_at DESC)`
- `(speaker_user_id, started_at DESC)`

## refresh_tokens

Menyimpan refresh token yang bisa di-rotate dan revoke.

Columns:

- `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
- `user_id UUID NOT NULL REFERENCES users(id)`
- `device_id UUID REFERENCES devices(id)`
- `token_hash TEXT NOT NULL UNIQUE`
- `expires_at TIMESTAMPTZ NOT NULL`
- `revoked_at TIMESTAMPTZ`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`

Indexes:

- `(user_id)`
- `(expires_at)`

## audit_logs

Menyimpan log aktivitas penting.

Columns:

- `id BIGSERIAL PRIMARY KEY`
- `actor_user_id UUID REFERENCES users(id)`
- `action TEXT NOT NULL`
- `entity_type TEXT NOT NULL`
- `entity_id TEXT`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`

Indexes:

- `(actor_user_id, created_at DESC)`
- `(action, created_at DESC)`
- `(created_at DESC)`

Initial actions:

- `auth.login_success`
- `auth.login_failed`
- `auth.logout`
- `user.created`
- `user.updated`
- `group.created`
- `group.member_added`
- `sos.created`
- `sos.acked`
- `ptt.started`
- `ptt.stopped`

## Seed Data

Local seed target:

- Super admin: `admin`.
- Dispatcher: `dispatcher1`.
- Field user: `field1`.
- Field user: `field2`.
- Group: `Default Patrol`.
- `field1`, `field2`, and `dispatcher1` are members of `Default Patrol`.
