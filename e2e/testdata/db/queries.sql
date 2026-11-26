-- The queries TestDB lists. The fake sqlite3 in bin/ answers each one the
-- test runs with what sqlite3 3.45.1 printed for it (out/); the last is only
-- listed, to check that its name shows as text.

-- name: users
SELECT id, name, email FROM users LIMIT 100;

-- name: active-users
-- every active user, by name
SELECT name FROM users WHERE active = 1 ORDER BY name;

-- name: nobody
SELECT * FROM users WHERE name = 'Nobody';

-- name: close-stale
UPDATE orders SET status = 'closed' WHERE day < '2026-11-25';

-- name: slow
SELECT count(*) FROM orders AS a, orders AS b, orders AS c;

-- name: <img/src=x/onerror=alert(1)>
SELECT 'tagged';
