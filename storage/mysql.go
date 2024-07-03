package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/VauntDev/tqla"
	_ "github.com/go-sql-driver/mysql"
	"github.com/nbd-wtf/go-nostr"
)

type MySQLDB struct {
	db            *sql.DB
	tqlaT         TQLATemplate
	insertStmt    *sql.Stmt
	queryTemplate string
}

func (db *MySQLDB) Connect(uri string) error {
	ctx := context.Background()
	database, err := sql.Open("mysql", uri)
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL: %w", err)
	}
	db.db = database

	initSQL, err := db.loadSQLFile("database/sql/mysql_init.sql")
	if err != nil {
		return fmt.Errorf("failed to load init SQL: %w", err)
	}

	_, err = db.db.ExecContext(ctx, initSQL)
	if err != nil {
		return fmt.Errorf("failed to execute init SQL: %w", err)
	}

	insertSQL, err := db.loadSQLFile("database/sql/insert_event.sql")
	if err != nil {
		return fmt.Errorf("failed to load insert event SQL: %w", err)
	}

	db.insertStmt, err = db.db.PrepareContext(ctx, insertSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare insert event statement: %w", err)
	}

	tqlaT, err := NewTQLATemplate(tqla.WithPlaceHolder(tqla.Question))
	if err != nil {
		return fmt.Errorf("failed to initialize tqla: %w", err)
	}
	db.tqlaT = tqlaT

	queryTemplate, err := db.loadSQLFile("database/sql/query_template.sql")
	if err != nil {
		return fmt.Errorf("failed to load query template: %w", err)
	}
	db.queryTemplate = queryTemplate

	return nil
}

func (db *MySQLDB) Disconnect() error {
	if db.insertStmt != nil {
		db.insertStmt.Close()
	}
	db.db.Close()
	return nil
}

func (db *MySQLDB) StoreEvent(ctx context.Context, event *nostr.Event) error {
	_, err := db.insertStmt.ExecContext(ctx, event.ID, event.PubKey, time.Unix(int64(event.CreatedAt), 0), event.Kind, event.Tags, event.String())
	if err != nil {
		return fmt.Errorf("failed to store event: %w", err)
	}
	return nil
}

func (db *MySQLDB) FetchEvents(ctx context.Context, filters []nostr.Filter) (map[string][]byte, error) {
	results := make(map[string][]byte)

	for _, filter := range filters {
		query, args, err := db.buildSQLQueryForFilter(db.tqlaT, db.queryTemplate, filter)
		if err != nil {
			return nil, fmt.Errorf("failed to build query: %w", err)
		}
		rows, err := db.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to execute query: %w", err)
		}

		for rows.Next() {
			var id string
			var raw []byte
			if err := rows.Scan(&id, &raw); err != nil {
				return nil, fmt.Errorf("failed to scan row: %w", err)
			}
			results[id] = raw
		}
		rows.Close()
	}

	return results, nil
}

func (db *MySQLDB) buildSQLQueryForFilter(t TQLATemplate, queryTemplate string, filter nostr.Filter) (string, []interface{}, error) {
	query, args, err := t.Compile(queryTemplate, filter)
	if err != nil {
		return "", nil, fmt.Errorf("failed to compile query: %w", err)
	}
	return query, args, nil
}

func (db *MySQLDB) loadSQLFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
