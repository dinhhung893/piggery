-- v21 -> v22: the manifest's name key is `template:` (was `model:`, easily taken for the AI model);
-- the team's copy of it follows. Stored manifest snapshots are not rewritten: the parser still
-- reads a legacy `model:` in one.
ALTER TABLE teams RENAME COLUMN model_name TO template_name;
