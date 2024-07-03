package storage

import (
	"testing"

	"github.com/VauntDev/tqla"
	"github.com/nbd-wtf/go-nostr"
	"log"
	"github.com/stretchr/testify/assert"
)


func TestQueryTemplates(t *testing.T) {
	tests := []struct {
		name    string
		filters []nostr.Filter
		want    string
	}{
		{
			name: "Basic Filter",
			filters: []nostr.Filter{
				{
					IDs:     []string{"id1", "id2"},
					Kinds:   []int{1, 2},
					Authors: []string{"author1", "author2"},
					Tags:    map[string][]string{"e": {"tag1", "tag2"}},
					Since:   (*nostr.Timestamp)(int64Ptr(1609459200)),
					Until:   (*nostr.Timestamp)(int64Ptr(1640995200)),
					Limit:   10,
				},
			},
			want: "SELECT events.id, events.raw FROM events LEFT JOIN tags ON events.id = tags.event_id WHERE (id IN ('id1', 'id2') AND kind IN (1, 2) AND pubkey IN ('author1', 'author2') AND (e IN ('tag1', 'tag2')) AND created_at > 1609459200 AND created_at < 1640995200 LIMIT 10)",
		},
		{
			name: "Multiple Filters",
			filters: []nostr.Filter{
				{
					IDs:   []string{"id1"},
					Limit: 5,
				},
				{
					Authors: []string{"author1"},
					Tags:    map[string][]string{"p": {"tag1"}},
					Limit:   5,
				},
			},
			want: "SELECT events.id, events.raw FROM events LEFT JOIN tags ON events.id = tags.event_id WHERE (id IN ('id1') LIMIT 5) OR (pubkey IN ('author1') AND (p IN ('tag1')) LIMIT 5)",
		},
		{
			name: "No Limit",
			filters: []nostr.Filter{
				{
					IDs:   []string{"id1"},
					Limit: 0,
				},
			},
			want: "SELECT events.id, events.raw FROM events LEFT JOIN tags ON events.id = tags.event_id WHERE (id IN ('id1'))",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new tqla template instance
			tq, err := tqla.New(tqla.WithPlaceHolder(tqla.Dollar))
			assert.NoError(t, err)

			// Read the SQL template from file
			template, err := loadSQLFile("sql/sqlite3_filter.sql.tmpl")
			if err != nil {
				log.Fatalf("an error: %v", err)
			}
			// Compile the query using the template and filters
			query, _, err := tq.Compile(template, tt.filters)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, query)
		})
	}
}

func int64Ptr(i int64) *int64 {
	return &i
}
