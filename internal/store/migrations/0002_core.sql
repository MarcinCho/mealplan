CREATE TABLE recipe (
    id TEXT PRIMARY KEY,
    household_id TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    base_servings INTEGER NOT NULL CHECK (base_servings > 0),
    source_url TEXT NOT NULL DEFAULT '',
    image_path TEXT NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

CREATE INDEX idx_recipe_household ON recipe (household_id);

CREATE TABLE ingredient (
    recipe_id TEXT NOT NULL REFERENCES recipe(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    name TEXT NOT NULL,
    normalized_name TEXT NOT NULL,
    quantity DOUBLE PRECISION,
    unit TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (recipe_id, position)
);

CREATE TABLE step (
    recipe_id TEXT NOT NULL REFERENCES recipe(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    text TEXT NOT NULL,
    PRIMARY KEY (recipe_id, position)
);

CREATE TABLE recipe_tag (
    recipe_id TEXT NOT NULL REFERENCES recipe(id) ON DELETE CASCADE,
    tag TEXT NOT NULL,
    PRIMARY KEY (recipe_id, tag)
);

CREATE TABLE plan_entry (
    id TEXT PRIMARY KEY,
    household_id TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    date TEXT NOT NULL,
    slot TEXT NOT NULL CHECK (slot IN ('breakfast', 'lunch', 'dinner', 'snack')),
    recipe_id TEXT REFERENCES recipe(id) ON DELETE RESTRICT,
    free_text TEXT NOT NULL DEFAULT '',
    servings INTEGER NOT NULL CHECK (servings > 0),
    cook_user_id TEXT REFERENCES app_user(id) ON DELETE SET NULL,
    CHECK ((recipe_id IS NOT NULL) <> (free_text <> ''))
);

CREATE INDEX idx_plan_entry_household_date ON plan_entry (household_id, date);

CREATE TABLE shopping_item (
    id TEXT PRIMARY KEY,
    household_id TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    week TEXT NOT NULL,
    name TEXT NOT NULL,
    normalized_name TEXT NOT NULL,
    quantity DOUBLE PRECISION,
    unit TEXT NOT NULL DEFAULT '',
    checked INTEGER NOT NULL DEFAULT 0,
    manual INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_shopping_item_household_week ON shopping_item (household_id, week)
