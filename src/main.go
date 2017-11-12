// +build appengine
package main

import (
	"net/http"

	"github.com/labstack/echo"
	"github.com/labstack/echo/engine/standard"
)

func init() {
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	e.GET("/api/adventar/:calid", AdventarHandler)
}

func createMux() *echo.Echo {
	e := echo.New()

	s := standard.New("")
	s.SetHandler(e)

	http.Handle("/", s)

	return e
}
