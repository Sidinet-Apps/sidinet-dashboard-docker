ALTER TABLE widgets ADD COLUMN parent_widget_id INTEGER REFERENCES widgets(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_widgets_parent ON widgets(parent_widget_id);
