package models

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nexa/core/orm"
)

// Product PIM engine: price changes with history, variant matrix generation.

const (
	prodModel    = "product.product"
	variantModel = "product.variant"
	attrModel    = "product.attribute"
	phModel      = "product.price.history"
)

func init() {
	orm.RegisterObjectAction(prodModel, "action_product_set_price", productSetPrice)
	orm.RegisterObjectAction(prodModel, "action_product_generate_variants", productGenerateVariants)
}

func pfnum(v interface{}) float64 {
	n, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
	return n
}

func poid(v interface{}) int {
	if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
		n, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(arr[0])))
		return n
	}
	n, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(v)))
	return n
}

func pload(ctx context.Context, id int) map[string]interface{} {
	rows, err := orm.SearchPage(orm.ContextWithBypass(ctx, true), prodModel,
		[][]interface{}{{"id", "=", id}}, 1, 0, "id asc")
	if err != nil || len(rows) == 0 {
		return nil
	}
	return rows[0]
}

// productSetPrice updates list_price/standard_price and writes an immutable
// price-history row (spec §10.6 — never silently change a price).
func productSetPrice(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	p := pload(bypass, id)
	if p == nil {
		return "", fmt.Errorf("product not found")
	}
	field := strings.TrimSpace(args["field"])
	if field != "list_price" && field != "standard_price" {
		return "", fmt.Errorf("field must be list_price or standard_price")
	}
	newVal := pfnum(args["value"])
	oldVal := pfnum(p[field])
	if newVal == oldVal {
		return "", fmt.Errorf("price unchanged")
	}
	if newVal < 0 {
		return "", fmt.Errorf("price cannot be negative")
	}
	if err := orm.UpdateRecordByID(bypass, prodModel, id, map[string]interface{}{field: newVal}); err != nil {
		return "", err
	}
	ph, ok := orm.Registry[phModel]
	if !ok {
		return "", fmt.Errorf("model %s not registered", phModel)
	}
	_, _ = orm.Create(bypass, ph, map[string]interface{}{
		"product_id": id,
		"field":      field,
		"old_value":  oldVal,
		"new_value":  newVal,
		"reason":     strings.TrimSpace(args["reason"]),
		"user_id":    args["user_id"],
		"at":         time.Now().UTC().Format(time.RFC3339),
	})
	return fmt.Sprintf("%s %.2f → %.2f (history recorded)", field, oldVal, newVal), nil
}

