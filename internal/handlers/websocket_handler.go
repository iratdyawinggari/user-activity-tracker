package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WebSocketHandler struct {
	upgrader   websocket.Upgrader
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
}

func NewWebSocketHandler() *WebSocketHandler {
	return &WebSocketHandler{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for testing
			},
		},
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

func (h *WebSocketHandler) HandleConnections(c *gin.Context) {
	log.Println("WebSocket connection attempt from:", c.Request.RemoteAddr)

	// Upgrade HTTP connection to WebSocket
	ws, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer func() {
		h.unregister <- ws
		ws.Close()
		log.Println("WebSocket connection closed")
	}()

	// Register client
	h.register <- ws
	log.Println("New WebSocket client registered")

	// Start goroutine to handle messages from this client
	go h.handleClientMessages(ws)

	// Keep connection alive
	for {
		time.Sleep(30 * time.Second)
		if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
			log.Println("Ping failed:", err)
			return
		}
	}
}

func (h *WebSocketHandler) handleClientMessages(ws *websocket.Conn) {
	for {
		var msg map[string]interface{}

		// Read message from client
		err := ws.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		log.Printf("Received message from client: %v", msg)

		// Handle different message types
		switch msg["type"] {
		case "subscribe":
			log.Println("Client subscribed to updates")
			response := map[string]interface{}{
				"type":      "subscribed",
				"message":   "Successfully subscribed to updates",
				"timestamp": time.Now().Unix(),
			}
			ws.WriteJSON(response)

		case "ping":
			log.Println("Received ping from client")
			response := map[string]interface{}{
				"type": "pong",
				"time": time.Now().Unix(),
			}
			ws.WriteJSON(response)

		default:
			log.Printf("Unknown message type: %s", msg["type"])
			response := map[string]interface{}{
				"type":      "error",
				"message":   "Unknown message type",
				"timestamp": time.Now().Unix(),
			}
			ws.WriteJSON(response)
		}
	}
}

func (h *WebSocketHandler) RunHub() {
	log.Println("Starting WebSocket hub")

	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			log.Println("Client registered. Total clients:", len(h.clients))

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
				log.Println("Client unregistered. Total clients:", len(h.clients))
			}

		case message := <-h.broadcast:
			// Send message to all connected clients
			for client := range h.clients {
				err := client.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					log.Printf("Error broadcasting to client: %v", err)
					client.Close()
					delete(h.clients, client)
				}
			}
		}
	}
}

func (h *WebSocketHandler) BroadcastUpdate(clientID string, data interface{}) {
	message := map[string]interface{}{
		"type":      "usage_update",
		"client_id": clientID,
		"data":      data,
		"timestamp": time.Now().Unix(),
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling broadcast message: %v", err)
		return
	}

	log.Printf("Broadcasting update for client %s: %s", clientID, string(jsonData))
	h.broadcast <- jsonData
}
