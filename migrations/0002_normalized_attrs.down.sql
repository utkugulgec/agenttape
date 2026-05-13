DROP INDEX IF EXISTS spans_normalized_attrs_gin;
ALTER TABLE spans DROP COLUMN IF EXISTS normalized_attrs;
