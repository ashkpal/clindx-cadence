package db

import (
	"time"

	"gorm.io/gorm"
)

type CadenceItem struct {
	gorm.Model

	PatientID  uint   `gorm:"not null;index:idx_cadence_patient_status,priority:1"`
	ItemStatus string `gorm:"size:50;not null;default:'Future';index:idx_cadence_patient_status,priority:2"`
	PracticeID uint   `gorm:"not null;index:idx_cadence_practice"`

	TRFID uint `gorm:"index"`

	CadenceDate           time.Time  `gorm:"type:date"`
	OrderDate             *time.Time `gorm:"type:date"`
	BloodCollectionMethod string     `gorm:"size:100"`
	BloodCollectionDate   *time.Time `gorm:"type:date"`

	Active    bool `gorm:"default:false"`
	Published bool `gorm:"default:false"`
}
