package cadence

import (
	"strings"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"
)

func TestDueCadenceItemsExcludeOpenOrders(t *testing.T) {
	for _, mobileOnly := range []bool{false, true} {
		name := "all collection methods"
		if mobileOnly {
			name = "mobile collection"
		}
		t.Run(name, func(t *testing.T) {
			db, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{DryRun: true})
			if err != nil {
				t.Fatal(err)
			}
			var query string
			if err := db.Callback().Query().After("gorm:query").Register("capture_due_items", func(tx *gorm.DB) {
				query = tx.Statement.SQL.String()
			}); err != nil {
				t.Fatal(err)
			}
			svc := New(db, DefaultConfig())
			if mobileOnly {
				_, err = svc.GetDueMobileCadenceItems()
			} else {
				_, err = svc.GetDueCadenceItems()
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"item_status = ?", "cadence_date <= ?", "NOT EXISTS (SELECT 1 FROM", "test_orders.test_name = ?", "test_orders.order_status NOT IN (?,?)", "test_orders.deleted_at IS NULL", "ORDER BY cadence_date ASC"} {
				if !strings.Contains(query, want) {
					t.Fatalf("missing %q: %s", want, query)
				}
			}
			if strings.Contains(query, "blood_collection_method = ?") != mobileOnly {
				t.Fatalf("incorrect collection method filter: %s", query)
			}
		})
	}
}
