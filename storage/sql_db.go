package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/VauntDev/tqla"
	"sharegap.net/nostrodomo/config"
	"sharegap.net/nostrodomo/logger"
	"sharegap.net/nostrodomo/models"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/nbd-wtf/go-nostr"
)

type SQLDB struct {
	db              *sql.DB
	tqlaT           TQLATemplate
	insertEventStmt *sql.Stmt
	insertTagsStmt  *sql.Stmt
	queryTemplate   string
	eventChannel    chan *nostr.Event
}

func NewSQLDatabase(settings *config.StorageConfig) (*SQLDB, error) {

	ctx := context.Background()
	driverName := settings.Type.String()
	db, err := sql.Open(driverName, settings.GetConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", settings.Type.String(), err)
	}

	initSQL, err := loadSQLFile(fmt.Sprintf("storage/sql/%s_init.sql", driverName))
	if err != nil {
		return nil, fmt.Errorf("failed to load init SQL: %w", err)
	}

	_, err = db.ExecContext(ctx, initSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to execute init SQL: %w", err)
	}

	insertEventSQL, err := loadSQLFile(fmt.Sprintf("storage/sql/%s_insert_event.stmt", driverName))
	if err != nil {
		return nil, fmt.Errorf("failed to load insert event SQL: %w", err)
	}

	insertTagsSQL, err := loadSQLFile(fmt.Sprintf("storage/sql/%s_insert_tags.stmt", driverName))
	if err != nil {
		return nil, fmt.Errorf("failed to load insert tags SQL: %w", err)
	}

	insertEventStmt, err := db.PrepareContext(ctx, insertEventSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare insert event statement: %w", err)
	}

	insertTagsStmt, err := db.PrepareContext(ctx, insertTagsSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare insert tags statement: %w", err)
	}

	tqlaT, err := NewTQLATemplate(tqla.WithPlaceHolder(getPlaceholder(driverName)))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tqla: %w", err)
	}

	queryTemplate, err := loadSQLFile(fmt.Sprintf("storage/sql/%s_filter.sql.tmpl", driverName))
	if err != nil {
		return nil, fmt.Errorf("failed to load query template: %w", err)
	}

	return &SQLDB{
		db:              db,
		tqlaT:           tqlaT,
		insertEventStmt: insertEventStmt,
		insertTagsStmt:  insertTagsStmt,
		queryTemplate:   queryTemplate,
	}, nil

}

func (db *SQLDB) Connect(eventChannel chan *nostr.Event) {
	db.eventChannel = eventChannel
}

func (db *SQLDB) Disconnect() error {
	if db.insertEventStmt != nil {
		db.insertEventStmt.Close()
	}
	if db.insertTagsStmt != nil {
		db.insertTagsStmt.Close()
	}
	db.db.Close()
	return nil
}

func (db *SQLDB) StoreEvent(ctx context.Context, pub *models.PubSubEnvelope) {
	responseChan := pub.Result
	e := pub.Event.(*nostr.EventEnvelope)
	go func() {
		defer close(responseChan)

		_, err := db.insertEventStmt.ExecContext(ctx, e.Event.ID, e.Event.PubKey, time.Unix(int64(e.Event.CreatedAt), 0), e.Event.Kind, e.Event.String())
		if err != nil {
			responseChan <- &nostr.OKEnvelope{EventID: e.Event.ID, OK: false, Reason: err.Error()}
			return
		}
		for _, tag := range e.Event.Tags {
			_, err := db.insertTagsStmt.ExecContext(ctx, e.Event.ID, tag.Key(), tag.Value())
			if err != nil {
				responseChan <- &nostr.OKEnvelope{EventID: e.Event.ID, OK: false, Reason: err.Error()}
				return
			}
		}
		logger.Debug("Event stored:", e.Event.ID)
		db.eventChannel <- &e.Event
		responseChan <- &nostr.OKEnvelope{EventID: e.Event.ID, OK: true, Reason: "Event Processed"}
	}()
}

func (db *SQLDB) FetchEvents(ctx context.Context, sub *models.PubSubEnvelope) {
	responseChan := sub.Result
	req := sub.Event.(*nostr.ReqEnvelope)
	go func() {
		defer close(responseChan)

		query, args, err := buildSQLQueryForFilter(db.tqlaT, db.queryTemplate, &req.Filters)
		if err != nil {
			responseChan <- &nostr.ClosedEnvelope{SubscriptionID: req.SubscriptionID, Reason: err.Error()}
			return
		}
		rows, err := db.db.Query(query, args...)
		if err != nil {
			responseChan <- &nostr.ClosedEnvelope{SubscriptionID: req.SubscriptionID, Reason: err.Error()}
			return
		}
		defer rows.Close()
		var r,v string
		rows.Scan(&r, &v)
		logger.Debug("Result:", r, v)
		for rows.Next() {
			var id string
			var raw []byte
			if err := rows.Scan(&id, &raw); err != nil {
				responseChan <- &nostr.ClosedEnvelope{SubscriptionID: req.SubscriptionID, Reason: err.Error()}
				return
			}
			responseChan <- &models.RawEventEnvelope{SubscriptionID: &req.SubscriptionID, RawEvent: raw}
		}

		esoe := nostr.EOSEEnvelope("")
		responseChan <- &esoe
	}()
}

func buildSQLQueryForFilter(t TQLATemplate, queryTemplate string, filter *nostr.Filters) (string, []interface{}, error) {
	query, args, err := t.Compile(queryTemplate, filter)
	if err != nil {
		return "", nil, fmt.Errorf("failed to compile query: %w", err)
	}
	logger.Debug("SQL Query generated: ", query, " args: ", args)
	return query, args, nil
}

func loadSQLFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	logger.Debug("SQL File loaded from path:", filePath)
	return string(data), nil
}

func getPlaceholder(driverName string) tqla.Placeholder {
	switch driverName {
	case "postgres":
		return tqla.Dollar
	case "mysql":
		return tqla.Question
	case "sqlite3":
		return tqla.Question
	default:
		return tqla.Question
	}
}

func (db *SQLDB) ServicWorker(opChan <-chan *models.PubSubEnvelope) {
	for op := range opChan {
		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
		switch op.Event.(type) {
		case *nostr.EventEnvelope:
			db.StoreEvent(ctx, op)
		case *nostr.ReqEnvelope:
			db.FetchEvents(ctx, op)
		}
	}
}
