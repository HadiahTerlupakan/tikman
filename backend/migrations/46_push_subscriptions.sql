-- AutoMigrate creates the table and its columns; this adds the constraint
-- that only real Postgres enforces (see migration 41 for the same split on
-- cs_conversations). A user deleted with their subscriptions still owned is
-- a device that should simply stop hearing from an account that is gone —
-- CASCADE, not RESTRICT.
ALTER TABLE push_subscriptions
    ADD CONSTRAINT fk_push_subscriptions_user
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
