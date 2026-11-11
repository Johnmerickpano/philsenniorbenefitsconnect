-- CQL-level smoke test with SYNTHETIC data. Exercises the main access paths of the beneficiary model.
-- Run:
--   docker cp infrastructure\scripts\smoke-test.cql psbc-scylla:/tmp/smoke-test.cql
--   docker exec psbc-scylla cqlsh -f /tmp/smoke-test.cql
-- Expected results are in the comments. Cleans up after itself.
USE psbc;

-- 1) Uniqueness of OSCA ID (LWT). Expect [applied]=True, then False.
INSERT INTO beneficiaries_by_osca_id (osca_id, beneficiary_id, created_at)
VALUES ('OSCA-TEST-001', 11111111-1111-1111-1111-111111111111, toTimestamp(now())) IF NOT EXISTS;
INSERT INTO beneficiaries_by_osca_id (osca_id, beneficiary_id, created_at)
VALUES ('OSCA-TEST-001', 22222222-2222-2222-2222-222222222222, toTimestamp(now())) IF NOT EXISTS;

-- 2) Multi-table write (logged batch). Expect no error.
BEGIN BATCH
  INSERT INTO beneficiaries_by_id (beneficiary_id, osca_id, first_name, last_name, sort_name, birth_date, sex,
    barangay_code, barangay_name, record_status, row_version, created_at, updated_at)
  VALUES (11111111-1111-1111-1111-111111111111, 'OSCA-TEST-001', 'Juan', 'Dela Cruz', 'dela cruz juan',
    '1958-03-14', 'M', 'ZZTEST', 'Test Barangay', 'VERIFIED', 1, toTimestamp(now()), toTimestamp(now()));
  INSERT INTO beneficiaries_by_barangay (barangay_code, sort_name, beneficiary_id, osca_id, display_name, birth_date, record_status, updated_at)
  VALUES ('ZZTEST', 'dela cruz juan', 11111111-1111-1111-1111-111111111111, 'OSCA-TEST-001', 'Juan Dela Cruz', '1958-03-14', 'VERIFIED', toTimestamp(now()));
  INSERT INTO beneficiaries_by_status (record_status, barangay_code, sort_name, beneficiary_id, osca_id, display_name, birth_date, updated_at)
  VALUES ('VERIFIED', 'ZZTEST', 'dela cruz juan', 11111111-1111-1111-1111-111111111111, 'OSCA-TEST-001', 'Juan Dela Cruz', '1958-03-14', toTimestamp(now()));
  INSERT INTO beneficiary_search (barangay_code, name_prefix, sort_name, beneficiary_id, osca_id, display_name, record_status)
  VALUES ('ZZTEST', 'jua', 'dela cruz juan', 11111111-1111-1111-1111-111111111111, 'OSCA-TEST-001', 'Juan Dela Cruz', 'VERIFIED');
  INSERT INTO beneficiary_search (barangay_code, name_prefix, sort_name, beneficiary_id, osca_id, display_name, record_status)
  VALUES ('ZZTEST', 'del', 'dela cruz juan', 11111111-1111-1111-1111-111111111111, 'OSCA-TEST-001', 'Juan Dela Cruz', 'VERIFIED');
  INSERT INTO beneficiary_search (barangay_code, name_prefix, sort_name, beneficiary_id, osca_id, display_name, record_status)
  VALUES ('ZZTEST', 'cru', 'dela cruz juan', 11111111-1111-1111-1111-111111111111, 'OSCA-TEST-001', 'Juan Dela Cruz', 'VERIFIED');
  INSERT INTO beneficiary_status_history (beneficiary_id, changed_at, event_id, old_status, new_status, remarks)
  VALUES (11111111-1111-1111-1111-111111111111, toTimestamp(now()), now(), 'PENDING_VERIFICATION', 'VERIFIED', 'smoke test');
APPLY BATCH;

-- 3) Lookup by OSCA ID -> id -> profile. Expect 1 row each.
SELECT beneficiary_id FROM beneficiaries_by_osca_id WHERE osca_id = 'OSCA-TEST-001';
SELECT osca_id, first_name, last_name, birth_date, record_status FROM beneficiaries_by_id
WHERE beneficiary_id = 11111111-1111-1111-1111-111111111111;

-- 4) Name search by prefix bucket (no ALLOW FILTERING). Expect 1 row.
SELECT display_name, osca_id, record_status FROM beneficiary_search WHERE barangay_code = 'ZZTEST' AND name_prefix = 'cru';

-- 5) Admin list + status filter + single-partition count. Expect 1 row / count 1.
SELECT display_name, record_status FROM beneficiaries_by_barangay WHERE barangay_code = 'ZZTEST' LIMIT 20;
SELECT COUNT(*) FROM beneficiaries_by_status WHERE record_status = 'VERIFIED' AND barangay_code = 'ZZTEST';

-- 6) Status history. Expect 1 row.
SELECT old_status, new_status, remarks FROM beneficiary_status_history
WHERE beneficiary_id = 11111111-1111-1111-1111-111111111111;

-- 7) Cleanup
DELETE FROM beneficiaries_by_osca_id WHERE osca_id = 'OSCA-TEST-001';
DELETE FROM beneficiaries_by_id WHERE beneficiary_id = 11111111-1111-1111-1111-111111111111;
DELETE FROM beneficiaries_by_barangay WHERE barangay_code = 'ZZTEST';
DELETE FROM beneficiaries_by_status WHERE record_status = 'VERIFIED' AND barangay_code = 'ZZTEST';
DELETE FROM beneficiary_search WHERE barangay_code = 'ZZTEST' AND name_prefix = 'jua';
DELETE FROM beneficiary_search WHERE barangay_code = 'ZZTEST' AND name_prefix = 'del';
DELETE FROM beneficiary_search WHERE barangay_code = 'ZZTEST' AND name_prefix = 'cru';
DELETE FROM beneficiary_status_history WHERE beneficiary_id = 11111111-1111-1111-1111-111111111111;
