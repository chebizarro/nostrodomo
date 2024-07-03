package relay

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nbd-wtf/go-nostr"
	"github.com/puzpuzpuz/xsync/v3"
	"sharegap.net/nostrodomo/config"
	"sharegap.net/nostrodomo/logger"
	"sharegap.net/nostrodomo/models"
)

type Connection struct {
	Relay         *Relay
	Subscriptions *xsync.MapOf[string, nostr.ReqEnvelope]
	Connection    *websocket.Conn
	id            int64
	sync.RWMutex
	config *config.WebSocketConfig
}

func NewConnection(cfg *config.WebSocketConfig, relay *Relay, conn *websocket.Conn) *Connection {
	return &Connection{
		Relay:         relay,
		Connection:    conn,
		Subscriptions: xsync.NewMapOf[string, nostr.ReqEnvelope](),
		config:        cfg,
	}
}

func (c *Connection) Listen() {
	ticker := time.NewTicker(c.config.PingPeriod)
	defer func() {
		ticker.Stop()
		if c.Connection != nil {
			c.Connection.Close()
		}
	}()
	logger.Info("Client is listening for incoming events")
	for {
		select {
		case event, ok := <-c.Relay.EventChannel:
			//c.Connection.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
			if !ok {
				// The relay closed the channel.
				err := c.Connection.WriteMessage(websocket.CloseMessage, []byte{})
				if err != nil {
					logger.Error("Failed to write close message:", err)
				}
				return
			}

			if c.Connection != nil {
				c.Lock()
				err := c.Connection.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
				if err != nil {
					logger.Error("Failed to set write deadline:", err)
					c.Unlock()
					return
				}
				c.Unlock()
			}

			c.Subscriptions.Range(func(key string, value nostr.ReqEnvelope) bool {
				if value.Filters.Match(event) {
					logger.Info("Message matches subscription filters")
					c.Write(&nostr.EventEnvelope{
						SubscriptionID: &key,
						Event:          *event,
					})
				}
				return true
			})
		case <-ticker.C:
			c.Lock()
			if c.Connection != nil {
				err := c.Connection.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
				if err != nil {
					logger.Error("Failed to set write deadline:", err)
					c.Unlock()
					return
				}
				err = c.Connection.WriteMessage(websocket.PingMessage, nil)
				if err != nil {
					logger.Error("Failed to send ping message:", err)
					c.Unlock()
					return
				}
			}
			c.Unlock()
		}
	}
}

func (c *Connection) closeConnection() {
	c.Lock()
	defer c.Unlock()
	if c.Connection != nil {
		c.Connection.Close()
		c.Connection = nil
	}
}

func (c *Connection) Read() {
	defer func() {
		c.Relay.DisconnectChannel <- c
		c.closeConnection()
	}()
	//c.Connection.SetReadLimit(c.config.MaxMessageSize)
	c.Connection.SetReadDeadline(time.Now().Add(c.config.PongWait))
	c.Connection.SetPongHandler(func(string) error { c.Connection.SetReadDeadline(time.Now().Add(c.config.PongWait)); return nil })
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
	responseChan := make(chan nostr.Envelope, 1)

	req := &models.PubSubEnvelope{
		ClientID: c.id,
		Event:    event,
		Result:   responseChan,
	}

	c.Relay.PublishChannel <- req
	res := <-responseChan
	c.Write(res)
}

func (c *Connection) Write(env nostr.Envelope) error {
	c.Lock()
	defer c.Unlock()
	logger.Debug("Writing to websocket:", env.Label())
	w, err := c.Connection.NextWriter(websocket.TextMessage)
	if err != nil {
		return fmt.Errorf("failed to get next writer: %w", err)
	}
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(env); err != nil {
		w.Close()
		return fmt.Errorf("failed to encode envelope: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}
	return nil
}

func (c *Connection) writeRaw(p []byte) error {
	c.Lock()
	defer c.Unlock()
	logger.Debug("Writing raw bytes to the websocket:", string(p))
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

	go func() {
		for res := range result {
			switch env := res.(type) {
			case *nostr.ClosedEnvelope:
				c.UnSubscribe(env.SubscriptionID)
				c.Write(env)
				close(result)
			case *models.RawEventEnvelope:
				raw, _ := env.MarshalJSON()
				c.writeRaw(raw)
			default:
				c.Write(env)
			}
		}
	}()
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
	c.closeConnection()
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
