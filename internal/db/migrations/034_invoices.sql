-- Local cache of Stripe invoices for dashboard display.
CREATE TABLE IF NOT EXISTS invoices (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    billing_account_id  UUID NOT NULL REFERENCES billing_accounts(id) ON DELETE CASCADE,
    stripe_invoice_id   TEXT UNIQUE NOT NULL,
    amount_cents        INT NOT NULL,
    currency            TEXT NOT NULL DEFAULT 'usd',
    status              TEXT NOT NULL,
    period_start        TIMESTAMPTZ,
    period_end          TIMESTAMPTZ,
    pdf_url             TEXT,
    hosted_invoice_url  TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_invoices_account ON invoices(billing_account_id, created_at DESC);