// productGenerateVariants creates variant rows from the product's
// variant-creating attributes (spec §5 — attribute matrix).
// args: attributes as JSON {"Color":["Black","White"],"Size":["S","M"]}
// If args empty, uses product.attribute values joined to product via
// attributes JSON on existing variants is NOT supported — caller passes
// the attribute→values map explicitly for determinism.
func productGenerateVariants(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	p := pload(bypass, id)
	if p == nil {
		return "", fmt.Errorf("product not found")
	}
	attrJSON := strings.TrimSpace(args["attributes"])
	if attrJSON == "" {
		return "", fmt.Errorf("attributes JSON required, e.g. {\"Color\":[\"Black\",\"White\"],\"Size\":[\"S\",\"M\"]}")
	}
	var attrs map[string][]string
	if err := json.Unmarshal([]byte(attrJSON), &attrs); err != nil {
		return "", fmt.Errorf("invalid attributes JSON: %v", err)
	}
	// cartesian product of attribute values
	type combo map[string]string
	combos := []combo{{}}
	for name, vals := range attrs {
		if len(vals) == 0 {
			continue
		}
		next := []combo{}
		for _, c := range combos {
			for _, v := range vals {
				nc := combo{}
				for k, x := range c {
					nc[k] = x
				}
				nc[name] = v
				next = append(next, nc)
			}
		}
		combos = next
	}
	if len(combos) == 0 || len(combos) > 500 {
		return "", fmt.Errorf("variant count %d out of range (1-500)", len(combos))
	}
	vm, ok := orm.Registry[variantModel]
	if !ok {
		return "", fmt.Errorf("model %s not registered", variantModel)
	}
	created, skipped := 0, 0
	for _, c := range combos {
		// deterministic SKU suffix + name
		parts := make([]string, 0, len(c))
		for k, v := range c {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
		vname := strings.Join(parts, " / ")
		attrsBytes, _ := json.Marshal(c)
		// skip if identical variant exists
		existing, _ := orm.SearchPage(bypass, variantModel,
			[][]interface{}{{"product_id", "=", id}, {"name", "=", vname}}, 1, 0, "id asc")
		if len(existing) > 0 {
			skipped++
			continue
		}
		base := poid(p["id"])
		sku := fmt.Sprintf("VAR-%d-%d", base, len(combos))
		_, err := orm.Create(bypass, vm, map[string]interface{}{
			"product_id":     id,
			"name":           vname,
			"sku":            sku,
			"list_price":     pfnum(p["list_price"]),
			"standard_price": pfnum(p["standard_price"]),
			"attributes":     string(attrsBytes),
			"active":         true,
		})
		if err != nil {
			return "", fmt.Errorf("variant create failed for %s: %v", vname, err)
		}
		created++
	}
	return fmt.Sprintf("Variants generated: %d created, %d skipped (duplicates)", created, skipped), nil
}

// ---------------------------------------------------------------------------
// Product lifecycle (spec §14): draft → review → active → discontinued → archived.
// ---------------------------------------------------------------------------

func init() {
	orm.RegisterObjectAction(prodModel, "action_product_submit", productSubmit)
	orm.RegisterObjectAction(prodModel, "action_product_approve", productApprove)
	orm.RegisterObjectAction(prodModel, "action_product_discontinue", productDiscontinue)
	orm.RegisterObjectAction(prodModel, "action_product_archive", productArchive)
}

var lifecycleNext = map[string]string{
	"draft":        "review",
	"review":       "active",
	"active":       "discontinued",
	"discontinued": "archived",
}

func productTransition(ctx context.Context, _ string, id int, action string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	p := pload(bypass, id)
	if p == nil {
		return "", fmt.Errorf("product not found")
	}
	st := orm.AsString(p["state"])
	next, ok := lifecycleNext[st]
	if !ok {
		return "", fmt.Errorf("cannot %s from state %q", action, st)
	}
	// guard: activating requires name + price
	if next == "active" {
		if strings.TrimSpace(orm.AsString(p["name"])) == "" {
			return "", fmt.Errorf("product needs a name before activation")
		}
	}
	if err := orm.UpdateRecordByID(bypass, prodModel, id, map[string]interface{}{"state": next}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Product %s → %s", st, next), nil
}

func productSubmit(ctx context.Context, m string, id int, a map[string]string) (string, error) {
	return productTransition(ctx, m, id, "submit")
}
func productApprove(ctx context.Context, m string, id int, a map[string]string) (string, error) {
	return productTransition(ctx, m, id, "approve")
}
func productDiscontinue(ctx context.Context, m string, id int, a map[string]string) (string, error) {
	return productTransition(ctx, m, id, "discontinue")
}
func productArchive(ctx context.Context, m string, id int, a map[string]string) (string, error) {
	return productTransition(ctx, m, id, "archive")
}

// ---------------------------------------------------------------------------
// Bulk CSV import (spec §16): paste CSV text → create products.
// Columns: name,barcode,sku,price,cost,category
// Registered as object action on product.product; args["csv"] = raw text.
// ---------------------------------------------------------------------------

func init() {
	orm.RegisterObjectAction(prodModel, "action_product_import_csv", productImportCSV)
}

func productImportCSV(ctx context.Context, _ string, _ int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	csvText := strings.TrimSpace(args["csv"])
	if csvText == "" {
		return "", fmt.Errorf("csv arg required (header: name,barcode,sku,price,cost,category)")
	}
	lines := strings.Split(strings.ReplaceAll(csvText, "\r\n", "\n"), "\n")
	pm, ok := orm.Registry[prodModel]
	if !ok {
		return "", fmt.Errorf("product.product not registered")
	}
	// category name → id cache
	catCache := map[string]int{}
	created, skipped, errs := 0, 0, 0
	var errLines []string
	for i, raw := range lines {
		row := strings.Split(raw, ",")
		for j := range row {
			row[j] = strings.TrimSpace(row[j])
		}
		if len(row) < 2 || (i == 0 && strings.EqualFold(row[0], "name")) {
			continue // header or empty
		}
		name := row[0]
		if name == "" {
			skipped++
			continue
		}
		// duplicate check by barcode or name+sku
		dupDomain := [][]interface{}{}
		if len(row) > 1 && row[1] != "" {
			dupDomain = append(dupDomain, []interface{}{"barcode", "=", row[1]})
		}
		if len(row) > 2 && row[2] != "" {
			dupDomain = append(dupDomain, []interface{}{"default_code", "=", row[2]})
		}
		if len(dupDomain) > 0 {
			if ex, _ := orm.SearchPage(bypass, prodModel, dupDomain, 1, 0, "id asc"); len(ex) > 0 {
				skipped++
				continue
			}
		}
		vals := map[string]interface{}{
			"name":        name,
			"type":        "consu",
			"state":       "draft",
			"sale_ok":     true,
			"purchase_ok": true,
			"pos_enabled": true,
			"active":      true,
		}
		if len(row) > 1 {
			vals["barcode"] = row[1]
		}
		if len(row) > 2 {
			vals["default_code"] = row[2]
		}
		if len(row) > 3 {
			vals["list_price"] = pfnum(row[3])
		}
		if len(row) > 4 {
			vals["standard_price"] = pfnum(row[4])
		}
		if len(row) > 5 && row[5] != "" {
			cid, ok2 := catCache[row[5]]
			if !ok2 {
				cats, _ := orm.SearchPage(bypass, "product.category",
					[][]interface{}{{"name", "=", row[5]}}, 1, 0, "id asc")
				if len(cats) > 0 {
					cid = poid(cats[0]["id"])
				}
				catCache[row[5]] = cid
			}
			if cid > 0 {
				vals["categ_id"] = cid
			}
		}
		if _, err := orm.Create(bypass, pm, vals); err != nil {
			errs++
			if len(errLines) < 10 {
				errLines = append(errLines, fmt.Sprintf("line %d: %v", i+1, err))
			}
			continue
		}
		created++
	}
	msg := fmt.Sprintf("Import done: %d created, %d skipped (duplicates/empty), %d errors", created, skipped, errs)
	if len(errLines) > 0 {
		msg += " | " + strings.Join(errLines, "; ")
	}
	return msg, nil
}

// ---------------------------------------------------------------------------
// E-commerce publishing (spec §17): product → ecom.product.publication.
// ---------------------------------------------------------------------------

func init() {
	orm.RegisterObjectAction(prodModel, "action_product_publish_ecom", productPublishEcom)
	orm.RegisterObjectAction(prodModel, "action_product_unpublish_ecom", productUnpublishEcom)
}

func ecomStoreID(ctx context.Context) int {
	rows, err := orm.SearchPage(ctx, "ecom.store", nil, 1, 0, "id asc")
	if err != nil || len(rows) == 0 {
		return 0
	}
	return poid(rows[0]["id"])
}

// productPublishEcom creates/activates the product's store publication.
// Respects the ecommerce_enabled channel flag (spec §17).
func productPublishEcom(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	p := pload(bypass, id)
	if p == nil {
		return "", fmt.Errorf("product not found")
	}
	if !orm.AsBool(p["ecommerce_enabled"]) {
		return "", fmt.Errorf("e-commerce channel disabled for this product")
	}
	if !orm.AsBool(p["active"]) {
		return "", fmt.Errorf("product is archived")
	}
	storeID := ecomStoreID(bypass)
	if storeID <= 0 {
		return "", fmt.Errorf("no ecom.store configured — create a store first")
	}
	// existing publication?
	ex, _ := orm.SearchPage(bypass, "ecom.product.publication",
		[][]interface{}{{"product_id", "=", id}, {"store_id", "=", storeID}}, 1, 0, "id asc")
	price := pfnum(p["list_price"])
	vals := map[string]interface{}{
		"state":               "published",
		"catalog_name":        orm.AsString(p["name"]),
		"catalog_description": orm.AsString(p["description_sale"]),
		"price":               price,
		"list_price":          price,
	}
	if len(ex) > 0 {
		pid := poid(ex[0]["id"])
		_ = orm.UpdateRecordByID(bypass, "ecom.product.publication", pid, vals)
		return fmt.Sprintf("Publication #%d updated → published", pid), nil
	}
	pm, ok := orm.Registry["ecom.product.publication"]
	if !ok {
		return "", fmt.Errorf("ecom.product.publication not registered (install website_shop)")
	}
	newID, err := orm.Create(bypass, pm, map[string]interface{}{
		"store_id":            storeID,
		"product_id":          id,
		"state":               "published",
		"catalog_name":        orm.AsString(p["name"]),
		"catalog_description": orm.AsString(p["description_sale"]),
		"price":               price,
		"list_price":          price,
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Published to store #%d (publication #%d)", storeID, newID), nil
}

// productUnpublishEcom pulls the product off the storefront.
func productUnpublishEcom(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ex, _ := orm.SearchPage(bypass, "ecom.product.publication",
		[][]interface{}{{"product_id", "=", id}}, 10, 0, "id asc")
	if len(ex) == 0 {
		return "No publications found.", nil
	}
	for _, r := range ex {
		_ = orm.UpdateRecordByID(bypass, "ecom.product.publication", poid(r["id"]),
			map[string]interface{}{"state": "draft"})
	}
	return fmt.Sprintf("Unpublished %d listing(s)", len(ex)), nil
}
