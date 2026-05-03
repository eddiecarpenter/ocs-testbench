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
	r.Get("/tac-catalog", listTacCatalog())
}

// tacEntry is one row in the TAC catalogue returned by GET /tac-catalog.
type tacEntry struct {
	TAC          string `json:"tac"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	Year         int    `json:"year,omitempty"`
}

// tacCatalog is a curated built-in list of common devices used to
// populate the Manufacturer → Model cascading selects in the UI.
var tacCatalog = []tacEntry{
	{TAC: "35617510", Manufacturer: "Apple", Model: "iPhone 15 Pro Max", Year: 2023},
	{TAC: "35617309", Manufacturer: "Apple", Model: "iPhone 15 Pro", Year: 2023},
	{TAC: "35617209", Manufacturer: "Apple", Model: "iPhone 15 Plus", Year: 2023},
	{TAC: "35617109", Manufacturer: "Apple", Model: "iPhone 15", Year: 2023},
	{TAC: "35390911", Manufacturer: "Apple", Model: "iPhone 14 Pro Max", Year: 2022},
	{TAC: "35390811", Manufacturer: "Apple", Model: "iPhone 14 Pro", Year: 2022},
	{TAC: "35390711", Manufacturer: "Apple", Model: "iPhone 14 Plus", Year: 2022},
	{TAC: "35390611", Manufacturer: "Apple", Model: "iPhone 14", Year: 2022},
	{TAC: "35254610", Manufacturer: "Apple", Model: "iPhone 13 Pro Max", Year: 2021},
	{TAC: "35254510", Manufacturer: "Apple", Model: "iPhone 13 Pro", Year: 2021},
	{TAC: "35254410", Manufacturer: "Apple", Model: "iPhone 13 Mini", Year: 2021},
	{TAC: "35254310", Manufacturer: "Apple", Model: "iPhone 13", Year: 2021},
	{TAC: "86751507", Manufacturer: "Samsung", Model: "Galaxy S24 Ultra", Year: 2024},
	{TAC: "86751407", Manufacturer: "Samsung", Model: "Galaxy S24+", Year: 2024},
	{TAC: "86751307", Manufacturer: "Samsung", Model: "Galaxy S24", Year: 2024},
	{TAC: "35694311", Manufacturer: "Samsung", Model: "Galaxy S23 Ultra", Year: 2023},
	{TAC: "35694211", Manufacturer: "Samsung", Model: "Galaxy S23+", Year: 2023},
	{TAC: "35694111", Manufacturer: "Samsung", Model: "Galaxy S23", Year: 2023},
	{TAC: "86348202", Manufacturer: "Samsung", Model: "Galaxy A54", Year: 2023},
	{TAC: "86348102", Manufacturer: "Samsung", Model: "Galaxy A34", Year: 2023},
	{TAC: "35978711", Manufacturer: "Google", Model: "Pixel 8 Pro", Year: 2023},
	{TAC: "35978611", Manufacturer: "Google", Model: "Pixel 8", Year: 2023},
	{TAC: "35290011", Manufacturer: "Google", Model: "Pixel 7 Pro", Year: 2022},
	{TAC: "35289911", Manufacturer: "Google", Model: "Pixel 7", Year: 2022},
	{TAC: "86853601", Manufacturer: "Huawei", Model: "P60 Pro", Year: 2023},
	{TAC: "86726601", Manufacturer: "Huawei", Model: "Mate 60 Pro", Year: 2023},
	{TAC: "35805711", Manufacturer: "OnePlus", Model: "12", Year: 2024},
	{TAC: "35805611", Manufacturer: "OnePlus", Model: "11", Year: 2023},
	{TAC: "86669904", Manufacturer: "Xiaomi", Model: "14 Pro", Year: 2024},
	{TAC: "86669804", Manufacturer: "Xiaomi", Model: "14", Year: 2024},
}

// listTacCatalog handles GET /tac-catalog.
func listTacCatalog() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, tacCatalog)
	}
}

// subscriberRequest is the JSON request body for Subscriber create and
// update operations.
type subscriberRequest struct {
	Name   string `json:"name"`
	Msisdn string `json:"msisdn"`
	Iccid  string `json:"iccid"`
	Imei   string `json:"imei"`
	Tac    string `json:"tac"`
}

// subscriberResponse is the JSON response shape for a Subscriber.
type subscriberResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Msisdn string `json:"msisdn"`
	Iccid  string `json:"iccid"`
	Imei   string `json:"imei,omitempty"`
	Tac    string `json:"tac,omitempty"`
}

// toSubscriberResponse converts a store.Subscriber to a
// subscriberResponse.
func toSubscriberResponse(s store.Subscriber) subscriberResponse {
	return subscriberResponse{
		ID:     uuidToString(s.ID),
		Name:   s.Name,
		Msisdn: s.Msisdn,
		Iccid:  s.Iccid,
		Imei:   s.Imei.String,
		Tac:    s.Tac.String,
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
			Name:   req.Name,
			Msisdn: req.Msisdn,
			Iccid:  req.Iccid,
			Imei:   optionalText(req.Imei),
			Tac:    optionalText(req.Tac),
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
			ID:     id,
			Name:   req.Name,
			Msisdn: req.Msisdn,
			Iccid:  req.Iccid,
			Imei:   optionalText(req.Imei),
			Tac:    optionalText(req.Tac),
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
