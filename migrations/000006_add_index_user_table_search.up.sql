CREATE INDEX IF NOT EXISTS user_email_idx ON users USING btree(email);
CREATE INDEX IF NOT EXISTS user_name_idx ON users USING btree(name);