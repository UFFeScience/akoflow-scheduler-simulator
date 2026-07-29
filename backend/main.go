package main

import (
	"encoding/json"
	"flag"
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
	experimentOutput := flag.String("experiment-output", "", "run the complete experimental protocol and write CSV results to this directory")
	experimentRepetitions := flag.Int("experiment-repetitions", 30, "number of paired interference seeds")
	experimentBeamWidth := flag.Int("experiment-beam-width", minBeamWidth, "beam width used by the experimental protocol")
	experimentRecommendations := flag.Int("experiment-recommendations", 100, "maximum PRISM-CC recommendations exported per algorithm, environment, and seed")
	experimentWorkers := flag.Int("experiment-workers", 0, "parallel environment/seed jobs (default: min(4, GOMAXPROCS))")
	experimentPRISMCCPriority := flag.String("experiment-prism-priority", "topological_order", "PRISM-CC task priority: topological_order, upward_rank, ready_lookahead, or adaptive_ready")
	experimentWorkflow := flag.String("experiment-workflow", "montage_050d", "experiment workflow: montage_050d or montage_dss_20d")
	experimentHEFTMode := flag.String("experiment-heft-mode", "classic_no_colocation", "HEFT baseline: classic_no_colocation or colocation")
	experimentScenarios := flag.String("experiment-scenarios", "", "comma-separated experiment scenarios (default: all)")
	experimentInterferenceRate := flag.Float64("experiment-interference-rate", 0.20, "controlled software interference penalty per overlapping task")
	experimentBudgetLimit := flag.Float64("experiment-budget-limit", 0, "fixed budget limit; 0 calibrates from the selected HEFT baseline")
	experimentDeadlineLimit := flag.Float64("experiment-deadline-limit", 0, "fixed deadline limit; 0 calibrates from the selected HEFT baseline")
	experimentBudgetMargin := flag.Float64("experiment-budget-margin", experimentSLAMargin, "budget multiplier over the per-scenario mean HEFT cost")
	experimentDeadlineMargin := flag.Float64("experiment-deadline-margin", experimentSLAMargin, "deadline multiplier over the per-scenario mean HEFT makespan")
	flag.Parse()
	if *experimentOutput != "" {
		if err := runExperimentalProtocol(ExperimentRunOptions{
			OutputDirectory: *experimentOutput, Repetitions: *experimentRepetitions,
			StructuralSeed: 42, BeamWidth: *experimentBeamWidth,
			RecommendationCount: *experimentRecommendations,
			Workers:             *experimentWorkers,
			PRISMCCPriority:     *experimentPRISMCCPriority,
			WorkflowID:          *experimentWorkflow,
			HEFTMode:            *experimentHEFTMode,
			ScenarioIDs:         splitNonEmptyCSV(*experimentScenarios),
			InterferenceRate:    *experimentInterferenceRate,
			FixedBudgetLimit:    *experimentBudgetLimit,
			FixedDeadlineLimit:  *experimentDeadlineLimit,
			BudgetMargin:        *experimentBudgetMargin,
			DeadlineMargin:      *experimentDeadlineMargin,
		}); err != nil {
			log.Fatal(err)
		}
		log.Printf("experimental results written to %s", *experimentOutput)
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /api/presets", presetsHandler)
	mux.HandleFunc("GET /api/experiment-scenarios", experimentScenariosHandler)
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

func experimentScenariosHandler(w http.ResponseWriter, _ *http.Request) {
	scenarios, err := experimentScenarios()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"scenarios": scenarios})
}

func schemaHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"openapi": "3.1.0",
		"info":    map[string]string{"title": "Scheduler Simulator API", "version": "0.1.0"},
		"paths": map[string]any{
			"/health":                        map[string]any{"get": map[string]string{"summary": "Health"}},
			"/api/presets":                   map[string]any{"get": map[string]string{"summary": "Presets"}},
			"/api/experiment-scenarios":      map[string]any{"get": map[string]string{"summary": "Experiment scenarios"}},
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
