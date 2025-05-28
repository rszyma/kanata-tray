package controlserver

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/labstack/gommon/log"
	applib "github.com/rszyma/kanata-tray/app"
)

var app *applib.SystrayApp // init in RunControlServer

// All possible status codes: 200, 400, 500

func RunControlServer(app_ *applib.SystrayApp, port int) error {
	app = app_

	mux := chi.NewRouter()

	srv := &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	mux.NotFound(WrapGenericResp(h_notFound))

	mux.HandleFunc("/stop/{preset_name}", WrapGenericResp(h_stopSpecific))
	mux.HandleFunc("/stop_all", WrapGenericResp(h_stopAll))
	mux.HandleFunc("/start/{preset_name}", WrapGenericResp(h_startSpecific))
	mux.HandleFunc("/start_all_default", WrapGenericResp(h_startAllDefault))
	mux.HandleFunc("/toggle/{preset_name}", WrapGenericResp(h_toggleSpecific))
	mux.HandleFunc("/toggle_all_default", WrapGenericResp(h_toggleAllDefault))

	log.Infof("Control server running at %s", srv.Addr)

	return srv.ListenAndServe()
}

func h_notFound[R *struct{}](w http.ResponseWriter, r *http.Request) (_ R, msg string, _ error) {
	return nil, "", fmt.Errorf("unrecognized command / invalid request path")
}

func h_stopSpecific[R *struct{}](w http.ResponseWriter, r *http.Request) (_ R, msg string, _ error) {
	presetName := chi.URLParam(r, "preset_name")
	err := app.StopPreset(presetName)
	if err != nil {
		return nil, "", fmt.Errorf("app.StopPreset: %v", err)
	}
	return nil, "", nil
}

func h_stopAll[R *struct{}](w http.ResponseWriter, r *http.Request) (_ R, msg string, _ error) {
	err := app.StopAllPresets()
	if err != nil {
		return nil, "", fmt.Errorf("app.StopAllPresets: %v", err)
	}
	return nil, "", nil
}

func h_startSpecific[R *struct{}](w http.ResponseWriter, r *http.Request) (_ R, msg string, _ error) {
	presetName := chi.URLParam(r, "preset_name")
	err := app.StartPreset(presetName)
	if err != nil {
		return nil, "", fmt.Errorf("app.StartPreset: %v", err)
	}
	return nil, "", nil
}

func h_startAllDefault[R *struct{}](w http.ResponseWriter, r *http.Request) (_ R, msg string, _ error) {
	err := app.StartAllDefaultPresets()
	if err != nil {
		return nil, "", fmt.Errorf("app.StartAllDefaultPresets: %v", err)
	}
	return nil, "", nil
}

func h_toggleSpecific[R *struct{}](w http.ResponseWriter, r *http.Request) (_ R, msg string, _ error) {
	presetName := chi.URLParam(r, "preset_name")
	msg, err := app.TogglePreset(presetName)
	if err != nil {
		return nil, "", fmt.Errorf("app.TogglePreset: %v", err)
	}
	return nil, msg, nil
}

func h_toggleAllDefault[R *struct{}](w http.ResponseWriter, r *http.Request) (_ R, msg string, _ error) {
	msg, err := app.ToggleAllDefaultPresets()
	if err != nil {
		return nil, "", fmt.Errorf("app.ToggleAllDefaultPresets: %v", err)
	}
	return nil, msg, nil
}
