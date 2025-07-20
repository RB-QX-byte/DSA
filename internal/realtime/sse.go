package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Event represents a real-time event
type Event struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// Client represents a connected SSE client
type Client struct {
	ID       string
	UserID   string
	Channel  chan Event
	Request  *http.Request
	Writer   http.ResponseWriter
	Context  context.Context
	Cancel   context.CancelFunc
}

// Hub manages all SSE connections
type Hub struct {
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan Event
	mutex      sync.RWMutex
}

// NewHub creates a new SSE hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan Event),
	}
}

// Run starts the hub
func (h *Hub) Run(ctx context.Context) {
	log.Println("Starting SSE Hub")
	
	// Cleanup timer
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client.ID] = client
			h.mutex.Unlock()
			log.Printf("Client %s connected (user: %s). Total clients: %d", client.ID, client.UserID, len(h.clients))

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, exists := h.clients[client.ID]; exists {
				close(client.Channel)
				delete(h.clients, client.ID)
				client.Cancel()
			}
			h.mutex.Unlock()
			log.Printf("Client %s disconnected. Total clients: %d", client.ID, len(h.clients))

		case event := <-h.broadcast:
			h.mutex.RLock()
			for _, client := range h.clients {
				select {
				case client.Channel <- event:
				default:
					// Client channel is full, disconnect
					close(client.Channel)
					delete(h.clients, client.ID)
					client.Cancel()
				}
			}
			h.mutex.RUnlock()

		case <-ticker.C:
			// Cleanup disconnected clients
			h.cleanupClients()

		case <-ctx.Done():
			log.Println("SSE Hub shutting down")
			h.mutex.Lock()
			for _, client := range h.clients {
				close(client.Channel)
				client.Cancel()
			}
			h.clients = make(map[string]*Client)
			h.mutex.Unlock()
			return
		}
	}
}

// cleanupClients removes disconnected clients
func (h *Hub) cleanupClients() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	for id, client := range h.clients {
		select {
		case <-client.Context.Done():
			close(client.Channel)
			delete(h.clients, id)
			log.Printf("Cleaned up disconnected client %s", id)
		default:
			// Client is still connected
		}
	}
}

// RegisterClient registers a new SSE client
func (h *Hub) RegisterClient(client *Client) {
	h.register <- client
}

// UnregisterClient unregisters an SSE client
func (h *Hub) UnregisterClient(client *Client) {
	h.unregister <- client
}

// BroadcastEvent broadcasts an event to all connected clients
func (h *Hub) BroadcastEvent(eventType string, data interface{}) {
	event := Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now(),
	}
	
	select {
	case h.broadcast <- event:
	default:
		log.Printf("Failed to broadcast event %s - channel full", eventType)
	}
}

// BroadcastToContest broadcasts an event to clients watching a specific contest
func (h *Hub) BroadcastToContest(contestID string, eventType string, data interface{}) {
	event := Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now(),
	}

	h.mutex.RLock()
	defer h.mutex.RUnlock()

	for _, client := range h.clients {
		// Check if client is subscribed to this contest
		if contestIDFromContext, ok := client.Request.Context().Value("contestID").(string); ok && contestIDFromContext == contestID {
			select {
			case client.Channel <- event:
			default:
				// Client channel is full, skip
			}
		}
	}
}

// BroadcastToUser broadcasts an event to a specific user
func (h *Hub) BroadcastToUser(userID string, eventType string, data interface{}) {
	event := Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now(),
	}

	h.mutex.RLock()
	defer h.mutex.RUnlock()

	for _, client := range h.clients {
		if client.UserID == userID {
			select {
			case client.Channel <- event:
			default:
				// Client channel is full, skip
			}
		}
	}
}

// GetClientCount returns the number of connected clients
func (h *Hub) GetClientCount() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return len(h.clients)
}

// GetContestClientCount returns the number of clients connected to a specific contest
func (h *Hub) GetContestClientCount(contestID string) int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	count := 0
	for _, client := range h.clients {
		if contestIDFromContext, ok := client.Request.Context().Value("contestID").(string); ok && contestIDFromContext == contestID {
			count++
		}
	}
	return count
}

// SendEvent sends an event to a specific client
func (c *Client) SendEvent(event Event) error {
	select {
	case c.Channel <- event:
		return nil
	default:
		return fmt.Errorf("client channel is full")
	}
}

// WriteEvent writes an event to the SSE connection
func (c *Client) WriteEvent(event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// SSE format
	fmt.Fprintf(c.Writer, "id: %s\n", event.ID)
	fmt.Fprintf(c.Writer, "event: %s\n", event.Type)
	fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))

	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}

	return nil
}

// Listen starts listening for events and writes them to the SSE connection
func (c *Client) Listen() {
	defer func() {
		log.Printf("Client %s listener stopped", c.ID)
	}()

	// Send initial connection event
	c.WriteEvent(Event{
		ID:        uuid.New().String(),
		Type:      "connected",
		Data:      map[string]string{"status": "connected", "clientId": c.ID},
		Timestamp: time.Now(),
	})

	// Listen for events
	for {
		select {
		case event := <-c.Channel:
			if err := c.WriteEvent(event); err != nil {
				log.Printf("Error writing event to client %s: %v", c.ID, err)
				return
			}
		case <-c.Context.Done():
			log.Printf("Client %s context cancelled", c.ID)
			return
		}
	}
}