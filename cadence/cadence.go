package cadence

import (
	"fmt"
	"time"

	"github.com/ashkpal/clindx-cadence/db"
	"gorm.io/gorm"
)

type ScheduleRequest struct {
	PatientID             uint
	TestOrderID           *uint
	PracticeID            uint
	BloodCollectionMethod string
	CadenceDays           int
	StartDate             time.Time
}

type Service interface {
	Schedule(db *gorm.DB, req ScheduleRequest) error
	ActivateUpcoming() error
	GetItemsByPatient(patientID uint) ([]db.CadenceItem, error)
	GetItemsByPractice(patientID uint) ([]db.CadenceItem, error)
	GetPendingItemsByPractice(patientID uint) ([]db.CadenceItem, error)
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

func (s *service) GetItemsByPatient(patientID uint) ([]db.CadenceItem, error) {
	var items []db.CadenceItem
	err := s.store.
		Where("patient_id = ?", patientID).
		Order("cadence_date ASC").
		Find(&items).Error
	return items, err
}

func (s *service) GetItemsByPractice(practiceID uint) ([]db.CadenceItem, error) {
	var items []db.CadenceItem
	err := s.store.
		Where("practice_id = ?", practiceID).
		Order("cadence_date ASC").
		Find(&items).Error
	return items, err
}

func (s *service) GetPendingItemsByPractice(practiceID uint) ([]db.CadenceItem, error) {
	var items []db.CadenceItem
	err := s.store.
		Where("practice_id = ? and item_status = ?", practiceID, "Pending").
		Order("cadence_date ASC").
		Find(&items).Error
	return items, err
}

func (s *service) Schedule(db *gorm.DB, req ScheduleRequest) error {

	if err := s.store.DeleteNonFulfilledCadenceItems(db, req.PatientID); err != nil {
		return err
	}

	items := buildCadenceItemsFrom(req.PatientID, req.TestOrderID, req.PracticeID, req.BloodCollectionMethod, req.CadenceDays, req.StartDate)

	if err := db.Create(&items).Error; err != nil {
		db.Rollback()
		return fmt.Errorf("failed to create new cadence items: %w", err)
	}

	return nil
}

func buildCadenceItemsFrom(
	patientID uint,
	testOrderID *uint,
	practiceID uint,
	method string,
	cadenceDays int,
	start time.Time,
) []db.CadenceItem {

	var items []db.CadenceItem

	start = start.Truncate(24 * time.Hour)
	next := start.AddDate(0, 0, cadenceDays)
	end := start.AddDate(1, 0, 0)

	for d := next; !d.After(end); d = d.AddDate(0, 0, cadenceDays) {
		items = append(items, db.CadenceItem{
			PatientID:             patientID,
			TestOrderID:           testOrderID,
			PracticeID:            practiceID,
			CadenceDate:           d,
			ItemStatus:            "Future",
			BloodCollectionMethod: method,
			Active:                false,
			Published:             false,
		})
	}

	return items
}
