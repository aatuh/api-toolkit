// Package chi adapts the chi router to api-toolkit HTTP ports.
//
// Route bootstrap validation
// --------------------------
//
// Use authz route bootstrap validation during startup to ensure route-level
// authz coverage is complete before serving traffic.
//
// Example:
//
//	router := chi.New()
//	adminMw, err := authz.NewRequireRoleMiddlewareChecked("admin", roleResolver)
//	if err != nil {
//		return fmt.Errorf("admin authz middleware: %w", err)
//	}
//	opsMw, err := authz.NewRequireRoleMiddlewareChecked("ops", roleResolver)
//	if err != nil {
//		return fmt.Errorf("ops authz middleware: %w", err)
//	}
//	router.Get("/admin", adminHandler)
//	router.Post("/ops", opsHandler)
//	router.Get("/public", publicHandler)
//
//	if err := chiAdapter.ValidateRequireRoleMiddlewareRoutes(router, func(method, route string, _ http.Handler) *authz.RequireRoleMiddleware {
//		switch route {
//		case "/admin":
//			return adminMw
//		case "/ops":
//			return opsMw
//		case "/public":
//			return nil
//		}
//		return nil
//	}); err != nil {
//		return fmt.Errorf("route contract scan failed: %w", err)
//	}
//
//	// Use strict-by-default startup bootstrap policy for mixed-route rollouts in CI
//	// and production-like deployments.
//	if err := chiAdapter.ValidateRequireRoleMiddlewareRoutesAuto(router, func(method, route string, _ http.Handler) *authz.RequireRoleMiddleware {
//		switch route {
//		case "/admin", "/ops":
//			return adminMw
//		case "/public":
//			return nil
//		}
//		return nil
//	}); err != nil {
//		return fmt.Errorf("route bootstrap strict-policy check failed: %w", err)
//	}
//
//	// Override bootstrap strictness explicitly from CI or environment signals:
//	// API_TOOLKIT_AUTHZ_BOOTSTRAP_STRICT=1|true|strict|enabled
//	// API_TOOLKIT_AUTHZ_BOOTSTRAP_STRICT=0|false|permissive|disabled
//
//	// Mixed public/protected routes can be validated with explicit coverage
//	// control when strict coverage is desired only for protected routes.
//	if err := chiAdapter.ValidateRequireRoleMiddlewareRoutesWithCoverage(
//		router,
//		func(method, route string, _ http.Handler) MiddlewareSpecResolution {
//			switch route {
//			case "/admin", "/ops":
//				return MiddlewareSpecResolution{Middleware: adminMw}
//			case "/public":
//				return MiddlewareSpecResolution{SkipFromValidation: true}
//			default:
//				return MiddlewareSpecResolution{}
//			}
//		},
//	); err != nil {
//		return fmt.Errorf("route contract scan with mixed coverage failed: %w", err)
//	}
//
//	// Recommended rollout policy:
//	// - Use ValidateRequireRoleMiddlewareRoutesAuto as the default startup policy.
//	// - Keep strict mode enabled in CI/production-like environments via
//	//   API_TOOLKIT_AUTHZ_BOOTSTRAP_STRICT or explicit strict profile signals.
//	// - Use MiddlewareSpecResolution SkipFromValidation for explicit public routes while
//	//   retaining strict validation for protected routes.
//
//	if err := chiAdapter.ValidateRequireRoleMiddlewareRoutesStrict(router, func(method, route string, _ http.Handler) *authz.RequireRoleMiddleware {
//		if route == "/public" {
//			return nil
//		}
//		return adminMw
//	}); err != nil {
//		return fmt.Errorf("strict route contract scan failed: %w", err)
//	}
package chi
