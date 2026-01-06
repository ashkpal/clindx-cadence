package db

import (
	"time"

	"gorm.io/gorm"
)

type CadenceItem struct {
	gorm.Model

	PatientID   uint  `gorm:"not null;index"`
	TestOrderID *uint `gorm:"index"`

	CadenceDate           time.Time  `gorm:"type:date"`
	OrderDate             *time.Time `gorm:"type:date"`
	BloodCollectionMethod string     `gorm:"size:100"`
	BloodCollectionDate   *time.Time `gorm:"type:date"`

	Active     bool   `gorm:"default:false"`
	ItemStatus string `gorm:"size:50;not null;default:'Future';index"`
	Published  bool   `gorm:"default:false;index"`
}
