package insights

import (
	"database/sql"
	"time"
)

type ModelStore struct {
	db  *sql.DB
	loc *time.Location
}

func NewModelStore(db *sql.DB, timezone string) *ModelStore {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	return &ModelStore{db: db, loc: loc}
}
