package relay

import (
	"log"
	"net/http"

	"github.com/nbd-wtf/go-nostr"
	"github.com/puzpuzpuz/xsync/v3"
	"sharegap.net/nostrodomo/logger"
	"sharegap.net/nostrodomo/models"
	"sharegap.net/nostrodomo/storage"
)

type Relay struct {
	// Relay storage
	Storage storage.Storage
	// Registered Connections
	Connections *xsync.MapOf[int64, Connection]
	// Connection index counter
	Counter *xsync.Counter
	// Inbound events from storage
	EventChannel chan *nostr.Event
	// Outbound events to the storage worker
	StorageChannel chan *models.PubSubEnvelope
	// Inbound messages from the clients.
	PublishChannel chan *models.PubSubEnvelope
	// Register requests from the clients.
	ConnectChannel chan *Connection
	// Unregister requests from clients.
	DisconnectChannel chan *Connection
	// Subscription request from clients.
	SubscriptionChannel chan *models.PubSubEnvelope
}

func NewRelay(store storage.Storage) *Relay {

	relay := &Relay{
		Storage:           store,
		Connections:       xsync.NewMapOf[int64, Connection](),
		Counter:           xsync.NewCounter(),
		StorageChannel:    make(chan *models.PubSubEnvelope),
		EventChannel:      make(chan *nostr.Event),
		PublishChannel:    make(chan *models.PubSubEnvelope),
		ConnectChannel:    make(chan *Connection),
		DisconnectChannel: make(chan *Connection),
		SubscriptionChannel: make(chan *models.PubSubEnvelope),
	}
	store.Connect(relay.EventChannel)

	logger.Info("Starting Storage Service Worker")
	go store.ServicWorker(relay.StorageChannel)

	return relay
}

// The relay main loop
func (r *Relay) Run() {
	for {
		select {
		case client := <-r.ConnectChannel:
			r.Connect(client)
		case client := <-r.DisconnectChannel:
			r.Disconnect(client)
		case event := <-r.PublishChannel:
			r.Publish(event)
		case sub := <-r.SubscriptionChannel:
			r.Subscribe(sub)
		}
	}
}

// All connections from clients
func (r *Relay) Serve(w http.ResponseWriter, h *http.Request) {

	logger.Info("Client connected: ", h.Host)
	conn, err := upgrader.Upgrade(w, h, nil)
	if err != nil {
		logger.Error("Error upgrading Websocket:", err)
		log.Println(err)
		return
	}

	client := &Connection{
		Relay:         r,
		Connection:    conn,
		Subscriptions: xsync.NewMapOf[string, nostr.ReqEnvelope](),
		EventChannel:  r.EventChannel,
	}
	r.ConnectChannel <- client

	go client.Listen()
	go client.Read()
}

// Accept a connection from a client
func (r *Relay) Connect(conn *Connection) {
	conn.id = r.Counter.Value()
	r.Connections.Store(conn.id, *conn)
	r.Counter.Inc()
}

// Disconnects client from the relay.
func (r *Relay) Disconnect(connex *Connection) {
	connex.Disconnect()
	r.Connections.Delete(connex.id)
}

func (r *Relay) Publish(env *models.PubSubEnvelope) {
	logger.Debug("Event received:", env)
	r.StorageChannel <- env
}

func (r *Relay) Subscribe(sub *models.PubSubEnvelope) {
	logger.Info("Subscription received from Client")
	r.StorageChannel <- sub
}

func (r *Relay) Shutdown () {
	logger.Info("Shutting down the Relay")
	r.Connections.Range(
		func(key int64, value Connection) bool {
			r.Disconnect(&value)
			return true
		},
	)
	logger.Info("Relay shut down")
}