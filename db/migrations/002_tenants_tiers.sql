CREATE TABLE IF NOT EXISTS tenants (
    id SERIAL PRIMARY KEY,
    tenant_id VARCHAR(255) UNIQUE NOT NULL,
    plan VARCHAR(50) NOT NULL,
    monthly_budget_cents FLOAT,
    per_request_budget_cents_default FLOAT,
    slo_p99_ms INTEGER,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tiers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL,
    base_cost_cents FLOAT NOT NULL,
    timeout_ms INTEGER NOT NULL,
    max_concurrency INTEGER,
    default_conf_threshold FLOAT,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS policies (
    id SERIAL PRIMARY KEY,
    tenant_id VARCHAR(255) NOT NULL,
    policy_json JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id)
);

INSERT INTO tiers (name, base_cost_cents, timeout_ms, max_concurrency, default_conf_threshold) VALUES
('tier0', 0.5, 50, 100, 0.75),
('tier1', 2.0, 200, 50, 0.85),
('tier2', 5.0, 500, 20, 0.95)
ON CONFLICT (name) DO NOTHING;

INSERT INTO tenants (tenant_id, plan, monthly_budget_cents, per_request_budget_cents_default, slo_p99_ms) VALUES
('tenant-1', 'free', 1000.0, 5.0, 500),
('tenant-2', 'premium', 10000.0, 15.0, 300)
ON CONFLICT (tenant_id) DO NOTHING;

