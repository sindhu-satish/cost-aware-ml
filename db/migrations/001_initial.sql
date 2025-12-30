CREATE TABLE IF NOT EXISTS inference_requests (
    id SERIAL PRIMARY KEY,
    request_id VARCHAR(255) UNIQUE NOT NULL,
    tier VARCHAR(50) NOT NULL,
    budget FLOAT,
    confidence FLOAT,
    latency_ms INTEGER,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_request_id ON inference_requests(request_id);
CREATE INDEX idx_created_at ON inference_requests(created_at);

