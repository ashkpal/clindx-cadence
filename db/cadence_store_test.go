package db

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"
)

func TestOpenPatientOrders(t *testing.T) {
	db, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	query := openPatientOrders(db).Find(&[]struct{}{}).Statement
	for _, want := range []string{
		"test_orders.patient_id = cadence_items.patient_id",
		"test_orders.test_name = ?",
		"test_orders.order_status NOT IN (?,?)",
		"test_orders.deleted_at IS NULL",
	} {
		if !strings.Contains(query.SQL.String(), want) {
			t.Fatalf("missing %q: %s", want, query.SQL.String())
		}
	}
	if strings.Contains(query.SQL.String(), "test_type") || strings.Contains(query.SQL.String(), "practice_id") {
		t.Fatalf("baseline orders and orders at other practices must also block cadence: %s", query.SQL.String())
	}
	want := []interface{}{"NeXT Personal Dx", "Complete", "Cancelled"}
	if !reflect.DeepEqual(query.Vars, want) {
		t.Fatalf("filters = %v, want %v", query.Vars, want)
	}
}

func TestWithoutOpenOrdersPreservesOuterFilters(t *testing.T) {
	db, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	cutoff := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	statement := db.Scopes(WithoutOpenOrders).
		Where("item_status = ? AND cadence_date <= ?", "Pending", cutoff).
		Find(&[]CadenceItem{}).Statement
	query := statement.SQL.String()
	for _, want := range []string{"item_status = ? AND cadence_date <= ?", "NOT EXISTS (SELECT 1 FROM", "test_orders.patient_id = cadence_items.patient_id", "test_orders.deleted_at IS NULL"} {
		if !strings.Contains(query, want) {
			t.Fatalf("missing %q: %s", want, query)
		}
	}
	if strings.Count(query, "item_status") != 1 || strings.Count(query, "cadence_date") != 1 {
		t.Fatalf("outer cadence filters leaked into the order subquery: %s", query)
	}
}
