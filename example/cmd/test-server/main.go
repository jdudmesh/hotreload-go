package main

import (
	"net/http"

	echoreloader "github.com/jdudmesh/hotreload-go/pkg/echo"
	"github.com/jdudmesh/hotreload-go/pkg/hotreloader"
	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()
	e.Static("/assets", "./static")

	middleware, err := echoreloader.New(
		hotreloader.WithStaticFilePath("./static"),
		hotreloader.WithTemplatePathGlob("./templates/*.html"),
		hotreloader.WithLogger(e.Logger),
	)

	if err != nil {
		e.Logger.Fatal(err)
	}
	defer middleware.Close()

	e.Use(middleware.Handler)
	e.Renderer = middleware

	e.GET("/", func(c echo.Context) error {
		return c.Render(http.StatusOK, "index.html", nil)
	})

	e.GET("/api/test", func(c echo.Context) error {
		return c.Render(http.StatusOK, "fragment.html", struct {
			Items []string
		}{
			Items: []string{"item1", "item2", "item3"},
		})
	})

	e.Logger.Fatal(e.Start(":8080"))
}
