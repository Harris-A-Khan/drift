package database

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func allowedAuthTablesForTest(tables ...string) map[string]bool {
	m := map[string]bool{}
	for _, table := range tables {
		m[normalizeQualifiedName(table)] = true
	}
	return m
}

func allowedAllTablesForTest(tables ...string) map[string]bool {
	m := map[string]bool{}
	for _, table := range tables {
		m[normalizeQualifiedName(table)] = true
	}
	return m
}

func TestPreprocessBackupFile_RemovesRestrictCommands(t *testing.T) {
	inputDir := t.TempDir()
	inputPath := filepath.Join(inputDir, "input.sql")
	input := strings.Join([]string{
		"-- Header",
		"\\restrict abc123",
		"COPY auth.users (id, email) FROM stdin;",
		"00000000-0000-0000-0000-000000000001\ttaha@example.com",
		"\\.",
		"   \\unrestrict abc123",
		"SELECT 1;",
		"",
	}, "\n")

	if err := os.WriteFile(inputPath, []byte(input), 0644); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}

	processedPath, err := preprocessBackupFile(inputPath, allowedAuthTablesForTest("auth.users"))
	if err != nil {
		t.Fatalf("preprocessBackupFile() error = %v", err)
	}
	defer os.Remove(processedPath)

	processedBytes, err := os.ReadFile(processedPath)
	if err != nil {
		t.Fatalf("failed to read processed file: %v", err)
	}
	processed := string(processedBytes)

	if strings.Contains(processed, "\\restrict") {
		t.Fatalf("processed file should not contain \\\\restrict commands")
	}
	if strings.Contains(processed, "\\unrestrict") {
		t.Fatalf("processed file should not contain \\\\unrestrict commands")
	}
	if !strings.Contains(processed, "SET session_replication_role = replica;") {
		t.Fatalf("processed file should set session_replication_role to replica")
	}
	if !strings.Contains(processed, "SET session_replication_role = DEFAULT;") {
		t.Fatalf("processed file should reset session_replication_role")
	}
	if !strings.Contains(processed, "COPY auth.users") {
		t.Fatalf("processed file should preserve auth.users COPY data")
	}
}

func TestRestoreSQL_UsesOnErrorStop(t *testing.T) {
	tempDir := t.TempDir()

	argsPath := filepath.Join(tempDir, "args.txt")
	psqlPath := filepath.Join(tempDir, "psql")
	psqlScript := strings.Join([]string{
		"#!/bin/sh",
		"printf '%s\\n' \"$@\" > \"$DRIFT_TEST_ARGS_FILE\"",
		"exit 0",
		"",
	}, "\n")

	if err := os.WriteFile(psqlPath, []byte(psqlScript), 0755); err != nil {
		t.Fatalf("failed to write fake psql: %v", err)
	}

	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+originalPath)
	t.Setenv("DRIFT_TEST_ARGS_FILE", argsPath)

	inputPath := filepath.Join(tempDir, "backup.sql")
	if err := os.WriteFile(inputPath, []byte("SELECT 1;\n"), 0644); err != nil {
		t.Fatalf("failed to write input backup: %v", err)
	}

	opts := RestoreOptions{
		Host:      "localhost",
		Port:      5432,
		Database:  "postgres",
		User:      "postgres",
		Password:  "secret",
		InputFile: inputPath,
	}

	if err := restoreSQL(opts); err != nil {
		t.Fatalf("restoreSQL() error = %v", err)
	}

	argsBytes, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("failed to read captured args: %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(argsBytes)), "\n")

	if !hasArgPair(args, "-v", "ON_ERROR_STOP=1") {
		t.Fatalf("restoreSQL should pass -v ON_ERROR_STOP=1 to psql, args = %v", args)
	}
	if !hasFlag(args, "-f") {
		t.Fatalf("restoreSQL should pass -f <file> to psql, args = %v", args)
	}
}

