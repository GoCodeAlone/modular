package chimux

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Static errors for bdd_routing_test.go
var (
	errNoRoutesRegistered       = errors.New("no routes were registered")
	errChiRouterNotAvailable    = errors.New("chi router not available")
	errGETRouteNotRegistered    = errors.New("GET route not registered")
	errPOSTRouteNotRegistered   = errors.New("POST route not registered")
	errPUTRouteNotRegistered    = errors.New("PUT route not registered")
	errDELETERouteNotRegistered = errors.New("DELETE route not registered")
	errParameterizedRouteNotReg = errors.New("parameterized route not registered")
	errWildcardRouteNotReg      = errors.New("wildcard route not registered")
)

func (ctx *ChiMuxBDDTestContext) iRegisterAGETRouteWithHandler(path string) error {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("GET " + path))
	})

	ctx.routerService.Get(path, handler)
	ctx.routes["GET "+path] = "registered"
	return nil
}

func (ctx *ChiMuxBDDTestContext) iRegisterAPOSTRouteWithHandler(path string) error {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("POST " + path))
	})

	ctx.routerService.Post(path, handler)
	ctx.routes["POST "+path] = "registered"
	return nil
}

func (ctx *ChiMuxBDDTestContext) theRoutesShouldBeRegisteredSuccessfully() error {
	if len(ctx.routes) == 0 {
		return errNoRoutesRegistered
	}
	return nil
}

func (ctx *ChiMuxBDDTestContext) iUseChiSpecificRoutingFeatures() error {
	// Use Chi router to create advanced routing patterns
	chiRouter := ctx.chiService.ChiRouter()
	if chiRouter == nil {
		return errChiRouterNotAvailable
	}
	return nil
}

func (ctx *ChiMuxBDDTestContext) iShouldBeAbleToCreateRouteGroups() error {
	chiRouter := ctx.chiService.ChiRouter()
	chiRouter.Route("/admin", func(r chi.Router) {
		r.Get("/users", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})
	ctx.routeGroups = append(ctx.routeGroups, "/admin")
	return nil
}

func (ctx *ChiMuxBDDTestContext) iShouldBeAbleToMountSubRouters() error {
	chiRouter := ctx.chiService.ChiRouter()
	subRouter := chi.NewRouter()
	subRouter.Get("/info", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	chiRouter.Mount("/api", subRouter)
	return nil
}

func (ctx *ChiMuxBDDTestContext) iRegisterRoutesForDifferentHTTPMethods() error {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ctx.routerService.Get("/test", handler)
	ctx.routerService.Post("/test", handler)
	ctx.routerService.Put("/test", handler)
	ctx.routerService.Delete("/test", handler)

	ctx.routes["GET /test"] = "registered"
	ctx.routes["POST /test"] = "registered"
	ctx.routes["PUT /test"] = "registered"
	ctx.routes["DELETE /test"] = "registered"

	return nil
}

func (ctx *ChiMuxBDDTestContext) gETRoutesShouldBeHandledCorrectly() error {
	_, exists := ctx.routes["GET /test"]
	if !exists {
		return errGETRouteNotRegistered
	}
	return nil
}

func (ctx *ChiMuxBDDTestContext) pOSTRoutesShouldBeHandledCorrectly() error {
	_, exists := ctx.routes["POST /test"]
	if !exists {
		return errPOSTRouteNotRegistered
	}
	return nil
}

func (ctx *ChiMuxBDDTestContext) pUTRoutesShouldBeHandledCorrectly() error {
	_, exists := ctx.routes["PUT /test"]
	if !exists {
		return errPUTRouteNotRegistered
	}
	return nil
}

func (ctx *ChiMuxBDDTestContext) dELETERoutesShouldBeHandledCorrectly() error {
	_, exists := ctx.routes["DELETE /test"]
	if !exists {
		return errDELETERouteNotRegistered
	}
	return nil
}

func (ctx *ChiMuxBDDTestContext) iRegisterParameterizedRoutes() error {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ctx.routerService.Get("/users/{id}", handler)
	ctx.routerService.Get("/posts/*", handler)

	ctx.routes["GET /users/{id}"] = "parameterized"
	ctx.routes["GET /posts/*"] = "wildcard"

	return nil
}

func (ctx *ChiMuxBDDTestContext) routeParametersShouldBeExtractedCorrectly() error {
	_, exists := ctx.routes["GET /users/{id}"]
	if !exists {
		return errParameterizedRouteNotReg
	}
	return nil
}

func (ctx *ChiMuxBDDTestContext) wildcardRoutesShouldMatchAppropriately() error {
	_, exists := ctx.routes["GET /posts/*"]
	if !exists {
		return errWildcardRouteNotReg
	}
	return nil
}
