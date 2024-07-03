package storage

import (
	"context"
	"fmt"

	"github.com/nbd-wtf/go-nostr"
	"sharegap.net/nostrodomo/config"
	"sharegap.net/nostrodomo/models"
)

type Storage interface {
	Connect(eventChannel chan *nostr.Event)
	Disconnect() error
	StoreEvent(ctx context.Context, event *models.PubSubEnvelope)
	FetchEvents(ctx context.Context, req *models.PubSubEnvelope)
	ServicWorker(opChan <-chan *models.PubSubEnvelope)
	//DeleteExpiredEvents(ctx context.Context) error
}

func InitStorage(settings *config.StorageConfig) (Storage, error) {
	switch {
	case config.DatabaseStorageTypes.Contains(settings.Type):
		return NewDatabase(settings)
	default:
		return nil, fmt.Errorf("unsupported storage type %s", settings.Type.String())
	}
}

type FilterWithIndex struct {
	Index  int
	Filter nostr.Filter
}
