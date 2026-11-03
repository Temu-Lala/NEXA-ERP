-- Surgical reset of the 5 accounting-deep companion modules so a fresh install
-- (with corrected short xmlids) is clean. One transaction; aborts on error.
-- Drops ONLY the 20 registered in-scope tables (never a wildcard account_%, which
-- would hit the anchor). No groups/categories are created by these modules.
BEGIN;

CREATE TEMP TABLE scope_models ON COMMIT DROP AS
  SELECT id, model FROM sys_model
  WHERE module IN ('account_period','account_multicurrency','account_bank','account_asset','account_dimension');

-- 1. Drop physical tables for the in-scope models only (dots -> underscores), CASCADE.
DO $$
DECLARE r RECORD;
BEGIN
  FOR r IN SELECT model FROM scope_models LOOP
    EXECUTE 'DROP TABLE IF EXISTS "' || replace(r.model, '.', '_') || '" CASCADE';
  END LOOP;
END $$;

-- 2. Menus (by module and by xmlid registry).
DELETE FROM sys_menu WHERE module IN
  ('account_period','account_multicurrency','account_bank','account_asset','account_dimension');
DELETE FROM sys_menu WHERE id IN
  (SELECT core_id FROM sys_model_data WHERE module IN
   ('account_period','account_multicurrency','account_bank','account_asset','account_dimension') AND model='sys.menu');

-- 3. Actions, views, access rules (by model, then by xmlid registry).
DELETE FROM sys_action_window WHERE core_model IN (SELECT model FROM scope_models);
DELETE FROM sys_action_window WHERE id IN
  (SELECT core_id FROM sys_model_data WHERE module IN
   ('account_period','account_multicurrency','account_bank','account_asset','account_dimension') AND model IN ('sys.action.window','ir.actions.window'));
DELETE FROM sys_view WHERE model IN (SELECT model FROM scope_models);
DELETE FROM sys_view WHERE id IN
  (SELECT core_id FROM sys_model_data WHERE module IN
   ('account_period','account_multicurrency','account_bank','account_asset','account_dimension') AND model='sys.view');
DELETE FROM sys_access WHERE model IN (SELECT model FROM scope_models);
DELETE FROM sys_access WHERE id IN
  (SELECT core_id FROM sys_model_data WHERE module IN
   ('account_period','account_multicurrency','account_bank','account_asset','account_dimension') AND model='sys.access');

-- 4. Fields then models.
DELETE FROM sys_field WHERE model_id IN (SELECT id FROM scope_models);
DELETE FROM sys_model WHERE id IN (SELECT id FROM scope_models);

-- 5. Xmlid registry for the in-scope modules.
DELETE FROM sys_model_data WHERE module IN
  ('account_period','account_multicurrency','account_bank','account_asset','account_dimension');

-- 6. Module install-state rows (so -i reinstalls fresh). Category left intact (shared "Accounting").
DELETE FROM sys_module WHERE name IN
  ('account_period','account_multicurrency','account_bank','account_asset','account_dimension');

COMMIT;
