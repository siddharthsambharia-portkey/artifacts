package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/nats-io/nats.go"
	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
)

type Hub struct {
	cfg     *config.Config
	rooms   map[string]*Room
	mu      sync.RWMutex
	events  chan DBEvent
	nats    *nats.Conn
	subject string
}

type DBEvent struct {
	Site       string          `json:"site"`
	Collection string          `json:"collection"`
	Type       string          `json:"type"`
	Document   json.RawMessage `json:"document"`
}

type Room struct {
	name    string
	clients map[*Client]struct{}
	mu      sync.RWMutex
}

type Client struct {
	user     *auth.User
	site     string
	room     string
	conn     *websocket.Conn
	send     chan []byte
	hub      *Hub
	presence map[string]*auth.User
}

func NewHub(cfg *config.Config) *Hub {
	h := &Hub{
		cfg:     cfg,
		rooms:   make(map[string]*Room),
		events:  make(chan DBEvent, 256),
		subject: "artifact.db." + cfg.Domain,
	}
	go h.broadcastDBEvents()
	return h
}

func (h *Hub) SetNATS(nc *nats.Conn) {
	h.nats = nc
	if nc != nil {
		nc.Subscribe(h.subject, func(msg *nats.Msg) {
			var e DBEvent
			if json.Unmarshal(msg.Data, &e) == nil {
				h.deliverLocal(e, msg.Data)
			}
		})
	}
}

func (h *Hub) getRoom(name string) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.rooms[name]; ok {
		return r
	}
	r := &Room{name: name, clients: make(map[*Client]struct{})}
	h.rooms[name] = r
	return r
}

func (h *Hub) PublishDBEvent(e DBEvent) {
	select {
	case h.events <- e:
	default:
	}
}

func (h *Hub) broadcastDBEvents() {
	for e := range h.events {
		data, _ := json.Marshal(e)
		h.deliverLocal(e, data)
		if h.nats != nil {
			h.nats.Publish(h.subject, data)
		}
	}
}

func (h *Hub) deliverLocal(e DBEvent, data []byte) {
	roomKey := e.Site + ":" + e.Collection
	h.getRoom(roomKey).broadcast(data)
}

func (r *Room) broadcast(msg []byte) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for c := range r.clients {
		select {
		case c.send <- msg:
		default:
		}
	}
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	site := h.cfg.SiteFromHost(r.Host)
	roomName := r.URL.Query().Get("room")
	if roomName == "" {
		roomName = "default"
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	client := &Client{
		user: u, site: site, room: roomName, conn: conn,
		send: make(chan []byte, 64), hub: h,
		presence: make(map[string]*auth.User),
	}
	room := h.getRoom(site + ":" + roomName)
	room.mu.Lock()
	room.clients[client] = struct{}{}
	room.mu.Unlock()
	go client.writePump()
	go client.readPump()
	client.sendPresence()
}

func (c *Client) readPump() {
	defer c.close()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		_, data, err := c.conn.Read(ctx)
		cancel()
		if err != nil {
			return
		}
		var msg struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		if msg.Type == "message" {
			out, _ := json.Marshal(map[string]any{
				"type": "message", "from": c.user, "payload": msg.Payload,
			})
			room := c.hub.getRoom(c.site + ":" + c.room)
			room.broadcast(out)
			if c.hub.nats != nil {
				c.hub.nats.Publish("artifact.ws."+c.hub.cfg.Domain+"."+c.site+"."+c.room, out)
			}
		}
	}
}

func (c *Client) writePump() {
	for msg := range c.send {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		c.conn.Write(ctx, websocket.MessageText, msg)
		cancel()
	}
}

func (c *Client) sendPresence() {
	out, _ := json.Marshal(map[string]any{
		"type": "presence", "user": c.user,
	})
	c.hub.getRoom(c.site + ":" + c.room).broadcast(out)
}

func (c *Client) close() {
	room := c.hub.getRoom(c.site + ":" + c.room)
	room.mu.Lock()
	delete(room.clients, c)
	room.mu.Unlock()
	c.conn.Close(websocket.StatusNormalClosure, "")
	close(c.send)
}
