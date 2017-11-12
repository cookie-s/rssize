// +build appengine
package main

import (
	"net/http"

	"github.com/labstack/echo"
	"github.com/labstack/echo/engine/standard"
)

func init() {
	e.GET("/hello", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello World")
	})
}

func createMux() *echo.Echo {
	e := echo.New()

	s := standard.New("")
	s.SetHandler(e)

	http.Handle("/", s)

	return e
}
