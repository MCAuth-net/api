package helpers

import (
	"fmt"
	"slices"
	"strings"

	"github.com/gofiber/fiber/v3"
)

const (
	RoutesGroupMWKey = "__group_mw"
	Limiter          = "__limiter"
	NeedsAuth        = "__needs_auth"
)

var httpMethods = []string{
	fiber.MethodGet,
	fiber.MethodHead,
	fiber.MethodPost,
	fiber.MethodPut,
	fiber.MethodPatch,
	fiber.MethodDelete,
	fiber.MethodConnect,
	fiber.MethodOptions,
	fiber.MethodTrace,
}

func Get(handlers any) fiber.Map    { return fiber.Map{fiber.MethodGet: handlers} }
func Post(handlers any) fiber.Map   { return fiber.Map{fiber.MethodPost: handlers} }
func Put(handlers any) fiber.Map    { return fiber.Map{fiber.MethodPut: handlers} }
func Patch(handlers any) fiber.Map  { return fiber.Map{fiber.MethodPatch: handlers} }
func Delete(handlers any) fiber.Map { return fiber.Map{fiber.MethodDelete: handlers} }

type routeState struct {
	limiter     fiber.Handler
	needsAuth   bool
	middlewares []fiber.Handler
}

func BuildRoutes(router fiber.Router, routes fiber.Map, auth fiber.Handler) {
	buildRoutes(router, "", routes, routeState{}, auth)
}

func buildRoutes(router fiber.Router, path string, node fiber.Map, state routeState, auth fiber.Handler) {
	if v, ok := node[Limiter]; ok {
		if v == nil {
			state.limiter = nil
		} else {
			h, ok := v.(fiber.Handler)
			if !ok {
				panic(fmt.Sprintf("routes: %q: Limiter must be a fiber.Handler or nil, got %T", path, v))
			}
			state.limiter = h
		}
	}

	if v, ok := node[NeedsAuth]; ok {
		b, ok := v.(bool)
		if !ok {
			panic(fmt.Sprintf("routes: %q: NeedsAuth must be a bool, got %T", path, v))
		}
		state.needsAuth = b
	}

	if v, ok := node[RoutesGroupMWKey]; ok {
		mws, ok := v.([]fiber.Handler)
		if !ok {
			panic(fmt.Sprintf("routes: %q: RoutesGroupMWKey must be a []fiber.Handler, got %T", path, v))
		}
		state.middlewares = append(slices.Clone(state.middlewares), mws...)
	}

	keys := make([]string, 0, len(node))
	for key := range node {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b string) int {
		aParam, bParam := strings.HasPrefix(a, ":"), strings.HasPrefix(b, ":")
		if aParam != bParam {
			if aParam {
				return 1
			}
			return -1
		}
		return strings.Compare(a, b)
	})

	for _, key := range keys {
		value := node[key]

		switch {
		case key == Limiter, key == NeedsAuth, key == RoutesGroupMWKey:

		case slices.Contains(httpMethods, key):
			registerRoute(router, key, path, value, state, auth)

		default:
			child, ok := value.(fiber.Map)
			if !ok {
				panic(fmt.Sprintf("routes: %q: %q must be a fiber.Map, got %T", path, key, value))
			}
			buildRoutes(router, path+"/"+key, child, state, auth)
		}
	}
}

func registerRoute(router fiber.Router, method, path string, value any, state routeState, auth fiber.Handler) {
	if path == "" {
		path = "/"
	}

	var handlers []fiber.Handler
	switch h := value.(type) {
	case fiber.Handler:
		handlers = []fiber.Handler{h}
	case []fiber.Handler:
		handlers = h
	default:
		panic(fmt.Sprintf("routes: %s %s: handler must be a fiber.Handler or []fiber.Handler, got %T", method, path, value))
	}
	if len(handlers) == 0 {
		panic(fmt.Sprintf("routes: %s %s: no handlers", method, path))
	}

	// order: limiter -> auth -> group middlewares -> route handlers
	chain := make([]any, 0, len(handlers)+len(state.middlewares)+2)
	if state.limiter != nil {
		chain = append(chain, state.limiter)
	}
	if state.needsAuth {
		if auth == nil {
			panic(fmt.Sprintf("routes: %s %s: NeedsAuth is true but no auth middleware was given", method, path))
		}
		chain = append(chain, auth)
	}
	for _, mw := range state.middlewares {
		chain = append(chain, mw)
	}
	for _, h := range handlers {
		chain = append(chain, h)
	}

	router.Add([]string{method}, path, chain[0], chain[1:]...)
}
