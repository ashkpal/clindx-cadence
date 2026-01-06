package cadence

import (
	"time"

	"github.com/ashkpal/clindx-cadence/db"
	"gorm.io/gorm"
)

type ScheduleRequest struct {
	PatientID             uint
	TestOrderID           *uint
	BloodCollectionMethod string
	CadenceDays           int
	StartDate             time.Time
}

type Service interface {
	Schedule(req ScheduleRequest) error
	ActivateUpcoming() error
}

func New(dbConn *gorm.DB) Service {
	return &service{
		store: db.NewCadenceStore(dbConn),
	}
}

type service struct {
	store *db.CadenceStore
}

func (s *service) ActivateUpcoming() error {
	return s.store.ActivateUpcomingCadenceItems()
}

func (s *service) Schedule(req ScheduleRequest) error {
	start := req.StartDate.Truncate(24 * time.Hour)

	// delete old cadence items safely
	return s.store.Transaction(func(tx *gorm.DB) error {

		if err := s.store.DeleteNonFulfilledCadenceItems(tx, req.PatientID); err != nil {
			return err
		}

		next := start.AddDate(0, 0, req.CadenceDays)
		end := start.AddDate(1, 0, 0)

		for d := next; !d.After(end); d = d.AddDate(0, 0, req.CadenceDays) {
			item := db.CadenceItem{
				PatientID:             req.PatientID,
				TestOrderID:           req.TestOrderID,
				CadenceDate:           d,
				ItemStatus:            "Future",
				BloodCollectionMethod: req.BloodCollectionMethod,
				Active:                false,
				Published:             false,
			}

			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}

		return nil
	})
}
