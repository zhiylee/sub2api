ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS actual_cost_before_discount NUMERIC(20,10);

COMMENT ON COLUMN usage_logs.actual_cost_before_discount IS
    'Customer charge before a Free Fast or Free Ultrafast waiver; NULL if no waiver amount was recorded';
