package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

type errorResponse struct {
	Detail string `json:"detail"`
}

var simulations = map[string]SimulationResult{}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /api/presets", presetsHandler)
	mux.HandleFunc("GET /api/schema", schemaHandler)
	mux.HandleFunc("POST /api/simulations/generate-only", generateOnlyHandler)
	mux.HandleFunc("POST /api/simulations/run", runSimulationHandler)
	mux.HandleFunc("POST /api/simulations/schedule", scheduleHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	log.Printf("Scheduler Simulator API listening on :%s", port)
	if err := http.ListenAndServe(":"+port, withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "http://localhost:5173" || origin == "http://127.0.0.1:5173" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func presetsHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"presets": presets})
}

func schemaHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"openapi": "3.1.0",
		"info":    map[string]string{"title": "Scheduler Simulator API", "version": "0.1.0"},
		"paths": map[string]any{
			"/health":                        map[string]any{"get": map[string]string{"summary": "Health"}},
			"/api/presets":                   map[string]any{"get": map[string]string{"summary": "Presets"}},
			"/api/schema":                    map[string]any{"get": map[string]string{"summary": "Schema"}},
			"/api/simulations/generate-only": map[string]any{"post": map[string]string{"summary": "Generate simulation"}},
			"/api/simulations/run":           map[string]any{"post": map[string]string{"summary": "Run simulation"}},
			"/api/simulations/schedule":      map[string]any{"post": map[string]string{"summary": "Schedule generated simulation"}},
		},
	})
}

func generateOnlyHandler(w http.ResponseWriter, r *http.Request) {
	req, ok := readSimulationRequest(w, r)
	if !ok {
		return
	}
	generated, err := generateSimulation(req)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusOK, generated)
}

func runSimulationHandler(w http.ResponseWriter, r *http.Request) {
	req, ok := readSimulationRequest(w, r)
	if !ok {
		return
	}
	generated, err := generateSimulation(req)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	result, err := scheduleWorkflow(generated)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	simulations[result.ID] = result
	writeJSON(w, http.StatusOK, result)
}

func scheduleHandler(w http.ResponseWriter, r *http.Request) {
	var generated GeneratedSimulation
	if err := json.NewDecoder(r.Body).Decode(&generated); err != nil {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("invalid generated simulation: %w", err))
		return
	}
	if generated.SLA.OptionCount < 1 {
		generated.SLA.OptionCount = 1
	}
	response, err := optimizeSchedule(generated)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	for _, option := range response.Options {
		simulations[option.Result.ID] = option.Result
	}
	writeJSON(w, http.StatusOK, response)
}

func readSimulationRequest(w http.ResponseWriter, r *http.Request) (SimulationRequest, bool) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return SimulationRequest{}, false
	}
	req, err := decodeRequest(body)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return SimulationRequest{}, false
	}
	return req, true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, errorResponse{Detail: err.Error()})
}
