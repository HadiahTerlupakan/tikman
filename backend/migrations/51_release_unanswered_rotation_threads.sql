-- Threads the round-robin handed out go back to the shared queue when their
-- holder never answered them.
--
-- The rotation gave every waiting thread to whoever had the inbox open, so an
-- agent who opened the page at the start of a shift was shown as holding
-- conversations they never looked at, and everyone else was told to take them
-- over before replying. The rotation is gone: a CS now holds a thread by
-- answering it first, or by taking it over (CSConversationService.ClaimForReply
-- and CSHandler.Assign).
--
-- A thread stays with its holder when that holder has sent a reply in it, or
-- when a person put them there. Every manual assignment and takeover is audited
-- with the new holder; the rotation never wrote to the audit log, so a holder
-- with no reply and no audited handover can only have come from the rotation.
--
-- Only open threads are touched. A closed thread keeps its holder as the record
-- of who dealt with it, and a customer writing again already reopens it
-- unassigned.
UPDATE cs_conversations c
SET status = 'unassigned', assigned_user_id = NULL
WHERE c.status = 'open'
  AND NOT EXISTS (
    SELECT 1 FROM cs_messages m
    WHERE m.conversation_id = c.id
      AND m.direction = 'out'
      AND m.sender_user_id = c.assigned_user_id
  )
  AND NOT EXISTS (
    SELECT 1 FROM audit_logs a
    WHERE a.resource_type = 'cs_conversation'
      AND a.resource_id = c.id
      AND a.action = 'assign'
      AND a.new_value ->> 'assigned_user_id' = c.assigned_user_id::text
  );
