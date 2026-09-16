package admin

import (
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestAdminServicesRetainInjectedLoggerAndKeepDefaultForNil(t *testing.T) {
	injected := slog.New(slog.NewTextHandler(io.Discard, nil))
	authService := NewAdminAuthService(nil, nil, nil, time.Minute, time.Hour)
	userService := NewAdminUserService(nil, nil, time.Minute)

	authService.SetLogger(injected)
	userService.SetLogger(injected)

	if authService.logger != injected {
		t.Fatal("admin auth service did not retain the injected logger")
	}
	if userService.logger != injected {
		t.Fatal("admin user service did not retain the injected logger")
	}

	authService.SetLogger(nil)
	userService.SetLogger(nil)
	if authService.logger != injected || userService.logger != injected {
		t.Fatal("nil logger injection must preserve the existing safe logger")
	}
}
