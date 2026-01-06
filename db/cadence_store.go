package db

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type CadenceStore struct {
	*gorm.DB
}

func NewCadenceStore(db *gorm.DB) *CadenceStore {
	return &CadenceStore{DB: db}
}

func (c CadenceStore) DeleteNonFulfilledCadenceItems(
	tx *gorm.DB,
	patientID uint,
) error {

	fulfilledStatuses := []string{"Fulfilled"}

	// Select items that needs to be deleted
	var itemsToDelete []CadenceItem
	if err := tx.Where("patient_id = ?", patientID).
		Where("item_status NOT IN ?", fulfilledStatuses).
		Find(&itemsToDelete).Error; err != nil {

		return fmt.Errorf("failed to select non-fulfilled cadence items: %w", err)
	}

	if len(itemsToDelete) == 0 {
		return nil
	}

	// Extract IDs
	ids := make([]uint, len(itemsToDelete))
	for i, item := range itemsToDelete {
		ids[i] = item.ID
	}

	// Mark published = false BEFORE deletion
	if err := tx.Model(&CadenceItem{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"published": false,
		}).Error; err != nil {

		return fmt.Errorf("failed to reset published flag before deletion: %w", err)
	}

	// Delete the cadence items
	if err := tx.Where("id IN ?", ids).
		Delete(&CadenceItem{}).Error; err != nil {

		return fmt.Errorf("failed to delete non-fulfilled cadence items: %w", err)
	}

	return nil
}

func (c CadenceStore) ActivateUpcomingCadenceItems() error {
	today := time.Now().Truncate(24 * time.Hour)
	activateUntil := today.AddDate(0, 0, 7)

	return c.Model(&CadenceItem{}).
		Where("item_status = ?", "Future").
		Where("cadence_date <= ?", activateUntil).
		Updates(map[string]interface{}{
			"item_status": "Pending",
		}).Error
}
