package db

import "gorm.io/gorm"

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&CadenceItem{},
		// future cadence tables go here
	)
}
