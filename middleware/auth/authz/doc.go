// Package authz provides role-based authorization middleware.
//
// NewRequireRoleMiddleware keeps the compatibility-oriented single-return
// constructor shape. Invalid configuration fails closed at request time. Use
// NewRequireRoleMiddlewareChecked when application startup should fail fast on
// an empty required role or nil RolesFromContext resolver.
//
// During route wiring, use ValidateRequireRoleMiddleware(method, route, middleware)
// or ValidateRequireRoleMiddlewareRoutes for startup checks over route registries.
//
// Route registry recommendation:
//
//	checks := []authz.RequireRoleRouteSpec{
//	    {Method: http.MethodGet, Route: "/admin", Middleware: adminMw},
//	    {Method: http.MethodPost, Route: "/billing", Middleware: billingMw},
//	}
//	if err := authz.ValidateRequireRoleMiddlewareRoutes(checks); err != nil {
//	    return fmt.Errorf("route contract scan failed: %w", err)
//	}
//
// chi route bootstrap helper:
//
//	if err := chiAdapter.ValidateRequireRoleMiddlewareRoutes(router.Mux, func(method, route string, _ http.Handler) *authz.RequireRoleMiddleware {
//	    switch route {
//	    case "/admin":
//	        return adminMw
//	    }
//	    return nil
//	}); err != nil {
//	    return fmt.Errorf("route contract scan failed: %w", err)
//	}
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/middleware/auth/authz`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package authz
