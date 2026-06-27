package client

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// businessConnIDHandlers are the method handlers that officially accept an optional
// business_connection_id and must route the call through invokeWithBusinessConnection
// when it is present. Each handler body must read the param via businessConnID(q).
// Guards the P0 cluster in docs/parameter-parity-gaps.md against regression.
var businessConnIDHandlers = []string{
	// Phase A — true-return
	"sendChatAction", "pinChatMessage", "unpinChatMessage",
	"convertGiftToStars", "transferGift", "upgradeGift", "deleteStory",
	// Phase B — scalar
	"createInvoiceLink",
	// Phase C — Message
	"editMessageText", "editMessageCaption", "editMessageMedia", "editMessageReplyMarkup",
	"editMessageLiveLocation", "editMessageChecklist", "stopPoll", "stopMessageLiveLocation",
	// Phase D — []Message
	"sendMediaGroup",
}

// TestBusinessConnIDHandlersReadParam asserts every handler in businessConnIDHandlers
// references the business_connection_id parameter (via businessConnID(q)), so the param
// cannot be silently dropped again.
func TestBusinessConnIDHandlersReadParam(t *testing.T) {
	bodies := handlerBodies(t) // map[method name]body source

	var missing []string
	for _, name := range businessConnIDHandlers {
		body, ok := bodies[name]
		if !ok {
			t.Errorf("handler %q not found in client package", name)
			continue
		}
		if !strings.Contains(body, "businessConnID(q)") && !strings.Contains(body, "requireBusinessConn(") {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("handlers do not read business_connection_id (businessConnID(q) missing): %v", missing)
	} else {
		t.Logf("all %d handlers read business_connection_id", len(businessConnIDHandlers))
	}
}

// handlerBodies parses the client package and returns each *Client method's body source.
func handlerBodies(t *testing.T) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read client package: %v", err)
	}
	out := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse client package file %s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Recv == nil || len(fn.Recv.List) == 0 {
				return true
			}
			// receiver must be *Client
			star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				return true
			}
			if ident, ok := star.X.(*ast.Ident); !ok || ident.Name != "Client" {
				return true
			}
			out[fn.Name.Name] = nodeSource(fset, fn.Body)
			return true
		})
	}
	return out
}

// nodeSource renders an AST node back to its source text by reading the file
// and slicing by token offsets from the same FileSet used to parse it.
func nodeSource(fset *token.FileSet, n ast.Node) string {
	start, end := fset.Position(n.Pos()), fset.Position(n.End())
	b, err := os.ReadFile(start.Filename)
	if err != nil {
		return ""
	}
	if end.Offset > len(b) {
		end.Offset = len(b)
	}
	if start.Offset > end.Offset {
		return ""
	}
	return string(b[start.Offset:end.Offset])
}
