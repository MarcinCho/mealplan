INSERT INTO product_alias (household_id, product_id, alias)
SELECT p.household_id, p.id, 'ziemniaki'
FROM product p
WHERE p.normalized_name = 'ziemniaki'
  AND NOT EXISTS (SELECT 1 FROM product q WHERE q.household_id = p.household_id AND q.normalized_name = 'ziemniak')
  AND NOT EXISTS (SELECT 1 FROM product_alias a WHERE a.household_id = p.household_id AND a.alias = 'ziemniak' AND a.product_id <> p.id)
  AND NOT EXISTS (SELECT 1 FROM product_alias a WHERE a.household_id = p.household_id AND a.alias = 'ziemniaki');

DELETE FROM product_alias
WHERE alias = 'ziemniak'
  AND product_id IN (
    SELECT p.id FROM product p
    WHERE p.normalized_name = 'ziemniaki'
      AND NOT EXISTS (SELECT 1 FROM product q WHERE q.household_id = p.household_id AND q.normalized_name = 'ziemniak')
  );

UPDATE product SET name = 'ziemniak', normalized_name = 'ziemniak'
WHERE normalized_name = 'ziemniaki'
  AND NOT EXISTS (SELECT 1 FROM product q WHERE q.household_id = product.household_id AND q.normalized_name = 'ziemniak')
  AND NOT EXISTS (SELECT 1 FROM product_alias a WHERE a.household_id = product.household_id AND a.alias = 'ziemniak');
