package http

import (
	"container-agent/module"
	"net/http"

	"github.com/labstack/echo/v4"
)

func (h *Handler) Register(e *echo.Echo) {
	e.GET("/", h.ok)
	e.GET("/PIDINFO", h.PID)
	e.GET("/PODINFO", h.FileSystemInfo)
	// // accounts
	// accounts := e.Group("/accounts")
	// accounts.GET("", h.getAccounts)
	// accounts.POST("", h.createAccount)
	// accounts.PUT("", h.updateAccount)
	// accounts.GET("/:accountID", h.getAccountByID)
	// accounts.POST("/:accountID/discover", h.discoverAccountByID)
	// accounts.POST("/:accountID/scan", h.scanAccountByID)

	// // discovers
	// discovers := e.Group("/discovers")
	// discovers.GET("/:discoverID/status", h.getDiscoverStatusByID)
	// discovers.GET("/:discoverID", h.getDiscoverByID)
}

func (h *Handler) ok(c echo.Context) error {
	return c.String(http.StatusOK, "OK\n")
}
func (h *Handler) PID(c echo.Context) error {
	return c.String(http.StatusOK, module.ProcessInfoJsonData)
}
func (h *Handler) FileSystemInfo(c echo.Context) error {
	return c.String(http.StatusOK, module.FileListJsonMerged)
}
