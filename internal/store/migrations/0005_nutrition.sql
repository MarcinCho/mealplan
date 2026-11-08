CREATE TABLE product (
    id TEXT PRIMARY KEY,
    household_id TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    normalized_name TEXT NOT NULL,
    kcal DOUBLE PRECISION,
    protein DOUBLE PRECISION,
    carbs DOUBLE PRECISION,
    fat DOUBLE PRECISION,
    piece_grams DOUBLE PRECISION,
    density DOUBLE PRECISION,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    UNIQUE (household_id, normalized_name)
);

CREATE TABLE product_alias (
    household_id TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    product_id TEXT NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    alias TEXT NOT NULL,
    PRIMARY KEY (household_id, alias)
);

CREATE INDEX idx_product_alias_product ON product_alias (product_id);

ALTER TABLE household ADD COLUMN products_seeded INTEGER NOT NULL DEFAULT 0;

ALTER TABLE recipe ADD COLUMN kcal DOUBLE PRECISION;

ALTER TABLE recipe ADD COLUMN protein DOUBLE PRECISION;

ALTER TABLE recipe ADD COLUMN carbs DOUBLE PRECISION;

ALTER TABLE recipe ADD COLUMN fat DOUBLE PRECISION;

ALTER TABLE ingredient ADD COLUMN product_id TEXT REFERENCES product(id) ON DELETE SET NULL;

CREATE TABLE nutrition_settings (
    user_id TEXT NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    household_id TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    enabled INTEGER NOT NULL DEFAULT 0,
    goal_kcal DOUBLE PRECISION,
    goal_protein DOUBLE PRECISION,
    goal_carbs DOUBLE PRECISION,
    goal_fat DOUBLE PRECISION,
    PRIMARY KEY (user_id, household_id)
);

CREATE TABLE food_log (
    id TEXT PRIMARY KEY,
    household_id TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    date TEXT NOT NULL,
    product_id TEXT NOT NULL REFERENCES product(id) ON DELETE RESTRICT,
    quantity DOUBLE PRECISION NOT NULL CHECK (quantity > 0),
    unit TEXT NOT NULL CHECK (unit IN ('g', 'szt')),
    created_at BIGINT NOT NULL
);

CREATE INDEX idx_food_log_user_date ON food_log (household_id, user_id, date)
;
