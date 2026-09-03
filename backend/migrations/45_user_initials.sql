-- AutoMigrate adds the initials column (default '') before this file runs, so
-- every existing user starts here with nothing set. Backfill it with the same
-- rule internal/services/user_initials.go applies to a fresh user: one letter
-- per word for a multi-word name, otherwise the first two letters of the one
-- word a username almost always is. This SQL cannot call that Go function, so
-- the rule is restated here — accepted once, since this file runs exactly one
-- time and then becomes history rather than a second copy that can drift.
WITH split AS (
    SELECT
        id,
        array_remove(regexp_split_to_array(username, '[[:space:]._-]+'), '') AS parts
    FROM users
    WHERE initials = ''
),
derived AS (
    SELECT
        id,
        CASE
            WHEN array_length(parts, 1) > 1 THEN (
                SELECT string_agg(left(p, 1), '' ORDER BY ord)
                FROM unnest(parts) WITH ORDINALITY AS u(p, ord)
            )
            WHEN array_length(parts, 1) = 1 THEN left(parts[1], 2)
            ELSE ''
        END AS mark
    FROM split
)
UPDATE users
SET initials = upper(derived.mark)
FROM derived
WHERE users.id = derived.id
  AND derived.mark != '';
