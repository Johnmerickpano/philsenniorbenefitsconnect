-- 002_beneficiaries: records, lookup/search tables, status history, representatives
-- record_status values: PENDING_VERIFICATION | VERIFIED | ACTIVE | INACTIVE | FOR_REVIEW | ARCHIVED
-- These are application workflow statuses, NOT official eligibility decisions.
-- No contact number is collected (data minimization); add only if OSCA requires it.

CREATE TABLE IF NOT EXISTS psbc.beneficiaries_by_id (
  beneficiary_id uuid PRIMARY KEY,
  osca_id text,
  first_name text,
  middle_name text,
  last_name text,
  suffix text,
  sort_name text,                 -- normalized "last first middle" (lowercase, no accents)
  birth_date date,                -- age is computed, never stored
  sex text,
  barangay_code text,
  barangay_name text,
  record_status text,
  last_verified_at timestamp,
  last_verified_by uuid,
  row_version bigint,             -- optimistic concurrency (compare-and-set with IF row_version = ?)
  created_at timestamp,
  created_by uuid,
  updated_at timestamp,
  updated_by uuid,
  archived_at timestamp,
  archived_by uuid,
  archive_reason text
);

-- OSCA ID uniqueness via INSERT ... IF NOT EXISTS. Also the ID-lookup path.
CREATE TABLE IF NOT EXISTS psbc.beneficiaries_by_osca_id (
  osca_id text PRIMARY KEY,
  beneficiary_id uuid,
  created_at timestamp
);

-- Name search. One row per name part, bucketed by the first 3 letters of that part.
-- Query: WHERE barangay_code = ? AND name_prefix = ?  -> then filter in the app, paginated and bounded.
CREATE TABLE IF NOT EXISTS psbc.beneficiary_search (
  barangay_code text,
  name_prefix text,
  sort_name text,
  beneficiary_id uuid,
  osca_id text,
  display_name text,
  record_status text,
  PRIMARY KEY ((barangay_code, name_prefix), sort_name, beneficiary_id)
) WITH CLUSTERING ORDER BY (sort_name ASC, beneficiary_id ASC);

-- Admin list view (default). review_flag = type of the most recent open flag, or null.
CREATE TABLE IF NOT EXISTS psbc.beneficiaries_by_barangay (
  barangay_code text,
  sort_name text,
  beneficiary_id uuid,
  osca_id text,
  display_name text,
  birth_date date,
  record_status text,
  review_flag text,
  updated_at timestamp,
  PRIMARY KEY ((barangay_code), sort_name, beneficiary_id)
) WITH CLUSTERING ORDER BY (sort_name ASC, beneficiary_id ASC);

-- Admin list filtered by status. A status change = delete from old partition + insert into new (batch).
CREATE TABLE IF NOT EXISTS psbc.beneficiaries_by_status (
  record_status text,
  barangay_code text,
  sort_name text,
  beneficiary_id uuid,
  osca_id text,
  display_name text,
  birth_date date,
  review_flag text,
  updated_at timestamp,
  PRIMARY KEY ((record_status, barangay_code), sort_name, beneficiary_id)
) WITH CLUSTERING ORDER BY (sort_name ASC, beneficiary_id ASC);

-- Append-only status timeline.
CREATE TABLE IF NOT EXISTS psbc.beneficiary_status_history (
  beneficiary_id uuid,
  changed_at timestamp,
  event_id timeuuid,
  old_status text,
  new_status text,
  changed_by uuid,
  remarks text,
  PRIMARY KEY ((beneficiary_id), changed_at, event_id)
) WITH CLUSTERING ORDER BY (changed_at DESC, event_id DESC);

-- Authorized representatives. Only ID type and last 4 characters of the ID number are kept.
CREATE TABLE IF NOT EXISTS psbc.representatives_by_beneficiary (
  beneficiary_id uuid,
  rep_id uuid,
  rep_name text,
  relationship text,
  id_type text,
  id_number_last4 text,
  auth_status text,               -- PENDING | VERIFIED | REVOKED | EXPIRED
  valid_from date,
  valid_until date,
  user_id uuid,                   -- linked login account, if any
  requested_at timestamp,
  verified_by uuid,
  verified_at timestamp,
  revoked_by uuid,
  revoked_at timestamp,
  remarks text,
  PRIMARY KEY ((beneficiary_id), rep_id)
);

-- Lets a representative who logs in find the beneficiary they may act for.
CREATE TABLE IF NOT EXISTS psbc.representatives_by_user (
  user_id uuid,
  beneficiary_id uuid,
  rep_id uuid,
  PRIMARY KEY ((user_id), beneficiary_id)
);
