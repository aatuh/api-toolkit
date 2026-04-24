package specs

import (
	"reflect"
	"testing"
)

func TestEndpointConstants(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Livez", got: Livez, want: "/livez"},
		{name: "Readyz", got: Readyz, want: "/readyz"},
		{name: "Healthz", got: Healthz, want: "/healthz"},
		{name: "Health", got: Health, want: "/health"},
		{name: "HealthDetailed", got: HealthDetailed, want: "/health/detailed"},
		{name: "Docs", got: Docs, want: "/docs"},
		{name: "DocsOpenAPI", got: DocsOpenAPI, want: "/docs/openapi.json"},
		{name: "DocsVersion", got: DocsVersion, want: "/docs/version"},
		{name: "DocsInfo", got: DocsInfo, want: "/docs/info"},
		{name: "Version", got: Version, want: "/version"},
		{name: "Metrics", got: Metrics, want: "/metrics"},
		{name: "PprofIndex", got: PprofIndex, want: "/debug/pprof/"},
		{name: "PprofCmdline", got: PprofCmdline, want: "/debug/pprof/cmdline"},
		{name: "PprofProfile", got: PprofProfile, want: "/debug/pprof/profile"},
		{name: "PprofSymbol", got: PprofSymbol, want: "/debug/pprof/symbol"},
		{name: "PprofTrace", got: PprofTrace, want: "/debug/pprof/trace"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestGroupedEndpointsMatchConstants(t *testing.T) {
	if HealthEndpoints.Livez != Livez ||
		HealthEndpoints.Readyz != Readyz ||
		HealthEndpoints.Healthz != Healthz ||
		HealthEndpoints.Health != Health ||
		HealthEndpoints.HealthDetailed != HealthDetailed {
		t.Fatalf("HealthEndpoints = %#v", HealthEndpoints)
	}

	if DocsEndpoints.Docs != Docs ||
		DocsEndpoints.OpenAPI != DocsOpenAPI ||
		DocsEndpoints.Version != DocsVersion ||
		DocsEndpoints.Info != DocsInfo {
		t.Fatalf("DocsEndpoints = %#v", DocsEndpoints)
	}

	if SystemEndpoints.Version != Version {
		t.Fatalf("SystemEndpoints = %#v", SystemEndpoints)
	}

	if PprofEndpoints.Index != PprofIndex ||
		PprofEndpoints.Cmdline != PprofCmdline ||
		PprofEndpoints.Profile != PprofProfile ||
		PprofEndpoints.Symbol != PprofSymbol ||
		PprofEndpoints.Trace != PprofTrace {
		t.Fatalf("PprofEndpoints = %#v", PprofEndpoints)
	}

	if !reflect.DeepEqual(AllEndpoints.Health, HealthEndpoints) {
		t.Fatalf("AllEndpoints.Health = %#v, want %#v", AllEndpoints.Health, HealthEndpoints)
	}
	if !reflect.DeepEqual(AllEndpoints.Docs, DocsEndpoints) {
		t.Fatalf("AllEndpoints.Docs = %#v, want %#v", AllEndpoints.Docs, DocsEndpoints)
	}
	if !reflect.DeepEqual(AllEndpoints.System, SystemEndpoints) {
		t.Fatalf("AllEndpoints.System = %#v, want %#v", AllEndpoints.System, SystemEndpoints)
	}
}
