package echo

import (
	"github.com/jdudmesh/hotreload-go/pkg/hotreloader"
	"github.com/jdudmesh/hotreload-go/pkg/res"
	"github.com/labstack/echo/v4"
)

type hotreloadMiddleware struct {
	*hotreloader.HotReloader
}

func New(opts ...hotreloader.HotReloaderOption) (*hotreloadMiddleware, error) {
	base, err := hotreloader.New(opts...)
	if err != nil {
		return nil, err
	}

	middleware := &hotreloadMiddleware{
		HotReloader: base,
	}

	return middleware, nil
}

func (m *hotreloadMiddleware) Handler(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		switch c.Request().URL.Path {
		case "/hotreload-go/reload.js":
			return m.handleScriptRequestfunc(c)
		case "/hotreload-go/ws":
			return m.handleWebsocketRequestfunc(c)
		default:
			return next(c)
		}
	}
}

func (m *hotreloadMiddleware) handleScriptRequestfunc(c echo.Context) error {
	c.Response().Header().Set("Content-Type", "application/javascript")
	if !m.IsHotReloadEnabled() {
		return c.String(200, "// hot reload disabled")
	}
	return c.String(200, res.ReloadScript)
}

func (m *hotreloadMiddleware) handleWebsocketRequestfunc(c echo.Context) error {
	m.WebSocketHandler().ServeHTTP(c.Response(), c.Request())
	return nil
}
