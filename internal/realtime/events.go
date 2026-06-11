package realtime

import "encoding/json"

// EventPublisher publishes realtime events to connected clients.
type EventPublisher interface {
	PublishDBEvent(e DBEvent)
}

func (h *Hub) PublishDocumentEvent(site, collection, eventType string, doc any) {
	data, err := json.Marshal(doc)
	if err != nil {
		return
	}
	h.PublishDBEvent(DBEvent{
		Site: site, Collection: collection, Type: eventType, Document: data,
	})
}
