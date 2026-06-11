package db

import (
	"database/sql"
	"fmt"
	"strings"
)

func shimExec(err error, driver string) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "UNIQUE constraint") {
		return fmt.Errorf("document already exists")
	}
	return err
}

func shimQuery(err error, driver string) error {
	if err == sql.ErrNoRows {
		return sql.ErrNoRows
	}
	return err
}
