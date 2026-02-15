package cmd

import "testing"

func TestParseMigrationListRows(t *testing.T) {
	output := `
Local          | Remote         | Time (UTC)
----------------|----------------|---------------------
 20260215035000 | 20260215035000 | 2026-02-15 03:50:00
 20260215062000 |                | 2026-02-15 06:20:00
`

	rows := parseMigrationListRows(output)
	if len(rows) != 2 {
		t.Fatalf("parseMigrationListRows() rows = %d, want 2", len(rows))
	}

	if rows[0].Local != "20260215035000" || rows[0].Remote != "20260215035000" {
		t.Fatalf("row[0] = %+v, want local/remote 20260215035000", rows[0])
	}

	if rows[1].Local != "20260215062000" || rows[1].Remote != "" {
		t.Fatalf("row[1] = %+v, want local 20260215062000 and empty remote", rows[1])
	}
}

func TestBuildMigrationFilenameIndex(t *testing.T) {
	localMigrations := []string{
		"20260215035000_add_profiles.sql",
		"20260215062000_add_preferences.sql",
	}

	index := buildMigrationFilenameIndex(localMigrations)

	if got := index["20260215035000"]; got != "20260215035000_add_profiles.sql" {
		t.Fatalf("index[20260215035000] = %q, want %q", got, "20260215035000_add_profiles.sql")
	}

	if got := index["20260215062000"]; got != "20260215062000_add_preferences.sql" {
		t.Fatalf("index[20260215062000] = %q, want %q", got, "20260215062000_add_preferences.sql")
	}
}

func TestMigrationFileForRow(t *testing.T) {
	filenameByTimestamp := map[string]string{
		"20260215035000": "20260215035000_add_profiles.sql",
	}

	tests := []struct {
		name string
		row  migrationListRow
		want string
	}{
		{
			name: "local timestamp match",
			row: migrationListRow{
				Local:  "20260215035000",
				Remote: "20260215035000",
			},
			want: "20260215035000_add_profiles.sql",
		},
		{
			name: "remote timestamp fallback",
			row: migrationListRow{
				Local:  "",
				Remote: "20260215035000",
			},
			want: "20260215035000_add_profiles.sql",
		},
		{
			name: "unknown timestamp",
			row: migrationListRow{
				Local:  "20260215100000",
				Remote: "",
			},
			want: "-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := migrationFileForRow(tt.row, filenameByTimestamp)
			if got != tt.want {
				t.Fatalf("migrationFileForRow() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMigrationTimestampFromFilename(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{
			filename: "20260215035000_add_profiles.sql",
			want:     "20260215035000",
		},
		{
			filename: "20260215035000.sql",
			want:     "20260215035000",
		},
		{
			filename: "custom_name.sql",
			want:     "custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := migrationTimestampFromFilename(tt.filename)
			if got != tt.want {
				t.Fatalf("migrationTimestampFromFilename(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}