func TestRestoreSQL_FailsOnNonZeroExitCode(t *testing.T) {
	tempDir := t.TempDir()

	psqlPath := filepath.Join(tempDir, "psql")
	psqlScript := strings.Join([]string{
		"#!/bin/sh",
		"echo \"psql: error: synthetic restore failure\" 1>&2",
		"exit 2",
		"",
	}, "\n")

	if err := os.WriteFile(psqlPath, []byte(psqlScript), 0755); err != nil {
		t.Fatalf("failed to write fake psql: %v", err)
	}

	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+originalPath)

	inputPath := filepath.Join(tempDir, "backup.sql")
	if err := os.WriteFile(inputPath, []byte("SELECT 1;\n"), 0644); err != nil {
		t.Fatalf("failed to write input backup: %v", err)
	}

	opts := RestoreOptions{
		Host:      "localhost",
		Port:      5432,
		Database:  "postgres",
		User:      "postgres",
		Password:  "secret",
		InputFile: inputPath,
	}

	err := restoreSQL(opts)
	if err == nil {
		t.Fatalf("restoreSQL() expected error for non-zero psql exit code")
	}
	if !strings.Contains(err.Error(), "synthetic restore failure") {
		t.Fatalf("restoreSQL() error = %v, want synthetic restore failure", err)
	}
}

func TestPreprocessBackupFile_RemovesEventTriggerStatements(t *testing.T) {
	inputDir := t.TempDir()
	inputPath := filepath.Join(inputDir, "input.sql")
	input := strings.Join([]string{
		"DROP EVENT TRIGGER IF EXISTS pgrst_drop_watch;",
		"CREATE EVENT TRIGGER pgrst_drop_watch",
		"    ON ddl_command_end",
		"    EXECUTE FUNCTION pgrst_ddl_watcher();",
		"COMMENT ON EVENT TRIGGER pgrst_drop_watch IS 'watcher';",
		"ALTER EVENT TRIGGER pgrst_drop_watch OWNER TO postgres;",
		"COPY public.users (id, username) FROM stdin;",
		"public_row_42\ttaha",
		"\\.",
		"",
	}, "\n")

	if err := os.WriteFile(inputPath, []byte(input), 0644); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}

	processedPath, err := preprocessBackupFile(inputPath, allowedAuthTablesForTest("auth.users"))
	if err != nil {
		t.Fatalf("preprocessBackupFile() error = %v", err)
	}
	defer os.Remove(processedPath)

	processedBytes, err := os.ReadFile(processedPath)
	if err != nil {
		t.Fatalf("failed to read processed file: %v", err)
	}
	processed := string(processedBytes)

	if strings.Contains(processed, "EVENT TRIGGER pgrst_drop_watch") {
		t.Fatalf("processed file should not contain event trigger statements")
	}
	if !strings.Contains(processed, "COPY public.users") {
		t.Fatalf("processed file should preserve allowed COPY statements")
	}
	if !strings.Contains(processed, "public_row_42\ttaha") {
		t.Fatalf("processed file should preserve allowed COPY payload rows")
	}
}

func TestPreprocessBackupFile_RemovesStorageSchemaStatementsAndCopyData(t *testing.T) {
	inputDir := t.TempDir()
	inputPath := filepath.Join(inputDir, "input.sql")
	input := strings.Join([]string{
		"ALTER TABLE IF EXISTS ONLY storage.vector_indexes DROP CONSTRAINT IF EXISTS vector_indexes_bucket_id_fkey;",
		"COPY storage.vector_indexes (id, name) FROM stdin;",
		"storage_row_1\tidx_name",
		"\\.",
		"COPY public.users (id, username) FROM stdin;",
		"public_row_1\ttaha",
		"\\.",
		"SELECT 99;",
		"",
	}, "\n")

	if err := os.WriteFile(inputPath, []byte(input), 0644); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}

	processedPath, err := preprocessBackupFile(inputPath, allowedAuthTablesForTest("auth.users"))
	if err != nil {
		t.Fatalf("preprocessBackupFile() error = %v", err)
	}
	defer os.Remove(processedPath)

	processedBytes, err := os.ReadFile(processedPath)
	if err != nil {
		t.Fatalf("failed to read processed file: %v", err)
	}
	processed := string(processedBytes)

	if strings.Contains(processed, "storage.vector_indexes") {
		t.Fatalf("processed file should not contain storage schema statements")
	}
	if strings.Contains(processed, "storage_row_1") {
		t.Fatalf("processed file should not contain storage schema COPY payload rows")
	}
	if !strings.Contains(processed, "COPY public.users") {
		t.Fatalf("processed file should preserve non-storage COPY statements")
	}
	if !strings.Contains(processed, "public_row_1\ttaha") {
		t.Fatalf("processed file should preserve non-storage COPY payload rows")
	}
	if strings.Contains(processed, "SELECT 99;") {
		t.Fatalf("processed file should not preserve unrelated SQL statements")
	}
}

