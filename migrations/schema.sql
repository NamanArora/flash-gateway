-- Flash Gateway Database Schema
-- Combined migrations for complete database initialization

-- Enable UUID generation
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create request_logs table
CREATE TABLE request_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    session_id VARCHAR(255),
    request_id UUID NOT NULL DEFAULT uuid_generate_v4(),
    endpoint VARCHAR(255) NOT NULL,
    method VARCHAR(10) NOT NULL,
    status_code INT,
    latency_ms BIGINT,
    provider VARCHAR(50),
    user_agent TEXT,
    remote_addr VARCHAR(45),
    request_headers JSONB,
    request_body TEXT,
    response_headers JSONB,
    response_body TEXT,
    error TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create indexes for performance
CREATE INDEX idx_request_logs_timestamp ON request_logs(timestamp DESC);
CREATE INDEX idx_request_logs_session_id ON request_logs(session_id) WHERE session_id IS NOT NULL;
CREATE UNIQUE INDEX idx_request_logs_request_id ON request_logs(request_id);
CREATE INDEX idx_request_logs_endpoint_status ON request_logs(endpoint, status_code);
CREATE INDEX idx_request_logs_provider ON request_logs(provider) WHERE provider IS NOT NULL;
CREATE INDEX idx_request_logs_method ON request_logs(method);

-- Create partial index for errors only
CREATE INDEX idx_request_logs_errors ON request_logs(timestamp DESC) WHERE error IS NOT NULL;

-- Create GIN indexes for JSONB columns
CREATE INDEX idx_request_logs_request_headers_gin ON request_logs USING GIN(request_headers);
CREATE INDEX idx_request_logs_response_headers_gin ON request_logs USING GIN(response_headers);
CREATE INDEX idx_request_logs_metadata_gin ON request_logs USING GIN(metadata);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger to automatically update updated_at
CREATE TRIGGER update_request_logs_updated_at
    BEFORE UPDATE ON request_logs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Create view for recent logs (commonly used query)
CREATE VIEW recent_request_logs AS
SELECT
    id,
    timestamp,
    session_id,
    request_id,
    endpoint,
    method,
    status_code,
    latency_ms,
    provider,
    user_agent,
    remote_addr,
    CASE
        WHEN length(request_body) > 1000 THEN left(request_body, 1000) || '...'
        ELSE request_body
    END as request_body_preview,
    CASE
        WHEN length(response_body) > 1000 THEN left(response_body, 1000) || '...'
        ELSE response_body
    END as response_body_preview,
    error,
    created_at
FROM request_logs
WHERE timestamp > NOW() - INTERVAL '24 hours'
ORDER BY timestamp DESC;

-- Create guardrail_metrics table for tracking performance and results
CREATE TABLE guardrail_metrics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    request_id UUID NOT NULL REFERENCES request_logs(request_id),
    guardrail_name VARCHAR(100) NOT NULL,
    layer VARCHAR(10) NOT NULL CHECK (layer IN ('input', 'output')),
    priority INT NOT NULL,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    duration_ms BIGINT NOT NULL,
    passed BOOLEAN NOT NULL,
    score FLOAT,
    error TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX idx_guardrail_metrics_request_id ON guardrail_metrics(request_id);
CREATE INDEX idx_guardrail_metrics_name ON guardrail_metrics(guardrail_name);
CREATE INDEX idx_guardrail_metrics_layer ON guardrail_metrics(layer);
CREATE INDEX idx_guardrail_metrics_priority ON guardrail_metrics(priority);
CREATE INDEX idx_guardrail_metrics_duration ON guardrail_metrics(duration_ms);
CREATE INDEX idx_guardrail_metrics_passed ON guardrail_metrics(passed);
CREATE INDEX idx_guardrail_metrics_created_at ON guardrail_metrics(created_at DESC);

-- Composite indexes for common queries
CREATE INDEX idx_guardrail_metrics_layer_passed ON guardrail_metrics(layer, passed);
CREATE INDEX idx_guardrail_metrics_name_layer ON guardrail_metrics(guardrail_name, layer);

-- GIN index for JSONB metadata queries
CREATE INDEX idx_guardrail_metrics_metadata_gin ON guardrail_metrics USING GIN(metadata);

-- Add guardrail status columns to request_logs table
ALTER TABLE request_logs ADD COLUMN guardrails_passed BOOLEAN DEFAULT TRUE;
ALTER TABLE request_logs ADD COLUMN guardrail_failure_reason TEXT;
ALTER TABLE request_logs ADD COLUMN failed_guardrail_name VARCHAR(100);

-- Create index on new guardrail columns for filtering
CREATE INDEX idx_request_logs_guardrails_passed ON request_logs(guardrails_passed);
CREATE INDEX idx_request_logs_failed_guardrail ON request_logs(failed_guardrail_name) WHERE failed_guardrail_name IS NOT NULL;

-- Create view for quick guardrail performance analysis
CREATE VIEW guardrail_performance_summary AS
SELECT
    guardrail_name,
    layer,
    priority,
    COUNT(*) as total_executions,
    COUNT(CASE WHEN passed THEN 1 END) as passed_count,
    COUNT(CASE WHEN NOT passed THEN 1 END) as failed_count,
    ROUND(
        (COUNT(CASE WHEN passed THEN 1 END)::FLOAT / COUNT(*) * 100)::NUMERIC, 2
    ) as pass_rate_percent,
    ROUND(AVG(duration_ms)::NUMERIC, 2) as avg_duration_ms,
    ROUND(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY duration_ms)::NUMERIC, 2) as median_duration_ms,
    ROUND(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms)::NUMERIC, 2) as p95_duration_ms,
    MIN(duration_ms) as min_duration_ms,
    MAX(duration_ms) as max_duration_ms,
    MIN(created_at) as first_execution,
    MAX(created_at) as last_execution
FROM guardrail_metrics
GROUP BY guardrail_name, layer, priority
ORDER BY layer, priority, guardrail_name;

-- Add columns for response override tracking
ALTER TABLE guardrail_metrics
ADD COLUMN original_response TEXT,      -- Original LLM response (before override)
ADD COLUMN override_response TEXT,      -- Override response sent to client
ADD COLUMN response_overridden BOOLEAN DEFAULT FALSE;

-- Create index for response override queries
CREATE INDEX idx_guardrail_metrics_response_overridden ON guardrail_metrics(response_overridden) WHERE response_overridden = TRUE;

-- Create view for recent guardrail failures
CREATE VIEW recent_guardrail_failures AS
SELECT
    gm.created_at,
    gm.request_id,
    gm.guardrail_name,
    gm.layer,
    gm.priority,
    gm.error,
    gm.duration_ms,
    gm.response_overridden,
    rl.endpoint,
    rl.method,
    rl.status_code as request_status_code
FROM guardrail_metrics gm
JOIN request_logs rl ON gm.request_id = rl.request_id
WHERE gm.passed = FALSE
  AND gm.created_at > NOW() - INTERVAL '24 hours'
ORDER BY gm.created_at DESC;

-- Create view for response overrides analysis
CREATE VIEW guardrail_response_overrides AS
SELECT
    gm.created_at,
    gm.request_id,
    gm.guardrail_name,
    gm.layer,
    gm.original_response,
    gm.override_response,
    rl.endpoint,
    rl.method,
    rl.user_agent
FROM guardrail_metrics gm
JOIN request_logs rl ON gm.request_id = rl.request_id
WHERE gm.response_overridden = TRUE
ORDER BY gm.created_at DESC;