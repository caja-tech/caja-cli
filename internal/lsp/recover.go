package lsp

import (
	"log/slog"
	"os"
	"runtime/debug"

	"github.com/owenrumney/go-lsp/lsp"
)

// strictPanics makes the recover helpers re-panic instead of swallowing. Tests set
// CAJA_LSP_STRICT so a panic surfaces as a test failure with its original stack rather
// than being silently absorbed by the production degradation path.
var strictPanics = os.Getenv("CAJA_LSP_STRICT") != ""

// onRecover, when set, is notified of every recovered panic. It exists so tests can
// observe degradation that is deliberately invisible in production: the recover helpers
// turn a panic into an empty result and a nil error, which a caller cannot distinguish
// from "nothing here". Nil outside tests.
var onRecover func(rec any, op string, uri lsp.DocumentURI, stack []byte)

// logRecovered reports a recovered panic on stderr. stdout is the JSON-RPC transport for
// stdio-based LSP, so every diagnostic write in this package must go to stderr —
// slog's default handler does. Never route this to stdout.
func logRecovered(rec any, op string, uri lsp.DocumentURI) {
	stack := debug.Stack()

	slog.Error("recovered from panic",
		"op", op,
		"uri", string(uri),
		"panic", rec,
		"stack", string(stack),
	)

	if onRecover != nil {
		onRecover(rec, op, uri, stack)
	}
}

// recoverWorker absorbs a panic raised while validating a document. It is deferred once
// per worker-loop iteration, never around the loop itself: recovering around the loop
// would terminate the goroutine, and because DidChange drains the size-1 channel before
// sending, the sender would never block — so the document would silently stop updating
// forever. Per-iteration recovery costs one failed validation instead.
func recoverWorker(op string, uri lsp.DocumentURI) {
	rec := recover()
	if rec == nil {
		return
	}
	logRecovered(rec, op, uri)
	if strictPanics {
		panic(rec)
	}
}

// recoverInto absorbs a panic raised while answering a request, zeroing the named result
// so the handler replies "no result" instead of an error. The go-lsp transport already
// recovers panics and replies CodeInternalError, but that surfaces in the editor as a
// user-visible error toast on every keystroke; an empty result degrades quietly.
func recoverInto[T any](op string, uri lsp.DocumentURI, res *T, err *error) {
	rec := recover()
	if rec == nil {
		return
	}
	logRecovered(rec, op, uri)
	if strictPanics {
		panic(rec)
	}
	var zero T
	*res = zero
	*err = nil
}
