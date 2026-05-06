package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/eddiecarpenter/ocs-testbench/internal/engine"
)

func mountExpressions(r chi.Router) {
	r.Post("/expressions/evaluate", evaluateExpression())
}

type evaluateRequest struct {
	Expression string         `json:"expression"`
	Vars       map[string]any `json:"vars"`
}

type evaluateResponse struct {
	Result any    `json:"result"`
	Error  string `json:"error,omitempty"`
}

// evaluateExpression handles POST /expressions/evaluate.
// Evaluates a ruleevaluator expression against the supplied variable map and
// returns the result. {{VAR}} notation is normalised to bare VAR before
// evaluation (same preprocessing as the execution engine).
func evaluateExpression() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req evaluateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondInvalidRequest(w, "invalid request body")
			return
		}
		if req.Expression == "" {
			respondInvalidRequest(w, "expression is required")
			return
		}
		if req.Vars == nil {
			req.Vars = map[string]any{}
		}
		// JSON decodes numbers as float64; the engine stores them as int64.
		// Normalise so expressions like RESULT_CODE != 2001 don't type-mismatch.
		for k, v := range req.Vars {
			if f, ok := v.(float64); ok {
				req.Vars[k] = int64(f)
			}
		}

		result, err := engine.EvalExprPublic(req.Vars, req.Expression)
		if err != nil {
			respondJSON(w, http.StatusOK, evaluateResponse{Error: err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, evaluateResponse{Result: result})
	}
}
