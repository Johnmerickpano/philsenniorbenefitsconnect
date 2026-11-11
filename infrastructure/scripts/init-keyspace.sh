#!/bin/bash
# Creates the keyspace if missing. Safe to run repeatedly (IF NOT EXISTS).
set -euo pipefail
KS="${SCYLLA_KEYSPACE:-psbc}"
RF="${SCYLLA_REPLICATION_FACTOR:-1}"
echo "Creating keyspace '${KS}' (RF=${RF}) on host 'scylla'..."
cqlsh scylla 9042 -e "CREATE KEYSPACE IF NOT EXISTS ${KS} WITH replication = {'class': 'NetworkTopologyStrategy', 'replication_factor': ${RF}} AND durable_writes = true AND TABLETS = {'enabled': false};"
cqlsh scylla 9042 -e "DESCRIBE KEYSPACE ${KS};" >/dev/null
echo "Keyspace ready."
