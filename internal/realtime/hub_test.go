package realtime

import (
	"testing"

	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
)

func makeTestClient(h *Hub, site, room string) *Client {
	return &Client{
		user:     &auth.User{Email: "u@co"},
		site:     site,
		room:     room,
		conn:     nil, // no real WebSocket needed
		send:     make(chan []byte, 1),
		hub:      h,
		presence: make(map[string]*auth.User),
	}
}

func addClientToRoom(h *Hub, c *Client) {
	r := h.getRoom(c.site + ":" + c.room)
	r.mu.Lock()
	r.clients[c] = struct{}{}
	r.mu.Unlock()
}

func TestRoomRemovedWhenLastClientLeaves(t *testing.T) {
	h := NewHub(config.DefaultDev())
	c := makeTestClient(h, "acme", "default")
	addClientToRoom(h, c)

	if got := h.RoomCount(); got != 1 {
		t.Fatalf("expected 1 room after adding client, got %d", got)
	}

	c.close()

	if got := h.RoomCount(); got != 0 {
		t.Fatalf("expected 0 rooms after last client leaves, got %d", got)
	}
}

func TestRoomStaysWhenOtherClientsRemain(t *testing.T) {
	h := NewHub(config.DefaultDev())
	c1 := makeTestClient(h, "acme", "default")
	c2 := makeTestClient(h, "acme", "default")
	addClientToRoom(h, c1)
	addClientToRoom(h, c2)

	if got := h.RoomCount(); got != 1 {
		t.Fatalf("expected 1 room with two clients, got %d", got)
	}

	c1.close()

	if got := h.RoomCount(); got != 1 {
		t.Fatalf("expected room to remain after one client leaves, got %d rooms", got)
	}

	room := h.getRoom("acme:default")
	room.mu.RLock()
	_, stillThere := room.clients[c2]
	count := len(room.clients)
	room.mu.RUnlock()
	if !stillThere {
		t.Fatalf("expected c2 to still be in the room")
	}
	if count != 1 {
		t.Fatalf("expected 1 client remaining in room, got %d", count)
	}
}

func TestRoomRecreatedOnNextGetRoom(t *testing.T) {
	h := NewHub(config.DefaultDev())
	c := makeTestClient(h, "acme", "default")
	addClientToRoom(h, c)
	c.close()

	if got := h.RoomCount(); got != 0 {
		t.Fatalf("expected 0 rooms after cleanup, got %d", got)
	}

	r := h.getRoom("acme:default")
	if r == nil {
		t.Fatalf("expected getRoom to return a fresh room")
	}
	r.mu.RLock()
	count := len(r.clients)
	r.mu.RUnlock()
	if count != 0 {
		t.Fatalf("expected fresh room to be empty, got %d clients", count)
	}
	if got := h.RoomCount(); got != 1 {
		t.Fatalf("expected 1 room after recreation, got %d", got)
	}
}
