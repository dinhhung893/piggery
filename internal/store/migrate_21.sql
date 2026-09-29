-- v20 -> v21: mail threads, expects_reply and the limits max_hops and messages_per_thread are gone
-- (nothing in the engine branched on them; the reply_to column and its in-view check stay).
-- A message held by one of the two removed limits would stay held for good (no rule left to hold
-- it, and an admin has nothing to explain), so it is released here, with its routing cc copies
-- (which share the original's hold). The rate limit's holds are left as they are. The 'held'
-- events stay in the log.
UPDATE messages SET held_reason = NULL
WHERE held_reason IS NOT NULL
  AND (id IN (SELECT ref_id FROM events WHERE type = 'held'
                AND json_extract(payload, '$.rule_id') IN ('limits.max_hops', 'limits.messages_per_thread'))
       OR cc_of IN (SELECT ref_id FROM events WHERE type = 'held'
                AND json_extract(payload, '$.rule_id') IN ('limits.max_hops', 'limits.messages_per_thread')));
DROP INDEX messages_thread;
ALTER TABLE messages DROP COLUMN thread_id;
ALTER TABLE messages DROP COLUMN expects_reply;
