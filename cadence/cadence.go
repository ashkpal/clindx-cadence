package cadence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ashkpal/clindx-cadence/db"
	"gorm.io/gorm"
)

type AlertPublisher interface {
	CreateAlerts(
		ctx context.Context,
		alerts []db.CadenceItem,
	) error
}

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
	ToggleCollection(tx *gorm.DB, cadenceItemID uint, bloodCollectionMethod string) error
	UpdateCadenceItem(tx *gorm.DB, cadenceItemID uint, itemStatus string) error
	GetCadenceItemWithinNDays(daysNum int) (*db.CadenceItem, error)
}

func New(dbConn *gorm.DB) Service {
	return &service{
		store: db.NewCadenceStore(dbConn),
	}
}

func NewWithAlertPublisher(
	dbConn *gorm.DB,
	alertPublisher AlertPublisher,
) Service {
	return &service{
		store:          db.NewCadenceStore(dbConn),
		alertPublisher: alertPublisher,
	}
}

type service struct {
	store          *db.CadenceStore
	alertPublisher AlertPublisher // optional
}

func (s *service) ActivateUpcoming() error {
	items, err := s.store.ActivateUpcomingCadenceItems()
	if err != nil {
		return err
	}

	if len(items) == 0 || s.alertPublisher == nil {
		return nil
	}

	// ✅ Filter only Mobile blood collection items
	var mobileItems []db.CadenceItem
	for _, item := range items {
		if item.BloodCollectionMethod == "Mobile Phlebotomy" && !item.Published {
			mobileItems = append(mobileItems, item)
		}
	}

	if len(mobileItems) == 0 {
		return nil
	}

	//alerts := buildCadenceItemViews(mobileItems)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.alertPublisher.CreateAlerts(ctx, mobileItems); err != nil {
		return fmt.Errorf("salesforce publish failed: %w", err)
	}

	// ✅ Mark as published *after* successful SF call
	if err := s.store.MarkPublished(mobileItems); err != nil {
		return fmt.Errorf("failed to mark cadence items published: %w", err)
	}

	return nil
}

func (s *service) ToggleCollection(tx *gorm.DB, cadenceItemID uint, bloodCollectionMethod string) error {

	if err := tx.Model(&db.CadenceItem{}).
		Where("id = ?", cadenceItemID).
		Update("blood_collection_method", bloodCollectionMethod).Error; err != nil {
		return fmt.Errorf("update cadenceItem for mobile: %w", err)
	}
	return nil
}

func (s *service) GetCadenceItemWithinNDays(daysNum int) (*db.CadenceItem, error) {
	var item db.CadenceItem

	now := time.Now().Truncate(24 * time.Hour)
	startDate := now.AddDate(0, 0, -daysNum)
	endDate := now.AddDate(0, 0, daysNum)

	err := s.store.
		Where("cadence_date BETWEEN ? AND ?", startDate, endDate).
		Order("cadence_date ASC").
		First(&item).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &item, nil
}

func (s *service) UpdateCadenceItem(tx *gorm.DB, cadenceItemID uint, itemStatus string) error {

	if err := tx.Model(&db.CadenceItem{}).
		Where("id = ?", cadenceItemID).
		Update("item_status", itemStatus).Error; err != nil {
		return fmt.Errorf("update cadenceItem status: %w", err)
	}
	return nil
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
