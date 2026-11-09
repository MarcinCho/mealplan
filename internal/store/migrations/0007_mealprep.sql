ALTER TABLE household ADD COLUMN portions_per_meal INTEGER NOT NULL DEFAULT 2;

CREATE TABLE prep_session (
    id TEXT PRIMARY KEY,
    household_id TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    date TEXT NOT NULL,
    name TEXT NOT NULL,
    week TEXT NOT NULL,
    cooked INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL
);

CREATE INDEX idx_prep_session_household_date ON prep_session (household_id, date);

CREATE TABLE prep_dish (
    id TEXT PRIMARY KEY,
    household_id TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    session_id TEXT NOT NULL REFERENCES prep_session(id) ON DELETE CASCADE,
    recipe_id TEXT NOT NULL REFERENCES recipe(id) ON DELETE RESTRICT,
    portions INTEGER NOT NULL CHECK (portions > 0),
    written_off INTEGER NOT NULL DEFAULT 0 CHECK (written_off >= 0),
    position INTEGER NOT NULL
);

CREATE INDEX idx_prep_dish_session ON prep_dish (session_id);

CREATE TABLE freezer_item (
    id TEXT PRIMARY KEY,
    household_id TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    dish_id TEXT REFERENCES prep_dish(id) ON DELETE SET NULL,
    recipe_id TEXT REFERENCES recipe(id) ON DELETE SET NULL,
    label TEXT NOT NULL,
    portions INTEGER NOT NULL CHECK (portions > 0),
    frozen_on TEXT NOT NULL
);

CREATE INDEX idx_freezer_item_household ON freezer_item (household_id);

CREATE TABLE pantry_movement_bak AS SELECT * FROM pantry_movement;

DROP TABLE pantry_movement;

CREATE TABLE plan_entry_new (
    id TEXT PRIMARY KEY,
    household_id TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    date TEXT NOT NULL,
    slot TEXT NOT NULL CHECK (slot IN ('breakfast', 'lunch', 'dinner', 'snack')),
    recipe_id TEXT REFERENCES recipe(id) ON DELETE RESTRICT,
    free_text TEXT NOT NULL DEFAULT '',
    servings INTEGER NOT NULL CHECK (servings > 0),
    cook_user_id TEXT REFERENCES app_user(id) ON DELETE SET NULL,
    done INTEGER NOT NULL DEFAULT 0,
    kind TEXT NOT NULL DEFAULT 'meal' CHECK (kind IN ('meal', 'prep', 'out')),
    prep_dish_id TEXT REFERENCES prep_dish(id) ON DELETE CASCADE,
    freezer_item_id TEXT REFERENCES freezer_item(id) ON DELETE CASCADE,
    out_kcal DOUBLE PRECISION,
    out_protein DOUBLE PRECISION,
    out_carbs DOUBLE PRECISION,
    out_fat DOUBLE PRECISION,
    CHECK (
        (kind = 'meal' AND ((recipe_id IS NOT NULL) <> (free_text <> '')) AND prep_dish_id IS NULL AND freezer_item_id IS NULL)
        OR (kind = 'prep' AND ((prep_dish_id IS NOT NULL) <> (freezer_item_id IS NOT NULL)) AND (recipe_id IS NOT NULL OR free_text <> ''))
        OR (kind = 'out' AND recipe_id IS NULL AND prep_dish_id IS NULL AND freezer_item_id IS NULL)
    )
);

INSERT INTO plan_entry_new (id, household_id, date, slot, recipe_id, free_text, servings, cook_user_id, done)
SELECT id, household_id, date, slot, recipe_id, free_text, servings, cook_user_id, done FROM plan_entry;

DROP TABLE plan_entry;

ALTER TABLE plan_entry_new RENAME TO plan_entry;

CREATE INDEX idx_plan_entry_household_date ON plan_entry (household_id, date);

CREATE TABLE pantry_movement (
    household_id TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    entry_id TEXT REFERENCES plan_entry(id) ON DELETE CASCADE,
    session_id TEXT REFERENCES prep_session(id) ON DELETE CASCADE,
    product_id TEXT NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    unit TEXT NOT NULL,
    amount DOUBLE PRECISION NOT NULL,
    CHECK ((entry_id IS NULL) <> (session_id IS NULL))
);

CREATE INDEX idx_pantry_movement_entry ON pantry_movement (household_id, entry_id);

CREATE INDEX idx_pantry_movement_session ON pantry_movement (household_id, session_id);

INSERT INTO pantry_movement (household_id, entry_id, product_id, unit, amount)
SELECT household_id, entry_id, product_id, unit, amount FROM pantry_movement_bak;

DROP TABLE pantry_movement_bak
