package test

import (
	"context"
	"fmt"
	"os"
	"testing"

	_ "nexa/addons/base"
	_ "nexa/addons/contacts"
	_ "nexa/addons/mail"
	_ "nexa_addons/crm"
	_ "nexa_addons/product"
	_ "nexa_addons/utm"

	"nexa/core/orm"
	"nexa/core/server"
	"nexa/core/server/config"
)

func TestLeadSetWon(t *testing.T) {
	conf := os.Getenv("NEXA_CONF")
	if conf == "" {
		t.Skip("set NEXA_CONF to nexa.conf path")
	}
	if err := server.LoadConfig(conf); err != nil {
		t.Fatal(err)
	}
	c := config.AppConfig
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DbHost, c.DbPort, c.DbUser, c.DbPass, c.DbName, c.DbSslMode)
	server.InitDB(dsn)
	ctx := orm.ContextWithBypass(context.Background(), true)

	rows, err := orm.SearchLimit(ctx, "crm.lead", [][]interface{}{{"active", "=", true}}, 1)
	if err != nil || len(rows) == 0 {
		t.Skip("no crm.lead rows — install crm and seed data first")
	}
	id, _ := orm.CoerceInt64(rows[0]["id"])
	if _, err := orm.RunObjectAction(ctx, "crm.lead", int(id), "action_set_won", nil); err != nil {
		t.Fatalf("action_set_won: %v", err)
	}
	lead, _ := orm.SearchOne(ctx, "crm.lead", map[string]interface{}{"id": id})
	if orm.AsString(lead["won_status"]) != "won" {
		t.Fatalf("expected won, got %v", lead["won_status"])
	}
}