func TestPreprocessBackupFile_RespectsAllowedAuthTables(t *testing.T) {
	inputDir := t.TempDir()
	inputPath := filepath.Join(inputDir, "input.sql")
	input := strings.Join([]string{
		"COPY auth.users (id, email) FROM stdin;",
		"user_1\ttaha@example.com",
		"\\.",
		"COPY auth.sso_domains (id, domain) FROM stdin;",
		"sso_1\texample.com",
		"\\.",
		"COPY auth.schema_migrations (version) FROM stdin;",
		"20250101000000",
		"\\.",
		"",
	}, "\n")

	if err := os.WriteFile(inputPath, []byte(input), 0644); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}

	allowed := allowedAuthTablesForTest("auth.users", "auth.sso_domains")
	processedPath, err := preprocessBackupFile(inputPath, allowed)
	if err != nil {
		t.Fatalf("preprocessBackupFile() error = %v", err)
	}
	defer os.Remove(processedPath)

	processedBytes, err := os.ReadFile(processedPath)
	if err != nil {
		t.Fatalf("failed to read processed file: %v", err)
	}
	processed := string(processedBytes)

	if !strings.Contains(processed, "COPY auth.users") {
		t.Fatalf("processed file should keep allowed auth.users copy")
	}
	if !strings.Contains(processed, "COPY auth.sso_domains") {
		t.Fatalf("processed file should keep allowed auth.sso_domains copy")
	}
	if strings.Contains(processed, "COPY auth.schema_migrations") {
		t.Fatalf("processed file should skip disallowed auth.schema_migrations copy")
	}
	if !strings.Contains(processed, "TRUNCATE TABLE auth.users CASCADE;") {
		t.Fatalf("processed file should include truncate for copied auth.users")
	}
	if !strings.Contains(processed, "TRUNCATE TABLE auth.sso_domains CASCADE;") {
		t.Fatalf("processed file should include truncate for copied auth.sso_domains")
	}
}

func TestPreprocessBackupFileWithScope_AllowsNonPublicWhenEnabled(t *testing.T) {
	inputDir := t.TempDir()
	inputPath := filepath.Join(inputDir, "input.sql")
	input := strings.Join([]string{
		"COPY storage.vector_indexes (id, name) FROM stdin;",
		"storage_row_1\tidx_name",
		"\\.",
		"COPY public.users (id, username) FROM stdin;",
		"public_row_1\ttaha",
		"\\.",
		"",
	}, "\n")

	if err := os.WriteFile(inputPath, []byte(input), 0644); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}

	allowedAll := allowedAllTablesForTest("storage.vector_indexes", "public.users")
	processedPath, err := preprocessBackupFileWithScope(inputPath, nil, allowedAll)
	if err != nil {
		t.Fatalf("preprocessBackupFileWithScope() error = %v", err)
	}
	defer os.Remove(processedPath)

	processedBytes, err := os.ReadFile(processedPath)
	if err != nil {
		t.Fatalf("failed to read processed file: %v", err)
	}
	processed := string(processedBytes)

	if !strings.Contains(processed, "COPY storage.vector_indexes") {
		t.Fatalf("all-scope preprocessing should keep allowed storage table copy")
	}
	if !strings.Contains(processed, "storage_row_1\tidx_name") {
		t.Fatalf("all-scope preprocessing should keep allowed storage table data")
	}
	if !strings.Contains(processed, "TRUNCATE TABLE storage.vector_indexes CASCADE;") {
		t.Fatalf("all-scope preprocessing should include truncate for allowed storage table")
	}
	if !strings.Contains(processed, "COPY public.users") {
		t.Fatalf("all-scope preprocessing should keep allowed public table copy")
	}
}

