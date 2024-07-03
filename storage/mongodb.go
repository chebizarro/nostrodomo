package storage

import (
	"context"
	"fmt"

	"github.com/nbd-wtf/go-nostr"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"sharegap.net/nostrodomo/config"
	"sharegap.net/nostrodomo/models"
)

type MongoDB struct {
	client       *mongo.Client
	database     *mongo.Database
	collection   *mongo.Collection
	eventChannel chan *nostr.Event
}

func NewMongoDatabase(settings *config.StorageConfig) (Database, error) {

	clientOptions := options.Client().ApplyURI(settings.Host)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	err = client.Ping(context.Background(), readpref.Primary())
	if err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	database := client.Database("nostr")
	collection := database.Collection("events")

	// Ensure indexes
	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "pubkey", Value: 1}}},
		{Keys: bson.D{{Key: "created_at", Value: -1}}},
		{Keys: bson.D{{Key: "kind", Value: 1}}},
		{Keys: bson.D{{Key: "tags", Value: 1}}},
	}
	_, err = collection.Indexes().CreateMany(context.Background(), indexes)
	if err != nil {
		return nil, fmt.Errorf("failed to create indexes: %w", err)
	}

	return &MongoDB{
		client:     client,
		database:   database,
		collection: collection,
	}, nil
}

func (db *MongoDB) Connect(eventChannel chan *nostr.Event) {
	db.eventChannel = eventChannel
}

func (db *MongoDB) Disconnect() error {
	return db.client.Disconnect(context.Background())
}

func (db *MongoDB) StoreEvent(ctx context.Context, pub *models.PubSubEnvelope) {
	responseChan := pub.Result
	e := pub.Event.(*nostr.EventEnvelope)

	go func() {
		defer close(responseChan)
		doc := bson.M{
			"id":         e.Event.ID,
			"pubkey":     e.Event.PubKey,
			"created_at": e.Event.CreatedAt,
			"kind":       e.Event.Kind,
			"tags":       e.Event.Tags,
			"raw":        e.Event.String(),
		}
		_, err := db.collection.InsertOne(ctx, doc)
		if err != nil {
			responseChan <- &nostr.OKEnvelope{EventID: e.Event.ID, OK: false, Reason: err.Error()}
			return
		}
		db.eventChannel <- &e.Event
		responseChan <- &nostr.OKEnvelope{EventID: e.Event.ID, OK: true, Reason: ""}
	}()

}

func (db *MongoDB) FetchEvents(ctx context.Context, sub *models.PubSubEnvelope) {
	responseChan := sub.Result
	req := sub.Event.(*nostr.ReqEnvelope)
	go func() {
		defer close(responseChan)
		for _, filter := range req.Filters {
			query := buildMongoQueryForFilter(filter)
			cursor, err := db.collection.Find(ctx, query)
			if err != nil {
				responseChan <- &nostr.ClosedEnvelope{SubscriptionID: req.SubscriptionID, Reason: err.Error()}
				return
			}
			for cursor.Next(ctx) {
				var result bson.M
				if err := cursor.Decode(&result); err != nil {
					responseChan <- &nostr.ClosedEnvelope{SubscriptionID: req.SubscriptionID, Reason: err.Error()}
					return
				}

				responseChan <- &models.RawEventEnvelope{SubscriptionID: &req.SubscriptionID, RawEvent: result["raw"].([]byte)}
			}
			cursor.Close(ctx)
		}
	}()
}

func buildMongoQueryForFilter(filter nostr.Filter) bson.M {
	query := bson.M{}

	if len(filter.IDs) > 0 {
		query["id"] = bson.M{"$in": filter.IDs}
	}
	if len(filter.Authors) > 0 {
		query["pubkey"] = bson.M{"$in": filter.Authors}
	}
	if len(filter.Kinds) > 0 {
		query["kind"] = bson.M{"$in": filter.Kinds}
	}
	for key, values := range filter.Tags {
		query["tags"] = bson.M{"$elemMatch": bson.M{"$eq": key + ":" + values[0]}}
	}
	if filter.Since != nil {
		query["created_at"] = bson.M{"$gte": filter.Since}
	}
	if filter.Until != nil {
		query["created_at"] = bson.M{"$lte": filter.Until}
	}
	if filter.Limit > 0 {
		query["limit"] = filter.Limit
	}

	return query
}

func (db *MongoDB) ServicWorker(opChan <-chan *models.PubSubEnvelope) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for op := range opChan {
		switch op.Event.(type) {
		case *nostr.EventEnvelope:
			db.StoreEvent(ctx, op)
		case *nostr.ReqEnvelope:
			db.FetchEvents(ctx, op)
		}
	}

}
