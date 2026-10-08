-- Seed authorization in the existing extension tables. No users schema change.
-- The marker distinguishes this system definition from a pre-existing custom key.
DO $$
BEGIN
 IF EXISTS (SELECT 1 FROM user_attribute_definitions WHERE key = 'insights_access'
   AND (deleted_at IS NOT NULL OR description IS DISTINCT FROM '系统授权字段：允许查看全部 AI基础设施看板数据，不授予编辑或系统管理权限。'
     OR type <> 'select' OR NOT enabled OR required
     OR options IS DISTINCT FROM '[{"value":"disabled","label":"关闭"},{"value":"enabled","label":"开启"}]'::jsonb)) THEN
  RAISE EXCEPTION 'insights_access conflicts with an existing attribute; review it before enabling Insights access';
 END IF;
 INSERT INTO user_attribute_definitions (key, name, description, type, options, required, enabled)
 VALUES ('insights_access', '允许查看 AI基础设施看板',
   '系统授权字段：允许查看全部 AI基础设施看板数据，不授予编辑或系统管理权限。', 'select',
   '[{"value":"disabled","label":"关闭"},{"value":"enabled","label":"开启"}]'::jsonb, false, true)
 ON CONFLICT (key) WHERE deleted_at IS NULL DO NOTHING;
END $$;
