package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type createSessionResponse struct {
	ID string `json:"id"`
}

type joinSessionRequest struct {
	Name string `json:"name"`
}

type joinSessionResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type client struct {
	id   string
	name string
}

type session struct {
	id      string
	clients map[string]client
}

type hub struct {
	sessions map[string]session
}

func (h *hub) createSession(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request: %+v", r)

	u := uuid.New().String()
	h.sessions[u] = session{
		id:      u,
		clients: make(map[string]client),
	}
	b, err := json.Marshal(createSessionResponse{ID: u})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte{})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

func (h *hub) joinSession(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request: %+v", r)

	vars := mux.Vars(r)
	sessionID := vars["id"]

	log.Printf("SessionId: %+v", sessionID)

	session, ok := h.sessions[sessionID]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte{})
		return
	}

	log.Printf("Session: %+v", session)

	b, err := ioutil.ReadAll(r.Body)
	r.Body.Close()

	log.Printf("Request body: %s", b)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte{})
		return
	}

	var req joinSessionRequest
	err = json.Unmarshal(b, &req)

	log.Printf("Request name: %+v", req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte{})
		log.Printf("Error: %+v", err)
		return
	}

	u := uuid.New().String()
	session.clients[u] = client{
		id:   u,
		name: req.Name,
	}

	res, err := json.Marshal(joinSessionResponse{ID: u, Name: req.Name})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte{})
		return
	}
	w.Write(res)
}

func handler(w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}

		if err := conn.WriteMessage(messageType, p); err != nil {
			log.Println(err)
			return
		}
	}
}

func main() {
	r := mux.NewRouter()
	h := hub{
		sessions: make(map[string]session),
	}

	r.HandleFunc("/socket", handler)
	r.HandleFunc("/session/create", h.createSession)
	r.HandleFunc("/session/{id}/join", h.joinSession)

	srv := &http.Server{
		Addr:           ":8080",
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Fatal(srv.ListenAndServe())
}
