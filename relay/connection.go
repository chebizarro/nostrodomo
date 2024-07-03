package relay

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nbd-wtf/go-nostr"
	"github.com/puzpuzpuz/xsync/v3"
	"sharegap.net/nostrodomo/logger"
	"sharegap.net/nostrodomo/models"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Connection struct {
	Relay         *Relay
	Subscriptions *xsync.MapOf[string, nostr.ReqEnvelope]
	EventChannel  chan *nostr.Event
	Connection    *websocket.Conn
	id            int64
}

func (c *Connection) Listen() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Connection.Close()
	}()
	logger.Info("Client is listening for incomming events")
	for {
		select {
		case event, ok := <-c.EventChannel:
			c.Connection.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The relay closed the channel.
				c.Connection.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.Subscriptions.Range(
				func(key string, value nostr.ReqEnvelope) bool {
					if value.Filters.Match(event) {
						logger.Info("Message matches susbscription filters")
						c.Write(&nostr.EventEnvelope{
							SubscriptionID: &key,
							Event:          *event,
						})
					}
					return true
				},
			)
		case <-ticker.C:
			c.Connection.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Connection.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Connection) Read() {
	defer func() {
		c.Relay.DisconnectChannel <- c
		c.Connection.Close()
	}()
	c.Connection.SetReadLimit(maxMessageSize)
	c.Connection.SetReadDeadline(time.Now().Add(pongWait))
	c.Connection.SetPongHandler(func(string) error { c.Connection.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.Connection.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		req := nostr.ParseMessage(message)
		switch env := req.(type) {
		case *nostr.EventEnvelope:
			// Send an event
			c.Publish(env)
		case *nostr.ReqEnvelope:
			// Request a subscription
			c.Subscribe(env)
		case *nostr.CloseEnvelope:
			// Close a subscription
			c.UnSubscribe(env.String())
		default:
			n := nostr.NoticeEnvelope("Unrecognized Event")
			c.Write(&n)
		}
	}
}

func (c *Connection) Publish(event *nostr.EventEnvelope) {
	logger.Info("Publishing event from client")
	result := make(chan nostr.Envelope)
	req := &models.PubSubEnvelope{
		ClientID: c.id,
		Event:    event,
		Result:   result,
	}
	c.Relay.PublishChannel <- req
	res := <-result
	c.Write(res)
}

func (c *Connection) Write(env nostr.Envelope) error {
	logger.Debug("Writing to websocket:", env.String())
	w, err := c.Connection.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(env); err != nil {
		return err
	}
	return w.Close()
}

func (c *Connection) writeRaw(p []byte) error {
	logger.Info("Writing raw bytes to the websocket")
	w, err := c.Connection.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	if _, err := w.Write(p); err != nil {
		return err
	}
	return w.Close()
}

func (c *Connection) Subscribe(env *nostr.ReqEnvelope) {
	result := make(chan nostr.Envelope)
	req := &models.PubSubEnvelope{
		ClientID: c.id,
		Event:    env,
		Result:   result,
	}

	c.Subscriptions.Store(env.SubscriptionID, *env)

	c.Relay.SubscriptionChannel <- req
	res := <-result

	switch env := res.(type) {
	case *nostr.ClosedEnvelope:
		c.UnSubscribe(env.SubscriptionID)
		c.Write(env)
	case *models.RawEventEnvelope:
		raw, _ := env.MarshalJSON()
		c.writeRaw(raw)
	default:
		c.Write(env)
	}
}

func (c *Connection) UnSubscribe(subID string) {
	logger.Info("Client unsubscribed")
	c.Subscriptions.Delete(subID)
}

func (c *Connection) Disconnect() {
	logger.Info("Client disconnected")
	c.Subscriptions.Range(
		func(key string, value nostr.ReqEnvelope) bool {
			c.Write(
				&nostr.ClosedEnvelope{
					SubscriptionID: key,
					Reason:         "",
				},
			)
			c.UnSubscribe(key)
			return true
		},
	)
	c.Connection.Close()
}
