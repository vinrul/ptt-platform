#!/usr/bin/env sh
set -eu

set -a
. /etc/ptt-fleet/api-server.env
set +a

exec /opt/ptt-fleet/api-server/api-server
