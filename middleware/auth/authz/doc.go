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
package authz
