-- The link card a message carries.
--
-- AutoMigrate runs BEFORE this file (cmd/api/main.go:56 against :68) and adds
-- these columns from the model tags, so every statement here is written to be
-- a no-op in that case rather than a collision that stops the API starting.
--
-- Outgoing cards are what the wa process already fetched in order to attach
-- them to the message; incoming ones arrive inside WhatsApp's own protobuf.
-- Storing them costs no extra request and keeps the card readable long after
-- the page behind it has gone.
ALTER TABLE cs_messages ADD COLUMN IF NOT EXISTS preview_url text;
ALTER TABLE cs_messages ADD COLUMN IF NOT EXISTS preview_title text;
ALTER TABLE cs_messages ADD COLUMN IF NOT EXISTS preview_description text;
ALTER TABLE cs_messages ADD COLUMN IF NOT EXISTS preview_thumbnail bytea;
