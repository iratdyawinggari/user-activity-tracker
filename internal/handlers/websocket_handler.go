package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"user-activity-tracker/internal/cache"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WebSocketHandler struct {
	cache      *cache.CacheManager
	upgrader   websocket.Upgrader
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
}

func NewWebSocketHandler() *WebSocketHandler {
	return &WebSocketHandler{
		cache: cache.GetCacheManager(),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // In production, restrict origins
			},
		},
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

func (h *WebSocketHandler) HandleConnections(c *gin.Context) {
	// Upgrade HTTP connection to WebSocket
	ws, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer ws.Close()

	// Register client
	h.register <- ws

	// Listen for Redis Pub/Sub messages
	go h.listenForBroadcasts()

	// Handle incoming messages from client
	for {
		var msg map[string]interface{}
		err := ws.ReadJSON(&msg)
		if err != nil {
			h.unregister <- ws
			break
		}

		// Handle different message types
		switch msg["type"] {
		case "subscribe":
			// Client wants to subscribe to updates
			log.Printf("Client subscribed to updates")
		case "ping":
			// Respond to ping
			ws.WriteJSON(map[string]interface{}{
				"type": "pong",
				"time": time.Now().Unix(),
			})
		}
	}
}

func (h *WebSocketHandler) listenForBroadcasts() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			log.Println("New WebSocket client connected")

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
				log.Println("WebSocket client disconnected")
			}

		case message := <-h.broadcast:
			// Send message to all connected clients
			for client := range h.clients {
				err := client.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					log.Printf("WebSocket write error: %v", err)
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

	jsonData, _ := json.Marshal(message)
	h.broadcast <- jsonData
}
