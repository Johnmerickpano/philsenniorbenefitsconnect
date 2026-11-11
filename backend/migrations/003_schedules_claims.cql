-- 003_schedules_claims: payout schedules and recorded claims
-- status: DRAFT | PUBLISHED | CANCELLED. Only PUBLISHED schedules are ever shown to citizens.
-- amount_per_beneficiary is entered by OSCA; the system never computes or transfers money.

CREATE TABLE IF NOT EXISTS psbc.payout_schedules_by_id (
  schedule_id uuid PRIMARY KEY,
  title text,
  year_quarter text,              -- e.g. '2026-Q4'
  payout_date date,
  payout_time time,
  venue_name text,
  venue_address text,
  applicable_barangay_codes set<text>,
  amount_per_beneficiary decimal,
  instructions text,
  status text,
  row_version bigint,
  created_at timestamp,
  created_by uuid,
  updated_at timestamp,
  updated_by uuid,
  published_at timestamp,
  published_by uuid
);

-- Admin lists: "upcoming/drafts for a quarter".
CREATE TABLE IF NOT EXISTS psbc.schedules_by_quarter_status (
  year_quarter text,
  status text,
  payout_date date,
  schedule_id uuid,
  title text,
  payout_time time,
  venue_name text,
  PRIMARY KEY ((year_quarter, status), payout_date, schedule_id)
) WITH CLUSTERING ORDER BY (payout_date ASC, schedule_id ASC);

-- Citizen view. Rows exist ONLY while a schedule is PUBLISHED (deleted on cancel/unpublish).
CREATE TABLE IF NOT EXISTS psbc.schedules_by_barangay (
  barangay_code text,
  year_quarter text,
  payout_date date,
  schedule_id uuid,
  title text,
  payout_time time,
  venue_name text,
  venue_address text,
  amount_per_beneficiary decimal,
  instructions text,
  published_at timestamp,
  PRIMARY KEY ((barangay_code, year_quarter), payout_date, schedule_id)
) WITH CLUSTERING ORDER BY (payout_date ASC, schedule_id ASC);

CREATE TABLE IF NOT EXISTS psbc.schedule_history (
  schedule_id uuid,
  event_id timeuuid,
  changed_at timestamp,
  action text,                    -- CREATED | EDITED | PUBLISHED | CANCELLED
  changed_by uuid,
  summary text,
  PRIMARY KEY ((schedule_id), event_id)
) WITH CLUSTERING ORDER BY (event_id DESC);

-- A staff member records that a claim happened. No money moves through this system.
-- claim_method: SELF | REPRESENTATIVE. Method is in the partition key so each count is a single-partition query.
CREATE TABLE IF NOT EXISTS psbc.payout_claims_by_schedule_method (
  schedule_id uuid,
  claim_method text,
  beneficiary_id uuid,
  rep_id uuid,
  recorded_by uuid,
  recorded_at timestamp,
  remarks text,
  PRIMARY KEY ((schedule_id, claim_method), beneficiary_id)
);

CREATE TABLE IF NOT EXISTS psbc.payout_claims_by_beneficiary (
  beneficiary_id uuid,
  recorded_at timestamp,
  schedule_id uuid,
  claim_method text,
  rep_id uuid,
  PRIMARY KEY ((beneficiary_id), recorded_at, schedule_id)
) WITH CLUSTERING ORDER BY (recorded_at DESC, schedule_id ASC);
