ALTER TABLE agent_llm_models ADD COLUMN IF NOT EXISTS icon TEXT NOT NULL DEFAULT '' CHECK (icon IN ('', 'lightning', 'brain'));
