package websocket

import (
	"CheckHealthDO/internal/pkg/logger"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var (
	// Registry singleton
	registry *Registry
	once     sync.Once
)

// Registry manages WebSocket handlers for different services
type Registry struct {
	mu              sync.RWMutex
	cpuHandler      *Handler
	memoryHandler   *Handler
	mariaDBHandler  *Handler
	sysInfoHandlers *Handler
	diskHandlers    *Handler
}

// GetRegistry returns the WebSocket registry singleton
func GetRegistry() *Registry {
	once.Do(func() {
		registry = &Registry{}
	})
	return registry
}

// Handler manages WebSocket connections
type Handler struct {
	clients  map[*Client]bool
	mu       sync.RWMutex
	upgrader websocket.Upgrader
}

// Client represents a WebSocket client connection
type Client struct {
	conn *websocket.Conn
}

// NewHandler creates a new WebSocket handler
func NewHandler() *Handler {
	return &Handler{
		clients: make(map[*Client]bool),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for development
			},
		},
	}
}

// ServeHTTP handles WebSocket connections
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("Failed to upgrade to WebSocket connection",
			logger.String("error", err.Error()))
		return
	}

	client := &Client{conn: conn}

	// Register client
	h.mu.Lock()
	h.clients[client] = true
	h.mu.Unlock()

	// Handle disconnect when connection closes
	defer func() {
		conn.Close()
		h.mu.Lock()
		delete(h.clients, client)
		h.mu.Unlock()
	}()

	// Keep connection open
	for {
		// Read messages but discard them - we're only interested in broadcasting
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// Broadcast sends a message to all clients of this handler
func (h *Handler) Broadcast(message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		err := client.conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			logger.Error("Error broadcasting to WebSocket client",
				logger.String("error", err.Error()))
			client.conn.Close()
			delete(h.clients, client)
		}
	}
}

// Handler getters and setters for the registry

// GetCPUHandler returns the CPU-specific WebSocket handler
func (r *Registry) GetCPUHandler() *Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cpuHandler
}

func (r *Registry) GetSysHandler() *Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sysInfoHandlers
}

func (r *Registry) GetDiskHandler() *Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.diskHandlers
}

// RegisterCPUHandler sets the CPU-specific WebSocket handler
func (r *Registry) RegisterDiskHandler(handler *Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.diskHandlers = handler
}

// RegisterCPUHandler sets the CPU-specific WebSocket handler
func (r *Registry) RegisterCPUHandler(handler *Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cpuHandler = handler
}

func (r *Registry) RegisterSysHandler(handler *Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sysInfoHandlers = handler
}

// GetMemoryHandler returns the memory-specific WebSocket handler
func (r *Registry) GetMemoryHandler() *Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.memoryHandler
}

// RegisterMemoryHandler sets the memory-specific WebSocket handler
func (r *Registry) RegisterMemoryHandler(handler *Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.memoryHandler = handler
}

// GetMariaDBHandler returns the MariaDB-specific WebSocket handler
func (r *Registry) GetMariaDBHandler() *Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.mariaDBHandler
}

// RegisterMariaDBHandler sets the MariaDB-specific WebSocket handler
func (r *Registry) RegisterMariaDBHandler(handler *Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mariaDBHandler = handler
}
