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

func (c CadenceStore) ActivateUpcomingCadenceItems(daysUntil int) ([]CadenceItem, error) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	activateUntil := today.AddDate(0, 0, daysUntil)

	var items []CadenceItem

	returnItems := []CadenceItem{}

	err := c.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where("item_status = ?", "Future").
			Where("cadence_date <= ?", activateUntil).
			Find(&items).Error; err != nil {
			return err
		}

		if len(items) == 0 {
			return nil
		}

		ids := make([]uint, len(items))
		for i, item := range items {
			ids[i] = item.ID
		}

		if err := tx.
			Model(&CadenceItem{}).
			Where("id IN ?", ids).
			Update("item_status", "Pending").Error; err != nil {
			return err
		}

		returnItems = items
		return nil
	})

	if err != nil {
		return nil, err
	}

	return returnItems, nil
}

func (c CadenceStore) MarkPublished(items []CadenceItem) error {
	ids := make([]uint, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}

	return c.
		Model(&CadenceItem{}).
		Where("id IN ?", ids).
		Update("published", true).
		Error
}
