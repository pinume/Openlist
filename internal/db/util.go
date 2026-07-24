package db

import (
	"fmt"

	"gorm.io/gorm"
)

func columnName(name string) string {
	return fmt.Sprintf("`%s`", name)
}

func addStorageOrder(db *gorm.DB) *gorm.DB {
	return db.Order(fmt.Sprintf("%s, %s", columnName("order"), columnName("id")))
}
