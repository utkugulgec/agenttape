ALTER TABLE spans ADD COLUMN normalized_attrs JSONB;
CREATE INDEX spans_normalized_attrs_gin ON spans USING gin(normalized_attrs);
