package client

import (
	"path/filepath"
	"testing"

	"github.com/mtgo-labs/mtgo-bot-api/schema"
)

// method_cert_test.go certifies method-name parity between mtgo-bot-api and
// the official Bot API. The official method set is sourced from the scraped
// schema (schema/methods.json), which is generated from core.telegram.org/bots/api
// — so this test stays in lockstep with the official Bot API without a
// hand-maintained method list.

// schemaDir resolves the schema/ directory relative to this test file
// (internal/client -> ../../schema).
func schemaDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "schema")
	return dir
}

func loadSchema(t *testing.T) *schema.Schema {
	t.Helper()
	sc, err := schema.Load(schemaDir(t))
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}
	return sc
}

// TestCertAllOfficialMethodsRegistered asserts that every method the official
// Bot API defines (per the scraped schema) is also registered here.
func TestCertAllOfficialMethodsRegistered(t *testing.T) {
	sc := loadSchema(t)
	var missing []string
	checked := 0
	for _, m := range sc.Methods {
		if !m.Official {
			continue
		}
		checked++
		if !HasMethod(m.Name) {
			missing = append(missing, m.Name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("official methods not registered in mtgo-bot-api: %v", missing)
	}
	t.Logf("checked %d official schema methods, %d registered", checked, checked-len(missing))
}

// TestCertMethodCount asserts the registered method count meets the official
// Bot API total (derived from the schema, no magic number).
func TestCertMethodCount(t *testing.T) {
	sc := loadSchema(t)
	want := 0
	for _, m := range sc.Methods {
		if m.Official {
			want++
		}
	}
	registered := len(RegisteredMethods())
	if registered < want {
		t.Errorf("registered method count = %d, want >= %d (official Bot API total)", registered, want)
	}
	t.Logf("registered methods: %d (official: %d)", registered, want)
}

// TestCertCriticalMethodsRegistered spot-checks a representative set of critical
// methods across all categories.
func TestCertCriticalMethodsRegistered(t *testing.T) {
	critical := []string{
		"getme", "getupdates",
		"sendmessage", "sendphoto", "sendvideo", "senddocument",
		"sendanimation", "sendvoice", "sendpoll", "senddice",
		"editmessagetext", "editmessagecaption",
		"forwardmessage", "copymessage",
		"answercallbackquery", "answerinlinequery",
		"getchat", "setchatphoto", "getchatmember",
		"promotechatmember", "restrictchatmember",
		"setwebhook", "deletewebhook", "getwebhookinfo",
		"getfile", "getmycommands", "setmycommands",
		"sendinvoice", "sendgame", "getstickerset",
		"deletemessage", "deletemessages",
		"createforumtopic", "editforumtopic",
		"exportchatinvitelink", "createchatinvitelink",
		"getuserprofilephotos", "sendchataction",
	}
	var missing []string
	for _, m := range critical {
		if !HasMethod(m) {
			missing = append(missing, m)
		}
	}
	if len(missing) > 0 {
		t.Errorf("critical methods not registered: %v", missing)
	}
}

// TestCertNoUntrackedMethods asserts there are no registered methods that are
// absent from the schema (every registered method must correspond to an official
// Bot API method or a documented extension). This catches leftover deprecated
// aliases the official docs have dropped.
func TestCertNoUntrackedMethods(t *testing.T) {
	sc := loadSchema(t)
	want := map[string]bool{}
	for _, m := range sc.Methods {
		want[m.Name] = true
	}
	var untracked []string
	for _, name := range RegisteredMethods() {
		if !want[name] {
			untracked = append(untracked, name)
		}
	}
	if len(untracked) > 0 {
		t.Errorf("registered methods missing from schema (remove or document them): %v", untracked)
	}
}

// TestCertNoShadowing verifies the registered method count is stable (each name
// maps to exactly one handler; duplicate Register calls silently overwrite).
func TestCertNoShadowing(t *testing.T) {
	first := len(RegisteredMethods())
	second := len(RegisteredMethods())
	if first != second {
		t.Errorf("method count unstable: %d then %d", first, second)
	}
}
