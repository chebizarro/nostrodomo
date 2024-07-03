package relay

import (
	"context"
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
	ctx    context.Context
	cancel context.CancelFunc
}

func NewConnection(cfg *config.WebSocketConfig, relay *Relay, conn *websocket.Conn) *Connection {
	ctx, cancel := context.WithCancel(context.Background())
	return &Connection{
		Relay:         relay,
		Connection:    conn,
		Subscriptions: xsync.NewMapOf[string, nostr.ReqEnvelope](),
		config:        cfg,
		ctx:           ctx,
		cancel:        cancel,
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
		case <-c.ctx.Done():
			return
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
			c.RLock()
			c.Connection.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
			c.RUnlock()

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
			c.RLock()
			err := c.Connection.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
			c.RUnlock()
			if err != nil {
				logger.Error("Failed to set write deadline:", err)
				return
			}
			if err := c.Connection.WriteMessage(websocket.PingMessage, nil); err != nil {
				logger.Error("Failed to send ping message:", err)
				return
			}
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
	c.Connection.SetReadLimit(c.config.MaxMessageSize)
	c.Connection.SetReadDeadline(time.Now().Add(c.config.PongWait))
	c.Connection.SetPongHandler(func(string) error { c.Connection.SetReadDeadline(time.Now().Add(c.config.PongWait)); return nil })
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			_, message, err := c.Connection.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("error: %v", err)
				}
				return
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
}

func (c *Connection) Publish(event *nostr.EventEnvelope) {
	logger.Info("Publishing event from client")
	result := make(chan nostr.Envelope)
	req := &models.PubSubEnvelope{
		ClientID: c.id,
		Event:    event,
		Result:   result,
	}
	ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
	defer cancel()
	go func() {
		select {
		case c.Relay.PublishChannel <- req:
		case <-ctx.Done():
			logger.Error("Failed to send event to PublishChannel:", ctx.Err())
			result <- &nostr.ClosedEnvelope{SubscriptionID: *event.SubscriptionID, Reason: ctx.Err().Error()}
			return
		}
	}()
	select {
	case res := <-result:
		c.Write(res)
	case <-ctx.Done():
		logger.Error("Publishing event timed out:", ctx.Err())
	}
}

func (c *Connection) Write(env nostr.Envelope) error {
	c.Lock()
	defer c.Unlock()
	if c.ctx.Err() != nil {
		return fmt.Errorf("connection closed")
	}
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
	c.Lock()
	defer c.Unlock()
	if c.ctx.Err() != nil {
		return fmt.Errorf("connection closed")
	}
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

	for res := range result {
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
}

func (c *Connection) UnSubscribe(subID string) {
	logger.Info("Client unsubscribed")
	c.Subscriptions.Delete(subID)
}

func (c *Connection) Disconnect() {
	logger.Info("Client disconnected")
	c.cancel() // Cancel all goroutines
	c.Subscriptions.Range(func(key string, value nostr.ReqEnvelope) bool {
		c.Write(&nostr.ClosedEnvelope{
			SubscriptionID: key,
			Reason:         "",
		})
		c.UnSubscribe(key)
		return true
	})
	if c.Connection != nil {
		c.Connection.Close()
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
