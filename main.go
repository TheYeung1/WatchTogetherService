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

type connectSessionRequest struct {
	ClientID string `json:"clientID"`
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
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

/*
* joinSession lets a new user join a room
 */
func (h *hub) joinSession(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request: %+v", r)

	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

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

/*
* connectSession connects to a socket for a given room
 */
func (h *hub) connectSession(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request: %+v", r)

	vars := mux.Vars(r)
	sessionID := vars["sessionId"]
	clientID := vars["clientId"]

	sess, ok := h.sessions[sessionID]
	if !ok {
		log.Printf("session not found: %+v", sessionID)
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte{})
		return
	}

	_, ok = sess.clients[clientID]
	if !ok {
		log.Printf("client %s not found for session %s", clientID, sessionID)
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte{})
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("could not upgrade to websocket: %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte{})
		return
	}

	// simple loop that echos back messages
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
	r.HandleFunc("/session/create", h.createSession).Methods("POST")
	r.HandleFunc("/session/{sessionId}/join", h.joinSession).Methods("POST")
	r.HandleFunc("/session/{sessionId}/connect/{clientId}", h.connectSession)

	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	srv := &http.Server{
		Addr:           ":8080",
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Fatal(srv.ListenAndServe())
}
