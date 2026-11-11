-- 001_auth: users, sessions, activation codes, rate limiting
-- All statements are idempotent (IF NOT EXISTS).

CREATE TABLE IF NOT EXISTS psbc.users_by_id (
  user_id uuid PRIMARY KEY,
  username text,
  password_hash text,
  user_role text,                 -- SUPER_ADMIN | OSCA_ADMIN | OSCA_STAFF | SENIOR_CITIZEN | AUTHORIZED_REPRESENTATIVE
  full_name text,
  email text,
  is_active boolean,
  beneficiary_id uuid,            -- set for SENIOR_CITIZEN accounts
  failed_login_count int,
  locked_until timestamp,
  must_change_password boolean,
  created_at timestamp,
  created_by uuid,
  updated_at timestamp,
  last_login_at timestamp
);

-- Uniqueness of username is enforced with INSERT ... IF NOT EXISTS (LWT) on this table.
CREATE TABLE IF NOT EXISTS psbc.users_by_username (
  username text PRIMARY KEY,
  user_id uuid,
  created_at timestamp
);

-- Staff listing only (admin screens). Citizen/representative accounts are NOT stored here.
CREATE TABLE IF NOT EXISTS psbc.staff_by_role (
  user_role text,
  user_id uuid,
  username text,
  full_name text,
  is_active boolean,
  PRIMARY KEY ((user_role), user_id)
);

-- Only a hash of the refresh token is stored. Rows are written with USING TTL.
CREATE TABLE IF NOT EXISTS psbc.refresh_tokens_by_hash (
  token_hash text PRIMARY KEY,
  user_id uuid,
  family_id uuid,                 -- all rotations of one login share a family (reuse detection)
  issued_at timestamp,
  expires_at timestamp,
  revoked boolean,
  replaced_by_hash text
);

-- Supports "revoke all sessions for this user".
CREATE TABLE IF NOT EXISTS psbc.refresh_tokens_by_user (
  user_id uuid,
  token_hash text,
  PRIMARY KEY ((user_id), token_hash)
);

-- Staff-issued activation code for senior/representative accounts. Only the hash is stored.
CREATE TABLE IF NOT EXISTS psbc.activation_codes_by_hash (
  code_hash text PRIMARY KEY,
  beneficiary_id uuid,
  issued_by uuid,
  issued_at timestamp,
  expires_at timestamp,
  used boolean
);

-- Sliding-window rate limiting (login, public lookup). Rows expire after 1 hour.
-- bucket_key = hash of (client IP | OSCA ID), never raw personal data.
CREATE TABLE IF NOT EXISTS psbc.rate_limit_events (
  bucket_key text,
  event_id timeuuid,
  PRIMARY KEY ((bucket_key), event_id)
) WITH CLUSTERING ORDER BY (event_id DESC)
  AND default_time_to_live = 3600;
