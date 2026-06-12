CREATE INDEX IF NOT EXISTS idx_audit_user_action_time ON audit_log(user_email, action, timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_site_time ON audit_log(site, timestamp);
CREATE INDEX IF NOT EXISTS idx_ai_usage_user_time ON ai_usage(user_email, timestamp);
CREATE INDEX IF NOT EXISTS idx_uploaded_files_site_time ON uploaded_files(site, uploaded_at);
