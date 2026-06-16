-- Bug-2 fix: set a 72-hour default TTL on yjs_updates so rows auto-expire.
--
-- yjs_state already holds the authoritative full-document snapshot that is
-- read on room hydration. yjs_updates is an audit/replay log that currently
-- grows without bound because nothing ever reads or trims it.
--
-- ALTER TABLE … WITH default_time_to_live applies to all rows written from
-- this point forward. Existing rows are unaffected by the ALTER but will
-- expire naturally once the TTL on their individual tombstones kicks in
-- (new writes use the table-level TTL automatically).
--
-- 72 hours = 259200 seconds.

ALTER TABLE notes_ks.yjs_updates
    WITH default_time_to_live = 259200;
