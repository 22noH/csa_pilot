package http

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
)

type Server struct {
	e *echo.Echo
	h *Handler
}

func Start(listenPort uint16, logLevel log.Lvl) (*Server, error) {
	s := &Server{}

	s.e = newEcho(logLevel)
	if s.e == nil {
		return nil, fmt.Errorf("failed to create new echo")
	}

	s.h = NewHandler()
	if s.h == nil {
		return nil, fmt.Errorf("failed to create new handler")
	}
	s.h.Register(s.e)

	address := fmt.Sprintf(":%d", listenPort)
	go func() {
		err := s.e.Start(address)
		if err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
			os.Exit(100)
		}
	}()

	return s, nil
}

func (s *Server) Stop() {
	if s == nil {
		return
	}

	if s.h != nil {
		s.h.Close()
		s.h = nil
	}

	if s.e != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.e.Shutdown(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		}
		s.e = nil
	}
}

func newEcho(logLevel log.Lvl) *echo.Echo {
	e := echo.New()
	if e == nil {
		return nil
	}
	e.HideBanner = true
	e.Logger.SetLevel(logLevel)
	e.Pre(middleware.RemoveTrailingSlash())
	e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		StackSize: 1 << 10, // 1 KB
		LogLevel:  log.ERROR,
	}))
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: `${time_rfc3339} HTTP "${method} ${uri} ${protocol}" ${status} ` +
			`${bytes_in} ${bytes_out} ${latency} "${remote_ip}" "${user_agent}" "${id}" "${error}"` + "\n",
	}))
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		AllowMethods: []string{echo.GET, echo.HEAD, echo.PUT, echo.PATCH, echo.POST, echo.DELETE},
	}))
	e.Validator = newValidator()
	return e
}

// 참고: https://github.com/go-playground/validator/
// 참고: https://echo.labstack.com/guide/request/#validate-data
type Validator struct {
	validator *validator.Validate
}

func newValidator() *Validator {
	return &Validator{validator: validator.New()}
}

func (v *Validator) Validate(i interface{}) error {
	if err := v.validator.Struct(i); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}
