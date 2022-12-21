package http

import (
	"container-agent/job"
	"io/ioutil"
	"net/http"

	"github.com/labstack/echo/v4"
)

func (h *Handler) Register(e *echo.Echo) {
	e.GET("/", h.ok)
	e.GET("/PIDINFO", h.PID)
	e.GET("/PODINFO", h.POD)
	e.GET("/Register", h.RegisterAgent)
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
	pidInfo, err := ioutil.ReadFile("/dist/pidinfo")
	if err != nil {
		panic(err)
		return c.String(http.StatusOK, err.Error())
	}
	return c.String(http.StatusOK, string(pidInfo))
}
func (h *Handler) POD(c echo.Context) error {
	podInfo, err := ioutil.ReadFile("/dist/podinfo")
	if err != nil {
		panic(err)
		return c.String(http.StatusOK, err.Error())
	}
	return c.String(http.StatusOK, string(podInfo))
}
func (h *Handler) RegisterAgent(c echo.Context) error {
	agentinfo := job.RegisterAgent()

	return c.String(http.StatusOK, agentinfo)
}
