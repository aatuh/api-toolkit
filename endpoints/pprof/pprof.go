package pprof

import (
	"errors"
	"net/http"
	pp "net/http/pprof"

	"github.com/aatuh/api-toolkit/v2/specs"
)

// Router defines the minimal GET registration contract needed for pprof routes.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
}

// RegisterRoutes wires the default pprof handlers on the provided router.
func RegisterRoutes(r Router) {
	registerRoutes(r, nil)
}

// RegisterAdminRoutes wires pprof handlers behind an explicit authorization or
// internal-network wrapper. Passing nil fails closed so admin-only mounts cannot
// accidentally expose pprof without policy.
func RegisterAdminRoutes(r Router, requireAdmin func(http.Handler) http.Handler) error {
	if requireAdmin == nil {
		return errors.New("pprof admin routes require an authorization wrapper")
	}
	registerRoutes(r, requireAdmin)
	return nil
}

func registerRoutes(r Router, wrap func(http.Handler) http.Handler) {
	if r == nil {
		return
	}
	r.Get(specs.PprofIndex, wrapHandler(pp.Index, wrap))
	r.Get(specs.PprofCmdline, wrapHandler(pp.Cmdline, wrap))
	r.Get(specs.PprofProfile, wrapHandler(pp.Profile, wrap))
	r.Get(specs.PprofSymbol, wrapHandler(pp.Symbol, wrap))
	r.Get(specs.PprofTrace, wrapHandler(pp.Trace, wrap))
}

func wrapHandler(h http.HandlerFunc, wrap func(http.Handler) http.Handler) http.HandlerFunc {
	if wrap == nil {
		return h
	}
	return wrap(h).ServeHTTP
}
