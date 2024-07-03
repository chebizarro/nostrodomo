package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/VauntDev/tqla"
	_ "github.com/lib/pq"
	"github.com/nbd-wtf/go-nostr"
)

type PostgresDB struct {
	db            *sql.DB
	tqlaT         TQLATemplate
	insertStmt    *sql.Stmt
	queryTemplate string
	eventChannel  chan *nostr.Event
}

func (db *PostgresDB) Connect(uri string, eventChannel chan *nostr.Event) error {
	ctx := context.Background()
	database, err := sql.Open("postgres", uri)
	if err != nil {
		return fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}
	db.db = database

	initSQL, err := loadSQLFile("database/sql/postgres_init.sql")
	if err != nil {
		return fmt.Errorf("failed to load init SQL: %w", err)
	}

	_, err = db.db.ExecContext(ctx, initSQL)
	if err != nil {
		return fmt.Errorf("failed to execute init SQL: %w", err)
	}

	insertSQL, err := loadSQLFile("database/sql/insert_event.sql")
	if err != nil {
		return fmt.Errorf("failed to load insert event SQL: %w", err)
	}

	db.insertStmt, err = db.db.PrepareContext(ctx, insertSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare insert event statement: %w", err)
	}

	tqlaT, err := NewTQLATemplate(tqla.WithPlaceHolder(tqla.Dollar))
	if err != nil {
		return fmt.Errorf("failed to initialize tqla: %w", err)
	}
	db.tqlaT = tqlaT

	queryTemplate, err := loadSQLFile("database/sql/query_template.sql")
	if err != nil {
		return fmt.Errorf("failed to load query template: %w", err)
	}
	db.queryTemplate = queryTemplate

	db.eventChannel = eventChannel

	return nil
}

func (db *PostgresDB) Disconnect() error {
	if db.insertStmt != nil {
		db.insertStmt.Close()
	}
	db.db.Close()
	return nil
}

func (db *PostgresDB) StoreEvent(ctx context.Context, event *nostr.Event) <-chan nostr.Envelope {
	responseChan := make(chan nostr.Envelope, 1)

	go func() {
		defer close(responseChan)
		_, err := db.insertStmt.ExecContext(ctx, event.ID, event.PubKey, time.Unix(int64(event.CreatedAt), 0), event.Kind, event.Tags, event.String())
		if err != nil {
			responseChan <- &nostr.OKEnvelope{EventID: event.ID, OK: false, Reason: err.Error()}
			return
		}
		db.eventChannel <- event
		responseChan <- &nostr.OKEnvelope{EventID: event.ID, OK: true, Reason: ""}
	}()

	return responseChan
}

func (db *PostgresDB) FetchEvents(ctx context.Context, req *nostr.ReqEnvelope) <-chan nostr.Envelope {
	responseChan := make(chan nostr.Envelope)

	go func() {
		defer close(responseChan)
		/*
		for _, filter := range req.Filters {

			query, args, err := buildSQLQueryForFilter(db.tqlaT, db.queryTemplate, nil)
			if err != nil {
				responseChan <- &nostr.ClosedEnvelope{SubscriptionID: req.SubscriptionID, Reason: err.Error()}
				return
			}
			rows, err := db.db.QueryContext(ctx, query, args...)
			if err != nil {
				responseChan <- &nostr.ClosedEnvelope{SubscriptionID: req.SubscriptionID, Reason: err.Error()}
				return
			}

			for rows.Next() {
				var id string
				var raw []byte
				if err := rows.Scan(&id, &raw); err != nil {
					responseChan <- &nostr.ClosedEnvelope{SubscriptionID: req.SubscriptionID, Reason: err.Error()}
					return
				}
				event := nostr.Event{}
				if err := event.UnmarshalJSON(raw); err != nil {
					responseChan <- &nostr.ClosedEnvelope{SubscriptionID: req.SubscriptionID, Reason: err.Error()}
					return
				}
				responseChan <- &nostr.EventEnvelope{SubscriptionID: &req.SubscriptionID, Event: event}
			}
			rows.Close()
		}
			*/
		esoe := nostr.EOSEEnvelope("")
		responseChan <- &esoe
}()

	return responseChan
}

