-- Add group and condition fields to workflow tasks
ALTER TABLE workflow_tasks ADD COLUMN group_name VARCHAR(255);
ALTER TABLE workflow_tasks ADD COLUMN condition_expr TEXT;
