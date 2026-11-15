ALTER TABLE agent_turns
    DROP COLUMN IF EXISTS delivery_status,
    DROP COLUMN IF EXISTS delivery_error;
