ALTER TABLE agent_turns
    ADD COLUMN IF NOT EXISTS delivery_status TEXT NOT NULL DEFAULT 'not_required',
    ADD COLUMN IF NOT EXISTS delivery_error TEXT NOT NULL DEFAULT '';
