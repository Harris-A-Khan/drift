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

func TestRestoreOptionsFromDBURL(t *testing.T) {
	tests := []struct {
		name    string
		dbURL   string
		wantErr bool
		want    struct {
			host     string
			port     int
			user     string
			password string
			database string
		}
	}{
		{
			name:  "full URL",
			dbURL: "postgresql://postgres.project:secret@aws-1-us-east-2.pooler.supabase.com:5432/postgres?connect_timeout=10",
			want: struct {
				host     string
				port     int
				user     string
				password string
				database string
			}{
				host:     "aws-1-us-east-2.pooler.supabase.com",
				port:     5432,
				user:     "postgres.project",
				password: "secret",
				database: "postgres",
			},
		},
		{
			name:  "defaults database and port",
			dbURL: "postgresql://postgres.project:secret@aws-1-us-east-2.pooler.supabase.com",
			want: struct {
				host     string
				port     int
				user     string
				password string
				database string
			}{
				host:     "aws-1-us-east-2.pooler.supabase.com",
				port:     5432,
				user:     "postgres.project",
				password: "secret",
				database: "postgres",
			},
		},
		{
			name:    "invalid URL",
			dbURL:   "://bad-url",
			wantErr: true,
		},
		{
			name:    "missing host",
			dbURL:   "postgresql:///postgres",
			wantErr: true,
		},
		{
			name:    "invalid port",
			dbURL:   "postgresql://postgres:secret@localhost:abc/postgres",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := restoreOptionsFromDBURL(tt.dbURL)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("restoreOptionsFromDBURL(%q) expected error", tt.dbURL)
				}
				return
			}

			if err != nil {
				t.Fatalf("restoreOptionsFromDBURL(%q) error = %v", tt.dbURL, err)
			}

			if got.Host != tt.want.host {
				t.Fatalf("Host = %q, want %q", got.Host, tt.want.host)
			}
			if got.Port != tt.want.port {
				t.Fatalf("Port = %d, want %d", got.Port, tt.want.port)
			}
			if got.User != tt.want.user {
				t.Fatalf("User = %q, want %q", got.User, tt.want.user)
			}
			if got.Password != tt.want.password {
				t.Fatalf("Password = %q, want %q", got.Password, tt.want.password)
			}
			if got.Database != tt.want.database {
				t.Fatalf("Database = %q, want %q", got.Database, tt.want.database)
			}
		})
	}
}
