# Database Design (ScyllaDB) - PhilSenior Benefits Connect

Keyspace `psbc`, created with **tablets disabled** (required for lightweight transactions).
Dev: RF=1, single node. Production: 3+ nodes, RF=3. Migrations: `backend/migrations/*.cql`.

## Modeling rules used
1. One table per query. Duplicated data is intentional.
2. No joins, no secondary indexes, no ALLOW FILTERING.
3. Uniqueness only through LWT (`IF NOT EXISTS`) on a dedicated lookup table. LWT cannot span partitions, so create flow is: (a) LWT insert into the lookup table, (b) logged batch for the remaining tables, (c) retry (b) idempotently on failure.
4. Metrics are single-partition `COUNT(*)` queries summed by the backend, never full-table scans.
5. "Working set" tables (open flags, open concerns) hold only active items; decided items move to bucketed tables.

## Tables and trade-offs
| Table | Partition / clustering | Serves | Trade-off |
|---|---|---|---|
| users_by_id | user_id | load user | none |
| users_by_username | username | login, unique username (LWT) | second write |
| staff_by_role | (user_role) / user_id | admin staff list | staff only; small partitions |
| refresh_tokens_by_hash | token_hash | validate/rotate (TTL) | stores hash only |
| refresh_tokens_by_user | (user_id) / token_hash | revoke all sessions | extra write |
| activation_codes_by_hash | code_hash | senior account activation | hash only, expires |
| rate_limit_events | (bucket_key) / event_id DESC | rate limiting, TTL 1h | counted by partition read |
| beneficiaries_by_id | beneficiary_id | profile (source of truth) | updates fan out to list tables |
| beneficiaries_by_osca_id | osca_id | ID lookup, unique OSCA ID | second write |
| beneficiary_search | (barangay_code, name_prefix) / sort_name, id | name search | 1 row per name part; prefix bucket = 3 letters, then app-side filter; search needs a barangay (or bounded fan-out over the barangay list) |
| beneficiaries_by_barangay | (barangay_code) / sort_name, id | admin list | large barangay = large partition (fine at expected scale, monitor) |
| beneficiaries_by_status | (record_status, barangay_code) / sort_name, id | status filter, counts | status change = delete + insert in a batch |
| beneficiary_status_history | (beneficiary_id) / changed_at DESC | timeline | append-only |
| representatives_by_beneficiary | (beneficiary_id) / rep_id | authorization checks | validity dates enforced in app |
| representatives_by_user | (user_id) / beneficiary_id | rep login | extra write |
| payout_schedules_by_id | schedule_id | schedule source of truth | |
| schedules_by_quarter_status | (year_quarter, status) / payout_date | admin lists | move on status change |
| schedules_by_barangay | (barangay_code, year_quarter) / payout_date | citizen view | rows exist only while PUBLISHED, so drafts can never leak |
| schedule_history | (schedule_id) / event_id DESC | change trail | |
| payout_claims_by_schedule_method | (schedule_id, claim_method) / beneficiary_id | claim counts/reports | |
| payout_claims_by_beneficiary | (beneficiary_id) / recorded_at DESC | claim history | |
| concerns_by_id / _by_beneficiary / concerns_open / concern_updates | see CQL | follow-up cases | "follow-up case" = concern with status workflow |
| review_flags_by_id / _open / _closed / _by_beneficiary | see CQL | review queue | decision = batch: remove from open, add to closed (monthly bucket) |
| audit_by_day / _by_entity / _by_actor | day / entity / actor, event_id DESC | audit views | triple write; no update/delete path in the API |
| notifications_by_user | (user_id) / notification_id DESC | inbox | |
| idempotency_keys | idempotency_key (TTL 24h) | duplicate POST protection | |
| dashboard_metrics_cache, dashboard_metric_snapshots | metric_key | cache; month-over-month deltas | snapshots start empty; delta hidden until a prior month exists |
| import_batches_by_id / _by_month / import_file_hashes | batch_id / month / file_hash (LWT) | import workflow, no double import | |
| import_rows_by_batch, import_errors_by_batch | (batch_id) / row_number | preview, error report | TTL 30 days |

## Known limitations (to disclose in the defense)
- Audit-log immutability is enforced in the application. Database-level role restrictions require enabling ScyllaDB authentication (planned for the deployment phase).
- Multi-table writes are not atomic across LWT boundaries; the repository layer must be idempotent and retry.
- Name search is prefix-bucketed, not full-text; fuzzy matching is out of scope.
- ScyllaDB is chosen as a project requirement; at OSCA scale a relational database would also work. The trade-off is documented here for the defense.