func TestPreprocessBackupFile_TruncatesBeforeAllCopies(t *testing.T) {
	// Regression test: when child tables (session_shields) appear before parent
	// tables (sessions, shields) in alphabetical dump order, the old inline
	// TRUNCATE approach would wipe already-loaded child data via CASCADE.
	// The fix emits all TRUNCATEs up front before any COPY blocks.
	inputDir := t.TempDir()
	inputPath := filepath.Join(inputDir, "input.sql")
	input := strings.Join([]string{
		"COPY public.session_shields (session_id, shield_id) FROM stdin;",
		"sid1\tshid1",
		"\\.",
		"COPY public.sessions (id, user_id) FROM stdin;",
		"sid1\tuid1",
		"\\.",
		"COPY public.shields (id, user_id) FROM stdin;",
		"shid1\tuid1",
		"\\.",
		"",
	}, "\n")

	if err := os.WriteFile(inputPath, []byte(input), 0644); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}

	processedPath, err := preprocessBackupFile(inputPath, nil)
	if err != nil {
		t.Fatalf("preprocessBackupFile() error = %v", err)
	}
	defer os.Remove(processedPath)

	processedBytes, err := os.ReadFile(processedPath)
	if err != nil {
		t.Fatalf("failed to read processed file: %v", err)
	}
	processed := string(processedBytes)

	// All three TRUNCATEs must appear before the first COPY
	firstCopy := strings.Index(processed, "COPY public.")
	if firstCopy == -1 {
		t.Fatalf("processed file should contain COPY statements")
	}

	for _, table := range []string{"public.session_shields", "public.sessions", "public.shields"} {
		truncStmt := fmt.Sprintf("TRUNCATE TABLE %s CASCADE;", table)
		truncIdx := strings.Index(processed, truncStmt)
		if truncIdx == -1 {
			t.Fatalf("processed file should contain %s", truncStmt)
		}
		if truncIdx >= firstCopy {
			t.Fatalf("TRUNCATE for %s (pos %d) should appear before first COPY (pos %d)", table, truncIdx, firstCopy)
		}
	}

	// All COPY blocks and their data should be present
	if !strings.Contains(processed, "sid1\tshid1") {
		t.Fatalf("processed file should contain session_shields data")
	}
	if !strings.Contains(processed, "sid1\tuid1") {
		t.Fatalf("processed file should contain sessions data")
	}
	if !strings.Contains(processed, "shid1\tuid1") {
		t.Fatalf("processed file should contain shields data")
	}
}

func TestIsAllowedSetvalStatement_AllScope(t *testing.T) {
	allowedAll := allowedAllTablesForTest("storage.vector_indexes")

	if !isAllowedSetvalStatement("SELECT pg_catalog.setval('storage.vector_indexes_id_seq', 42, true);", nil, allowedAll) {
		t.Fatalf("expected non-system setval to be allowed in all-scope mode")
	}
	if isAllowedSetvalStatement("SELECT pg_catalog.setval('pg_catalog.some_seq', 1, true);", nil, allowedAll) {
		t.Fatalf("expected pg_catalog setval to be blocked in all-scope mode")
	}
	if isAllowedSetvalStatement("SELECT pg_catalog.setval('information_schema.some_seq', 1, true);", nil, allowedAll) {
		t.Fatalf("expected information_schema setval to be blocked in all-scope mode")
	}
}

func TestResolveAllowedAuthCopyTables_UsesOverride(t *testing.T) {
	opts := RestoreOptions{
		AuthCopyTables: []string{
			"auth.users",
			"AUTH.SSO_DOMAINS",
			"public.users", // should be ignored
		},
	}

	tables, err := resolveAllowedAuthCopyTables(opts)
	if err != nil {
		t.Fatalf("resolveAllowedAuthCopyTables() error = %v", err)
	}

	if !tables["auth.users"] {
		t.Fatalf("expected auth.users to be present in allowed auth tables")
	}
	if !tables["auth.sso_domains"] {
		t.Fatalf("expected auth.sso_domains to be present in allowed auth tables")
	}
	if tables["public.users"] {
		t.Fatalf("public.users should not appear in auth table allowlist")
	}
}

func hasArgPair(args []string, key, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == key && args[i+1] == value {
			return true
		}
	}
	return false
}

func hasFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}
