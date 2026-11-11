-- 005_imports: Excel masterlist import workflow
-- UPLOAD -> VALIDATE -> PREVIEW -> CONFIRM -> PROCESS -> REPORT
-- status: UPLOADED | VALIDATED | CONFIRMED | PROCESSING | COMPLETED | FAILED | CANCELLED

CREATE TABLE IF NOT EXISTS psbc.import_batches_by_id (
  batch_id uuid PRIMARY KEY,
  file_name text,
  file_hash text,
  status text,
  column_mapping text,            -- JSON
  total_rows int,
  valid_rows int,
  error_rows int,
  duplicate_rows int,
  created_by uuid,
  created_at timestamp,
  confirmed_by uuid,
  confirmed_at timestamp,
  completed_at timestamp
);

CREATE TABLE IF NOT EXISTS psbc.import_batches_by_month (
  month_bucket text,
  created_at timestamp,
  batch_id uuid,
  file_name text,
  status text,
  PRIMARY KEY ((month_bucket), created_at, batch_id)
) WITH CLUSTERING ORDER BY (created_at DESC, batch_id ASC);

-- The same file cannot be imported twice: INSERT ... IF NOT EXISTS on file_hash.
CREATE TABLE IF NOT EXISTS psbc.import_file_hashes (
  file_hash text PRIMARY KEY,
  batch_id uuid,
  created_at timestamp
);

-- Staging rows for preview. Expire after 30 days.
CREATE TABLE IF NOT EXISTS psbc.import_rows_by_batch (
  batch_id uuid,
  row_number int,
  raw_json text,
  normalized_json text,
  row_status text,                -- VALID | ERROR | DUPLICATE_WARNING
  action text,                    -- CREATE | UPDATE | SKIP
  PRIMARY KEY ((batch_id), row_number)
) WITH default_time_to_live = 2592000;

CREATE TABLE IF NOT EXISTS psbc.import_errors_by_batch (
  batch_id uuid,
  row_number int,
  error_code text,
  field_name text,
  message text,
  PRIMARY KEY ((batch_id), row_number, error_code)
) WITH default_time_to_live = 2592000;
