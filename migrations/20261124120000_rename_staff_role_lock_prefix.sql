-- Switch the Staff Group role-exclusivity trigger advisory lock prefix to
-- 'xenobot:staff_role:'.
--
-- The earlier migration that first created staff_role_exclusivity_guard()
-- remains untouched because schema_migrations.checksum verifies every applied
-- migration file's SHA-256 on startup. This forward migration replaces the
-- function in place with identical role-exclusivity logic and lock ordering.
--
-- Every statement is idempotent; the Go runner applies each file once inside a
-- transaction and records its sha256 in schema_migrations.

CREATE OR REPLACE FUNCTION staff_role_exclusivity_guard()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_TABLE_NAME = 'staff_groups' THEN
        PERFORM pg_advisory_xact_lock(hashtextextended('xenobot:staff_role:' || NEW.chat_id::text, 0));

        IF EXISTS (SELECT 1 FROM staff_group_links WHERE group_chat_id = NEW.chat_id) THEN
            RAISE EXCEPTION 'staff role conflict: chat % is a linked group', NEW.chat_id
                USING ERRCODE = 'check_violation';
        END IF;
    ELSE
        PERFORM pg_advisory_xact_lock(hashtextextended(
            'xenobot:staff_role:' || LEAST(NEW.group_chat_id, NEW.staff_chat_id)::text, 0));
        PERFORM pg_advisory_xact_lock(hashtextextended(
            'xenobot:staff_role:' || GREATEST(NEW.group_chat_id, NEW.staff_chat_id)::text, 0));

        IF EXISTS (SELECT 1 FROM staff_groups WHERE chat_id = NEW.group_chat_id) THEN
            RAISE EXCEPTION 'staff role conflict: chat % is a Staff Group', NEW.group_chat_id
                USING ERRCODE = 'check_violation';
        END IF;

        IF EXISTS (
            SELECT 1 FROM staff_group_links
            WHERE group_chat_id = NEW.staff_chat_id AND id <> NEW.id
        ) THEN
            RAISE EXCEPTION 'staff role conflict: Staff Group % is itself linked', NEW.staff_chat_id
                USING ERRCODE = 'check_violation';
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
