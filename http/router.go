package http

import (
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"sync"

	"github.com/go-chi/chi"
	platform "github.com/influxdata/influxdb"
	influxlogger "github.com/influxdata/influxdb/logger"
	"github.com/julienschmidt/httprouter"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewRouter returns a new router with a 404 handler, a 405 handler, and a panic handler.
func NewRouter(h platform.HTTPErrorHandler) *httprouter.Router {
	b := baseHandler{HTTPErrorHandler: h}
	router := httprouter.New()
	router.NotFound = http.HandlerFunc(b.notFound)
	router.MethodNotAllowed = http.HandlerFunc(b.methodNotAllowed)
	router.PanicHandler = b.panic
	return router
}

func newBaseChiRouter(errorHandler platform.HTTPErrorHandler) chi.Router {
	router := chi.NewRouter()
	bh := baseHandler{HTTPErrorHandler: errorHandler}
	router.NotFound(bh.notFound)
	router.MethodNotAllowed(bh.methodNotAllowed)
	return router
}

type baseHandler struct {
	platform.HTTPErrorHandler
}

// notFound represents a 404 handler that return a JSON response.
func (h baseHandler) notFound(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pe := &platform.Error{
		Code: platform.ENotFound,
		Msg:  "path not found",
	}

	h.HandleHTTPError(ctx, pe, w)
}

// methodNotAllowed represents a 405 handler that return a JSON response.
func (h baseHandler) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	allow := w.Header().Get("Allow")
	pe := &platform.Error{
		Code: platform.EMethodNotAllowed,
		Msg:  fmt.Sprintf("allow: %s", allow),
	}

	h.HandleHTTPError(ctx, pe, w)
}

// panic handles panics recovered from http handlers.
// It returns a json response with http status code 500 and the recovered error message.
func (h baseHandler) panic(w http.ResponseWriter, r *http.Request, rcv interface{}) {
	ctx := r.Context()
	pe := &platform.Error{
		Code: platform.EInternal,
		Msg:  "a panic has occurred",
		Err:  fmt.Errorf("%v", rcv),
	}

	l := getPanicLogger()
	if entry := l.Check(zapcore.ErrorLevel, pe.Msg); entry != nil {
		entry.Stack = string(debug.Stack())
		entry.Write(zap.Error(pe.Err))
	}

	h.HandleHTTPError(ctx, pe, w)
}

var panicLogger *zap.Logger
var panicLoggerOnce sync.Once

// getPanicLogger returns a logger for panicHandler.
func getPanicLogger() *zap.Logger {
	panicLoggerOnce.Do(func() {
		panicLogger = influxlogger.New(os.Stderr)
		panicLogger = panicLogger.With(zap.String("handler", "panic"))
	})

	return panicLogger
}
