package repository

func schemaStatements() []string {
	return []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE IF NOT EXISTS schema_meta (id INTEGER PRIMARY KEY CHECK(id=1), version INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS inspection_cases (
			id TEXT PRIMARY KEY, case_number TEXT NOT NULL UNIQUE, venue_name TEXT NOT NULL,
			scope TEXT NOT NULL, status TEXT NOT NULL, version INTEGER NOT NULL,
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL, aggregate_json BLOB NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS rigging_assets (
			id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES inspection_cases(id) ON DELETE CASCADE,
			asset_code TEXT NOT NULL, asset_type TEXT NOT NULL, rated_load_kg REAL NOT NULL,
			brake_distance_limit_mm REAL NOT NULL, limit_device_required INTEGER NOT NULL,
			baseline_locked_at TEXT, UNIQUE(case_id, asset_code)
		)`,
		`CREATE TABLE IF NOT EXISTS load_test_records (
			id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES inspection_cases(id) ON DELETE CASCADE,
			asset_id TEXT NOT NULL REFERENCES rigging_assets(id), test_kind TEXT NOT NULL,
			applied_load_kg REAL NOT NULL, brake_distance_mm REAL NOT NULL,
			limit_triggered INTEGER NOT NULL, result TEXT NOT NULL, failure_reason TEXT NOT NULL,
			recorded_by TEXT NOT NULL, recorded_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS test_retest_links (
			record_id TEXT PRIMARY KEY REFERENCES load_test_records(id) ON DELETE CASCADE,
			case_id TEXT NOT NULL REFERENCES inspection_cases(id) ON DELETE CASCADE,
			original_record_id TEXT NOT NULL REFERENCES load_test_records(id)
		)`,
		`CREATE TABLE IF NOT EXISTS defects (
			id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES inspection_cases(id) ON DELETE CASCADE,
			asset_id TEXT NOT NULL REFERENCES rigging_assets(id), source_record_id TEXT,
			severity TEXT NOT NULL, description TEXT NOT NULL, status TEXT NOT NULL,
			remediation_evidence TEXT NOT NULL, review_comment TEXT NOT NULL, resolved_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS defect_evidence_versions (
			defect_id TEXT NOT NULL REFERENCES defects(id) ON DELETE CASCADE,
			case_id TEXT NOT NULL REFERENCES inspection_cases(id) ON DELETE CASCADE,
			version INTEGER NOT NULL, submitted_by TEXT NOT NULL, submitted_at TEXT NOT NULL,
			content TEXT NOT NULL, PRIMARY KEY(defect_id, version)
		)`,
		`CREATE TABLE IF NOT EXISTS defect_review_decisions (
			defect_id TEXT NOT NULL REFERENCES defects(id) ON DELETE CASCADE,
			case_id TEXT NOT NULL REFERENCES inspection_cases(id) ON DELETE CASCADE,
			sequence INTEGER NOT NULL, evidence_version INTEGER NOT NULL, reviewer TEXT NOT NULL,
			accepted INTEGER NOT NULL, comment TEXT NOT NULL, decided_at TEXT NOT NULL,
			PRIMARY KEY(defect_id, sequence)
		)`,
		`CREATE TABLE IF NOT EXISTS frozen_reports (
			case_id TEXT PRIMARY KEY REFERENCES inspection_cases(id) ON DELETE CASCADE,
			digest TEXT NOT NULL UNIQUE, content TEXT NOT NULL, frozen_by TEXT NOT NULL,
			frozen_at TEXT NOT NULL, version INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS certificates (
			id TEXT PRIMARY KEY, case_id TEXT NOT NULL UNIQUE REFERENCES inspection_cases(id) ON DELETE CASCADE,
			report_digest TEXT NOT NULL UNIQUE, certificate_number TEXT NOT NULL UNIQUE,
			issued_by TEXT NOT NULL, issued_at TEXT NOT NULL, verification_digest TEXT NOT NULL UNIQUE
		)`,
		`CREATE TABLE IF NOT EXISTS idempotency_records (
			key TEXT PRIMARY KEY, case_number TEXT NOT NULL, command TEXT NOT NULL,
			fingerprint TEXT NOT NULL, status_code INTEGER NOT NULL, response BLOB NOT NULL, created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS audit_events (
			case_id TEXT NOT NULL REFERENCES inspection_cases(id) ON DELETE CASCADE,
			sequence INTEGER NOT NULL, actor TEXT NOT NULL, role TEXT NOT NULL, command TEXT NOT NULL,
			before_version INTEGER NOT NULL, after_version INTEGER NOT NULL, occurred_at TEXT NOT NULL,
			result_digest TEXT NOT NULL, previous_hash TEXT NOT NULL, hash TEXT NOT NULL,
			PRIMARY KEY(case_id, sequence), UNIQUE(hash)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tests_case ON load_test_records(case_id, recorded_at)`,
		`CREATE INDEX IF NOT EXISTS idx_cases_queue ON inspection_cases(updated_at DESC, case_number ASC)`,
		`CREATE INDEX IF NOT EXISTS idx_defects_case ON defects(case_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_case ON audit_events(case_id, sequence)`,
	}
}
