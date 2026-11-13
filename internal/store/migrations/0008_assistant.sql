ALTER TABLE household ADD COLUMN openrouter_key TEXT NOT NULL DEFAULT '';

ALTER TABLE household ADD COLUMN openrouter_model TEXT NOT NULL DEFAULT '';

DELETE FROM product_alias
WHERE alias IN ('cebula czerwona', 'cebuli czerwonej')
  AND product_id IN (SELECT id FROM product WHERE normalized_name = 'cebula');

INSERT INTO product (id, household_id, name, normalized_name, kcal, protein, carbs, fat, piece_grams, density, created_at, updated_at)
SELECT 'seed-redonion-' || h.id, h.id, 'cebula czerwona', 'cebula czerwona', 40, 1.1, 9.34, 0.1, 110, NULL, h.created_at, h.created_at
FROM household h
WHERE h.products_seeded = 1
  AND NOT EXISTS (SELECT 1 FROM product p WHERE p.household_id = h.id AND p.normalized_name = 'cebula czerwona')
  AND NOT EXISTS (SELECT 1 FROM product_alias a WHERE a.household_id = h.id AND a.alias = 'cebula czerwona');

INSERT INTO product_alias (household_id, product_id, alias)
SELECT p.household_id, p.id, n.alias
FROM product p
CROSS JOIN (
    SELECT 'czerwona cebula' AS alias
    UNION ALL SELECT 'czerwonej cebuli'
    UNION ALL SELECT 'cebuli czerwonej'
    UNION ALL SELECT 'czerwoną cebulę'
    UNION ALL SELECT 'cebulka czerwona'
    UNION ALL SELECT 'czerwona cebulka'
    UNION ALL SELECT 'czerwonej cebulki'
) n
WHERE p.id = 'seed-redonion-' || p.household_id
  AND NOT EXISTS (SELECT 1 FROM product_alias a WHERE a.household_id = p.household_id AND a.alias = n.alias)
  AND NOT EXISTS (SELECT 1 FROM product q WHERE q.household_id = p.household_id AND q.normalized_name = n.alias);
