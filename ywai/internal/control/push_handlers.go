package control

import (
	"encoding/json"
	"log"
	"net/http"
)

// PushAPI holds the push sender and store for HTTP handlers.
type PushAPI struct {
	sender *PushSender
	store  *PushStore
}

// NewPushAPI creates a PushAPI with the given store.
func NewPushAPI(store *PushStore) *PushAPI {
	sender, err := NewPushSender(store)
	if err != nil {
		log.Printf("push: sender init error (push will be unavailable): %v", err)
		return &PushAPI{store: store}
	}
	return &PushAPI{sender: sender, store: store}
}

// registerPushRoutes mounts push endpoints on the server mux.
func (s *Server) registerPushRoutes() {
	api := s.push
	if api == nil {
		return
	}
	s.mux.HandleFunc("POST /api/push/subscribe", api.handleSubscribe)
	s.mux.HandleFunc("DELETE /api/push/subscribe", api.handleUnsubscribe)
	s.mux.HandleFunc("GET /api/push/vapid-key", api.handleVapidKey)
	s.mux.HandleFunc("POST /api/push/test", api.handleTestNotification)
}

// handleSubscribe stores a push subscription.
func (api *PushAPI) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.store == nil {
		http.Error(w, "push not available", http.StatusServiceUnavailable)
		return
	}
	var sub PushSubscription
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "invalid subscription", http.StatusBadRequest)
		return
	}
	if sub.Endpoint == "" || sub.Keys.P256DH == "" || sub.Keys.Auth == "" {
		http.Error(w, "missing endpoint or keys", http.StatusBadRequest)
		return
	}
	if err := api.store.Subscribe(sub); err != nil {
		log.Printf("push: subscribe error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "subscribed"})
}

// handleUnsubscribe removes a push subscription.
func (api *PushAPI) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.store == nil {
		http.Error(w, "push not available", http.StatusServiceUnavailable)
		return
	}
	endpoint := r.URL.Query().Get("endpoint")
	if endpoint == "" {
		// Support JSON body too
		var body struct {
			Endpoint string `json:"endpoint"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			endpoint = body.Endpoint
		}
	}
	if endpoint == "" {
		http.Error(w, "missing endpoint", http.StatusBadRequest)
		return
	}
	if err := api.store.Unsubscribe(endpoint); err != nil {
		log.Printf("push: unsubscribe error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unsubscribed"})
}

// handleVapidKey returns the VAPID public key for subscription.
func (api *PushAPI) handleVapidKey(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.sender == nil {
		http.Error(w, "push not available", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"publicKey": api.sender.PublicKey()})
}

// handleTestNotification sends a test push to all subscriptions.
func (api *PushAPI) handleTestNotification(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.sender == nil {
		http.Error(w, "push not available", http.StatusServiceUnavailable)
		return
	}
	count := len(api.store.List())
	if count == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"status": "no-subscribers", "sent": 0})
		return
	}
	if err := api.sender.Send("ywai Test", "Push notifications are working!"); err != nil {
		log.Printf("push: test notification error: %v", err)
		http.Error(w, "send failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "sent", "sent": count})
}
