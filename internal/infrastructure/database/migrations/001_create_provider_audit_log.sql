-- Create provider audit log table for traceability
CREATE TABLE IF NOT EXISTS provider_audit_log (
    id BIGSERIAL PRIMARY KEY,
    correlation_id VARCHAR(255) NOT NULL,
    provider VARCHAR(100) NOT NULL,
    operation VARCHAR(100) NOT NULL,
    request_method VARCHAR(10) NOT NULL,
    request_url TEXT NOT NULL,
    request_headers JSONB,
    request_body JSONB,
    response_status INTEGER,
    response_headers JSONB,
    response_body JSONB,
    duration_ms BIGINT NOT NULL,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_correlation_id ON provider_audit_log(correlation_id);
CREATE INDEX IF NOT EXISTS idx_provider_operation ON provider_audit_log(provider, operation);
CREATE INDEX IF NOT EXISTS idx_created_at ON provider_audit_log(created_at);
CREATE INDEX IF NOT EXISTS idx_response_status ON provider_audit_log(response_status);

-- Add comment for documentation
COMMENT ON TABLE provider_audit_log IS 'Audit trail for all external provider API requests and responses';
