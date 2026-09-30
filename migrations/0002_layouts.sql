INSERT OR IGNORE INTO widget_layouts(widget_id,breakpoint,x,y,width,height)
SELECT id,'desktop',0,(id-1)*2,3,2 FROM widgets;
INSERT OR IGNORE INTO widget_layouts(widget_id,breakpoint,x,y,width,height)
SELECT id,'tablet',0,(id-1)*2,4,2 FROM widgets;
INSERT OR IGNORE INTO widget_layouts(widget_id,breakpoint,x,y,width,height)
SELECT id,'mobile',0,(id-1)*2,4,2 FROM widgets;
INSERT OR IGNORE INTO schema_migrations(version,name,applied_at) VALUES(2,'responsive layouts',CURRENT_TIMESTAMP);
