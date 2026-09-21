package codegen

import "testing"

func TestAccessIdentifierGrammar(t *testing.T) {
	for _, value := range []string{"owner", "customer_id", "Owner1", "_scope"} {
		if !accessIdentifier(value) {
			t.Errorf("rejected identifier %q", value)
		}
	}
	for _, value := range []string{"", " owner", "1owner", "a.b", "customer-id", "id; DROP TABLE x", "владелец"} {
		if accessIdentifier(value) {
			t.Errorf("accepted nonidentifier %q", value)
		}
	}
}

func TestAccessColumnReferencesUseOnlyDeclaredQualifiers(t *testing.T) {
	for _, column := range []string{"customer_id", "orders.customer_id", "public.orders.customer_id"} {
		if err := validateAccessColumnReference(column, "orders", "public.orders"); err != nil {
			t.Errorf("%s: %v", column, err)
		}
	}
	if err := validateAccessColumnReference("o.customer_id", "o"); err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"o.customer_id", "other.customer_id", "customer_id::int", "COALESCE(customer_id,0)", "data->>'owner'", "a.b.c.d", "customer_id;--", " customer_id", `"customer_id"`} {
		if err := validateAccessColumnReference(column, "orders", "public.orders"); err == nil {
			t.Errorf("accepted unbound column or SQL %q", column)
		}
	}
	if err := validateAccessColumnReference("orders.customer_id", "o"); err == nil {
		t.Fatal("a declared alias must hide the original qualifier")
	}
}

func TestAccessWriteColumnCanonicalization(t *testing.T) {
	for _, input := range []string{"customer_id", "CUSTOMER_ID", "Customer_ID"} {
		column, err := canonicalAccessWriteColumn(input)
		if err != nil || column != "customer_id" {
			t.Errorf("%q: expected customer_id, got %q: %v", input, column, err)
		}
	}
	for _, input := range []string{"", " customer_id", "customer_id ", "o.customer_id", "public.orders.customer_id", `"CUSTOMER_ID"`, "customer_id::int", "id; DROP TABLE orders", "customer-id", "1customer", "идентификатор"} {
		if _, err := canonicalAccessWriteColumn(input); err == nil {
			t.Errorf("accepted invalid write column %q", input)
		}
	}
}
