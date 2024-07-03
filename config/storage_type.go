package config

import (
	"fmt"
	"strings"
)

// StorageType is a custom type representing the storage type enumeration.
type StorageType string

const (
	Postgres StorageType = "postgres"
	MySQL    StorageType = "mysql"
	SQLite   StorageType = "sqlite3"
	MongoDB  StorageType = "mongodb"
)

type DatabaseStorage []StorageType

var DatabaseStorageTypes = DatabaseStorage{
	Postgres,
	MySQL,
	SQLite,
	MongoDB,
}

var SQLDatabaseTypes = DatabaseStorage{
	Postgres,
	MySQL,
	SQLite,
}

func (s DatabaseStorage) Contains(item StorageType) bool {
	for _, v := range s {
			if v == item {
					return true
			}
	}
	return false
}


// IsValid checks if the storage type is valid.
func (d StorageType) IsValid() bool {
	switch d {
	case Postgres, MySQL, SQLite, MongoDB:
		return true
	}
	return false
}

// String returns the string representation of the storage type.
func (d StorageType) String() string {
	return string(d)
}

// UnmarshalText unmarshals a text value into a StorageType, enforcing validation.
func (d *StorageType) UnmarshalText(text []byte) error {
	dbType := StorageType(strings.ToLower(string(text)))
	if !dbType.IsValid() {
		return fmt.Errorf("invalid storage type: %s", text)
	}
	*d = dbType
	return nil
}
