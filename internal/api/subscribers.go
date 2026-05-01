package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// mountSubscribers registers all /subscribers routes on r.
func mountSubscribers(r chi.Router, s store.Store) {
	r.Get("/subscribers", listSubscribers(s))
	r.Post("/subscribers", createSubscriber(s))
	r.Get("/subscribers/{id}", getSubscriber(s))
	r.Put("/subscribers/{id}", updateSubscriber(s))
	r.Delete("/subscribers/{id}", deleteSubscriber(s))
}

// subscriberRequest is the JSON request body for Subscriber create and
// update operations.
type subscriberRequest struct {
	Name        string `json:"name"`
	Msisdn      string `json:"msisdn"`
	Iccid       string `json:"iccid"`
	Imei        string `json:"imei"`
	DeviceMake  string `json:"deviceMake"`
	DeviceModel string `json:"deviceModel"`
}

// subscriberResponse is the JSON response shape for a Subscriber.
type subscriberResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Msisdn      string `json:"msisdn"`
	Iccid       string `json:"iccid"`
	Imei        string `json:"imei,omitempty"`
	DeviceMake  string `json:"deviceMake,omitempty"`
	DeviceModel string `json:"deviceModel,omitempty"`
}

// toSubscriberResponse converts a store.Subscriber to a
// subscriberResponse.
func toSubscriberResponse(s store.Subscriber) subscriberResponse {
	return subscriberResponse{
		ID:          uuidToString(s.ID),
		Name:        s.Name,
		Msisdn:      s.Msisdn,
		Iccid:       s.Iccid,
		Imei:        s.Imei.String,
		DeviceMake:  s.DeviceMake.String,
		DeviceModel: s.DeviceModel.String,
	}
}

// optionalText converts a string to a pgtype.Text, treating empty
// string as null.
func optionalText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

// listSubscribers handles GET /subscribers.
func listSubscribers(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subs, err := s.ListSubscribers(r.Context())
		if err != nil {
			respondInternalError(w)
			return
		}
		out := make([]subscriberResponse, len(subs))
		for i, sub := range subs {
			out[i] = toSubscriberResponse(sub)
		}
		respondJSON(w, http.StatusOK, out)
	}
}

// createSubscriber handles POST /subscribers.
func createSubscriber(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req subscriberRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Msisdn == "" {
			respondInvalidRequest(w, "msisdn is required")
			return
		}
		if req.Iccid == "" {
			respondInvalidRequest(w, "iccid is required")
			return
		}
		sub, err := s.InsertSubscriber(r.Context(), store.InsertSubscriberParams{
			Name:        req.Name,
			Msisdn:      req.Msisdn,
			Iccid:       req.Iccid,
			Imei:        optionalText(req.Imei),
			DeviceMake:  optionalText(req.DeviceMake),
			DeviceModel: optionalText(req.DeviceModel),
		})
		if mapStoreError(w, err) != nil {
			return
		}
		respondJSON(w, http.StatusCreated, toSubscriberResponse(sub))
	}
}

// getSubscriber handles GET /subscribers/{id}.
func getSubscriber(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDParam(w, r, "id")
		if !ok {
			return
		}
		sub, err := s.GetSubscriber(r.Context(), id)
		if mapStoreError(w, err) != nil {
			return
		}
		respondJSON(w, http.StatusOK, toSubscriberResponse(sub))
	}
}

// updateSubscriber handles PUT /subscribers/{id}.
func updateSubscriber(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDParam(w, r, "id")
		if !ok {
			return
		}
		var req subscriberRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Msisdn == "" {
			respondInvalidRequest(w, "msisdn is required")
			return
		}
		if req.Iccid == "" {
			respondInvalidRequest(w, "iccid is required")
			return
		}
		sub, err := s.UpdateSubscriber(r.Context(), store.UpdateSubscriberParams{
			ID:          id,
			Name:        req.Name,
			Msisdn:      req.Msisdn,
			Iccid:       req.Iccid,
			Imei:        optionalText(req.Imei),
			DeviceMake:  optionalText(req.DeviceMake),
			DeviceModel: optionalText(req.DeviceModel),
		})
		if mapStoreError(w, err) != nil {
			return
		}
		respondJSON(w, http.StatusOK, toSubscriberResponse(sub))
	}
}

// deleteSubscriber handles DELETE /subscribers/{id}.
func deleteSubscriber(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDParam(w, r, "id")
		if !ok {
			return
		}
		err := s.DeleteSubscriber(r.Context(), id)
		if mapStoreError(w, err) != nil {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
