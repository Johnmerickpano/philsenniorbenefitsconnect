-- 004: concerns/follow-up, review flags, audit log, notifications, idempotency, dashboard cache

-- ---------- Concerns / follow-up cases ----------
-- status: OPEN | IN_PROGRESS | RESOLVED | CLOSED
CREATE TABLE IF NOT EXISTS psbc.concerns_by_id (
  concern_id timeuuid PRIMARY KEY,
  beneficiary_id uuid,
  submitted_by uuid,
  category text,
  subject text,
  description text,
  status text,
  assigned_to uuid,
  created_at timestamp,
  updated_at timestamp,
  resolved_at timestamp
);

CREATE TABLE IF NOT EXISTS psbc.concerns_by_beneficiary (
  beneficiary_id uuid,
  created_at timestamp,
  concern_id timeuuid,
  subject text,
  status text,
  updated_at timestamp,
  PRIMARY KEY ((beneficiary_id), created_at, concern_id)
) WITH CLUSTERING ORDER BY (created_at DESC, concern_id DESC);

-- Working set only: rows exist while status is OPEN or IN_PROGRESS (moved out when resolved).
-- "Pending follow-ups" = COUNT(*) per status partition.
CREATE TABLE IF NOT EXISTS psbc.concerns_open (
  status text,
  created_at timestamp,
  concern_id timeuuid,
  beneficiary_id uuid,
  category text,
  subject text,
  assigned_to uuid,
  PRIMARY KEY ((status), created_at, concern_id)
) WITH CLUSTERING ORDER BY (created_at ASC, concern_id ASC);

CREATE TABLE IF NOT EXISTS psbc.concern_updates (
  concern_id timeuuid,
  update_id timeuuid,
  author_id uuid,
  author_role text,
  message text,
  new_status text,
  visible_to_citizen boolean,
  PRIMARY KEY ((concern_id), update_id)
) WITH CLUSTERING ORDER BY (update_id ASC);

-- ---------- Review flags ----------
-- flag_type: POSSIBLE_DUPLICATE | DECEASED_VERIFICATION | INCONSISTENT_INFO | INCOMPLETE_RECORD
-- status: OPEN | UNDER_REVIEW | RESOLVED | DISMISSED
-- Flags NEVER change eligibility, merge or delete records by themselves. Staff decide; every decision is audited.
CREATE TABLE IF NOT EXISTS psbc.review_flags_by_id (
  flag_id timeuuid PRIMARY KEY,
  beneficiary_id uuid,
  related_beneficiary_id uuid,
  flag_type text,
  status text,
  source text,                    -- MANUAL | IMPORT | SYSTEM_CHECK
  reason text,
  raised_by uuid,
  raised_at timestamp,
  decided_by uuid,
  decided_at timestamp,
  decision text,
  decision_remarks text
);

-- Working set (OPEN / UNDER_REVIEW). Row is removed when a decision is recorded.
CREATE TABLE IF NOT EXISTS psbc.review_flags_open (
  flag_type text,
  raised_at timestamp,
  flag_id timeuuid,
  beneficiary_id uuid,
  related_beneficiary_id uuid,
  status text,
  reason text,
  PRIMARY KEY ((flag_type), raised_at, flag_id)
) WITH CLUSTERING ORDER BY (raised_at ASC, flag_id ASC);

-- Decided flags, bucketed by month ('YYYY-MM') to bound partition size.
CREATE TABLE IF NOT EXISTS psbc.review_flags_closed (
  flag_type text,
  month_bucket text,
  decided_at timestamp,
  flag_id timeuuid,
  beneficiary_id uuid,
  status text,
  decision text,
  PRIMARY KEY ((flag_type, month_bucket), decided_at, flag_id)
) WITH CLUSTERING ORDER BY (decided_at DESC, flag_id DESC);

CREATE TABLE IF NOT EXISTS psbc.review_flags_by_beneficiary (
  beneficiary_id uuid,
  raised_at timestamp,
  flag_id timeuuid,
  flag_type text,
  status text,
  PRIMARY KEY ((beneficiary_id), raised_at, flag_id)
) WITH CLUSTERING ORDER BY (raised_at DESC, flag_id DESC);

-- ---------- Audit log (append-only; the app exposes no update/delete path) ----------
CREATE TABLE IF NOT EXISTS psbc.audit_by_day (
  day_bucket date,
  event_id timeuuid,
  event_time timestamp,
  actor_id uuid,
  actor_role text,
  action text,
  entity_type text,
  entity_id text,
  result text,                    -- SUCCESS | DENIED | FAILED
  before_json text,
  after_json text,
  ip_address text,
  request_id text,
  PRIMARY KEY ((day_bucket), event_id)
) WITH CLUSTERING ORDER BY (event_id DESC);

CREATE TABLE IF NOT EXISTS psbc.audit_by_entity (
  entity_type text,
  entity_id text,
  event_id timeuuid,
  event_time timestamp,
  actor_id uuid,
  actor_role text,
  action text,
  result text,
  before_json text,
  after_json text,
  PRIMARY KEY ((entity_type, entity_id), event_id)
) WITH CLUSTERING ORDER BY (event_id DESC);

CREATE TABLE IF NOT EXISTS psbc.audit_by_actor (
  actor_id uuid,
  event_id timeuuid,
  event_time timestamp,
  actor_role text,
  action text,
  entity_type text,
  entity_id text,
  result text,
  PRIMARY KEY ((actor_id), event_id)
) WITH CLUSTERING ORDER BY (event_id DESC);

-- ---------- Notifications ----------
CREATE TABLE IF NOT EXISTS psbc.notifications_by_user (
  user_id uuid,
  notification_id timeuuid,
  title text,
  body text,
  category text,
  is_read boolean,
  PRIMARY KEY ((user_id), notification_id)
) WITH CLUSTERING ORDER BY (notification_id DESC);

-- ---------- Idempotency (duplicate POST protection). Rows expire after 24 h. ----------
CREATE TABLE IF NOT EXISTS psbc.idempotency_keys (
  idempotency_key text PRIMARY KEY,
  user_id uuid,
  request_hash text,
  result_ref text,
  created_at timestamp
) WITH default_time_to_live = 86400;

-- ---------- Dashboard ----------
-- Live figures come from single-partition COUNT(*) queries. This table is only a short-lived cache.
CREATE TABLE IF NOT EXISTS psbc.dashboard_metrics_cache (
  metric_key text PRIMARY KEY,
  metric_value bigint,
  computed_at timestamp
);

-- Month-end snapshots so "vs last month" can be computed from real stored values later.
CREATE TABLE IF NOT EXISTS psbc.dashboard_metric_snapshots (
  metric_key text,
  month_bucket text,              -- 'YYYY-MM'
  metric_value bigint,
  captured_at timestamp,
  PRIMARY KEY ((metric_key), month_bucket)
) WITH CLUSTERING ORDER BY (month_bucket DESC);
