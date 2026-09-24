package db

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

const (
	subsequentOrderType = "Subsequent"
	placedOrderStatus   = "Placed"
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
		activeCadenceOrder := tx.
			Table("test_orders").
			Select("1").
			Where("test_orders.patient_id = cadence_items.patient_id").
			//Where("test_orders.cadence_item_id IS NOT NULL").
			Where("test_orders.test_type = ?", subsequentOrderType).
			Where("test_orders.order_status = ?", placedOrderStatus).
			Where("test_orders.deleted_at IS NULL")

		skipped := tx.
			Model(&CadenceItem{}).
			Where("item_status = ?", "Future").
			Where("cadence_date <= ?", today).
			Where("EXISTS (?)", activeCadenceOrder).
			Update("item_status", "Skipped")
		if skipped.Error != nil {
			return skipped.Error
		}

		log.Printf("# of due Items skipped because of an existing active cadence order:%d", skipped.RowsAffected)

		if err := tx.
			Where("item_status = ?", "Future").
			Where("cadence_date <= ?", activateUntil).
			Where("NOT EXISTS (?)", activeCadenceOrder).
			Find(&items).Error; err != nil {
			return err
		}

		log.Printf("# of Items found to be activated:%d", len(items))

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
