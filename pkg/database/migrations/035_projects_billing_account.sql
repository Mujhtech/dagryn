-- Link projects to billing accounts for quota enforcement.
ALTER TABLE projects ADD COLUMN billing_account_id UUID REFERENCES billing_accounts(id);

CREATE INDEX idx_projects_billing_account ON projects(billing_account_id)
    WHERE billing_account_id IS NOT NULL;
