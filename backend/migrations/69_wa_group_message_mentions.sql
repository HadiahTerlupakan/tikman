-- Members tagged by an outgoing group message, as a JSON list of JIDs.
-- AutoMigrate adds the column from the model tag first; this keeps the schema
-- stated in the versioned files as well.
ALTER TABLE wa_group_messages ADD COLUMN IF NOT EXISTS mentions text NOT NULL DEFAULT '';
