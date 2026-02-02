package pprof

import (
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
	if r == nil {
		return
	}
	r.Get(specs.PprofIndex, pp.Index)
	r.Get(specs.PprofCmdline, pp.Cmdline)
	r.Get(specs.PprofProfile, pp.Profile)
	r.Get(specs.PprofSymbol, pp.Symbol)
	r.Get(specs.PprofTrace, pp.Trace)
}
