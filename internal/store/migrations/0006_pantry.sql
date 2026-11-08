CREATE TABLE pantry_item (
    id TEXT PRIMARY KEY,
    household_id TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    product_id TEXT NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    quantity DOUBLE PRECISION,
    unit TEXT NOT NULL,
    expires_on TEXT NOT NULL DEFAULT '',
    staple INTEGER NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL,
    UNIQUE (household_id, product_id, unit)
);

CREATE TABLE pantry_movement (
    household_id TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    entry_id TEXT NOT NULL REFERENCES plan_entry(id) ON DELETE CASCADE,
    product_id TEXT NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    unit TEXT NOT NULL,
    amount DOUBLE PRECISION NOT NULL
);

CREATE INDEX idx_pantry_movement_entry ON pantry_movement (household_id, entry_id);

ALTER TABLE household ADD COLUMN pantry_reviewed_at BIGINT;

ALTER TABLE shopping_item ADD COLUMN have_quantity DOUBLE PRECISION;

ALTER TABLE shopping_item ADD COLUMN moved INTEGER NOT NULL DEFAULT 0
