-- Surgical reset of the school* namespace (removes the dead FluentHub "school"
-- app orphans + my polluted anchor models) so a fresh install is clean.
-- Runs in one transaction; aborts+rolls back on any error. Tables are empty.
BEGIN;

-- 0. Capture ids of in-scope models/groups before we start deleting.
CREATE TEMP TABLE scope_models ON COMMIT DROP AS
  SELECT id, model FROM sys_model
  WHERE module IN ('school','school_fees','school_payments','school_promotion','school_reporting');

CREATE TEMP TABLE scope_groupids ON COMMIT DROP AS
  SELECT core_id AS id FROM sys_model_data
  WHERE module = 'school' AND model = 'core.group';

-- 1. Drop physical tables for every in-scope model (dots -> underscores), CASCADE
--    to clear FKs among the school tables themselves.
DO $$
DECLARE r RECORD;
BEGIN
  FOR r IN SELECT model FROM scope_models LOOP
    EXECUTE 'DROP TABLE IF EXISTS "' || replace(r.model, '.', '_') || '" CASCADE';
  END LOOP;
END $$;

-- 1b. Drop any leftover school_* physical tables that were not registered.
DO $$
DECLARE r RECORD;
BEGIN
  FOR r IN SELECT tablename FROM pg_tables
           WHERE schemaname = 'public' AND tablename LIKE 'school\_%' LOOP
    EXECUTE 'DROP TABLE IF EXISTS "' || r.tablename || '" CASCADE';
  END LOOP;
END $$;

-- 2. Groups: relations first, then the group rows.
DELETE FROM core_group_user_rel WHERE group_id IN (SELECT id FROM scope_groupids);
DELETE FROM core_group_implied  WHERE group_id IN (SELECT id FROM scope_groupids)
                                   OR implied_group_id IN (SELECT id FROM scope_groupids);
DELETE FROM sys_rule_group_rel  WHERE group_id IN (SELECT id FROM scope_groupids);
DELETE FROM core_group          WHERE id IN (SELECT id FROM scope_groupids);

-- 3. Menus (reference actions) before actions.
DELETE FROM sys_menu WHERE id IN
  (SELECT core_id FROM sys_model_data WHERE module IN
   ('school','school_fees','school_payments','school_promotion','school_reporting') AND model='sys.menu');
DELETE FROM sys_menu WHERE module IN
  ('school','school_fees','school_payments','school_promotion','school_reporting');

-- 4. Actions, views, access rules, record rules, sequences.
DELETE FROM sys_action_window WHERE id IN
  (SELECT core_id FROM sys_model_data WHERE module IN
   ('school','school_fees','school_payments','school_promotion','school_reporting') AND model IN ('sys.action.window','ir.actions.window'));
DELETE FROM sys_view WHERE id IN
  (SELECT core_id FROM sys_model_data WHERE module IN
   ('school','school_fees','school_payments','school_promotion','school_reporting') AND model='sys.view');
DELETE FROM sys_access WHERE id IN
  (SELECT core_id FROM sys_model_data WHERE module IN
   ('school','school_fees','school_payments','school_promotion','school_reporting') AND model='sys.access');

-- 5. Fields then models.
DELETE FROM sys_field WHERE model_id IN (SELECT id FROM scope_models);
DELETE FROM sys_model WHERE id IN (SELECT id FROM scope_models);

-- 6. Xmlid registry for the in-scope modules.
DELETE FROM sys_model_data WHERE module IN
  ('school','school_fees','school_payments','school_promotion','school_reporting');

-- 7. Module category + module install-state rows (so -i reinstalls fresh).
DELETE FROM sys_module_category WHERE id IN
  (SELECT category_id FROM sys_module WHERE name IN
   ('school','school_fees','school_payments','school_promotion','school_reporting') AND category_id IS NOT NULL);
DELETE FROM sys_module WHERE name IN
  ('school','school_fees','school_payments','school_promotion','school_reporting');

COMMIT;
