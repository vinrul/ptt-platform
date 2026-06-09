package audit

import (
	"reflect"
	"testing"
)

func TestAuditFilters(t *testing.T) {
	where, args := auditFilters(ListInput{
		Action:      "ptt.started",
		ActorUserID: "user-1",
	})

	if where != " WHERE a.action = $1 AND a.actor_user_id = $2" {
		t.Fatalf("unexpected where clause: %s", where)
	}
	if !reflect.DeepEqual(args, []any{"ptt.started", "user-1"}) {
		t.Fatalf("unexpected args: %#v", args)
	}
}
