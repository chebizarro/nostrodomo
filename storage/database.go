package storage

import (
	"sharegap.net/nostrodomo/config"
)

type Database interface {
	Storage
}

func NewDatabase(settings *config.StorageConfig) (Database, error) {
	if config.SQLDatabaseTypes.Contains(settings.Type) {
		return NewSQLDatabase(settings)
	} else {
		return NewMongoDatabase(settings)
	}
}

