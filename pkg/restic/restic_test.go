package restic

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	v1 "github.com/garethgeorge/resticui/gen/go/v1"
	test "github.com/garethgeorge/resticui/internal/test/helpers"
)

func TestResticInit(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()

	r := NewRepo(&v1.Repo{
		Id:       "test",
		Uri:      repo,
		Password: "test",
	}, WithFlags("--no-cache"))

	if err := r.Init(context.Background()); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}
}

func TestResticBackup(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()

	// create a new repo with cache disabled for testing
	r := NewRepo(&v1.Repo{
		Id:       "test",
		Uri:      repo,
		Password: "test",
	}, WithFlags("--no-cache"))
	if err := r.Init(context.Background()); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	testData := test.CreateTestData(t)
	testData2 := test.CreateTestData(t)

	var tests = []struct {
		name    string
		opts    []BackupOption
		files   int // expected files at the end of the backup
		wantErr bool
	}{
		{
			name:  "no options",
			opts:  []BackupOption{WithBackupPaths(testData)},
			files: 100,
		},
		{
			name:  "with two paths",
			opts:  []BackupOption{WithBackupPaths(testData), WithBackupPaths(testData2)},
			files: 200,
		},
		{
			name:  "with exclude",
			opts:  []BackupOption{WithBackupPaths(testData), WithBackupExcludes("file1*")},
			files: 90,
		},
		{
			name:  "with exclude pattern",
			opts:  []BackupOption{WithBackupPaths(testData), WithBackupExcludes("file*")},
			files: 0,
		},
		{
			name:    "with nothing to backup",
			opts:    []BackupOption{},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			summary, err := r.Backup(context.Background(), func(event *BackupProgressEntry) {
				t.Logf("backup event: %v", event)
			}, tc.opts...)
			if (err != nil) != tc.wantErr {
				t.Fatalf("wanted error: %v, got: %v", tc.wantErr, err)
			}

			if tc.wantErr {
				return
			}

			if summary == nil {
				t.Fatalf("wanted summary, got: nil")
			}

			if summary.TotalFilesProcessed != tc.files {
				t.Errorf("wanted %d files, got: %d", tc.files, summary.TotalFilesProcessed)
			}
		})
	}
}

func TestSnapshot(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()

	r := NewRepo(&v1.Repo{
		Id:       "test",
		Uri:      repo,
		Password: "test",
	}, WithFlags("--no-cache"))
	if err := r.Init(context.Background()); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	testData := test.CreateTestData(t)

	for i := 0; i < 10; i++ {
		_, err := r.Backup(context.Background(), nil, WithBackupPaths(testData), WithBackupTags(fmt.Sprintf("tag%d", i)))
		if err != nil {
			t.Fatalf("failed to backup and create new snapshot: %v", err)
		}
	}

	var tests = []struct {
		name  string
		opts  []GenericOption
		count int
	}{
		{
			name:  "no options",
			opts:  []GenericOption{},
			count: 10,
		},
		{
			name:  "with tag",
			opts:  []GenericOption{WithTags("tag1")},
			count: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			snapshots, err := r.Snapshots(context.Background(), tc.opts...)
			if err != nil {
				t.Fatalf("failed to list snapshots: %v", err)
			}

			if len(snapshots) != tc.count {
				t.Errorf("wanted %d snapshots, got: %d", tc.count, len(snapshots))
			}

			// Ensure that snapshot timestamps are set, this is critical for correct ordering in the orchestrator.
			for _, snapshot := range snapshots {
				if snapshot.UnixTimeMs() == 0 {
					t.Errorf("wanted snapshot time to be non-zero, got: %v", snapshot.UnixTimeMs())
				}
			}
		})
	}
}

func TestLs(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	r := NewRepo(&v1.Repo{
		Id:       "test",
		Uri:      repo,
		Password: "test",
	}, WithFlags("--no-cache"))
	if err := r.Init(context.Background()); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	testData := test.CreateTestData(t)

	snapshot, err := r.Backup(context.Background(), nil, WithBackupPaths(testData))
	if err != nil {
		t.Fatalf("failed to backup and create new snapshot: %v", err)
	}

	_, entries, err := r.ListDirectory(context.Background(), snapshot.SnapshotId, testData)

	if err != nil {
		t.Fatalf("failed to list directory: %v", err)
	}

	if len(entries) != 101 {
		t.Errorf("wanted 101 entries, got: %d", len(entries))
	}
}

func TestResticForget(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	r := NewRepo(&v1.Repo{
		Id:       "test",
		Uri:      repo,
		Password: "test",
	}, WithFlags("--no-cache"))
	if err := r.Init(context.Background()); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	testData := test.CreateTestData(t)

	ids := make([]string, 0)
	for i := 0; i < 10; i++ {
		output, err := r.Backup(context.Background(), nil, WithBackupPaths(testData))
		if err != nil {
			t.Fatalf("failed to backup and create new snapshot: %v", err)
		}

		ids = append(ids, output.SnapshotId)
	}

	// prune all snapshots
	output := bytes.NewBuffer(nil)
	res, err := r.Forget(context.Background(), RetentionPolicy{KeepLastN: 3}, output)
	if err != nil {
		t.Fatalf("failed to prune snapshots: %v", err)
	}

	if len(res.Keep) != 3 {
		t.Errorf("wanted 3 snapshots to be kept, got: %d", len(res.Keep))
	}

	if len(res.Remove) != 7 {
		t.Errorf("wanted 7 snapshots to be removed, got: %d", len(res.Remove))
	}

	removedIds := make([]string, 0)
	for _, snapshot := range res.Remove {
		removedIds = append(removedIds, snapshot.Id)
	}
	slices.Reverse(removedIds)
	keptIds := make([]string, 0)
	for _, snapshot := range res.Keep {
		keptIds = append(keptIds, snapshot.Id)
	}
	slices.Reverse(keptIds)

	if !reflect.DeepEqual(removedIds, ids[:7]) {
		t.Errorf("wanted removed ids to be %v, got: %v", ids[:7], removedIds)
	}

	if !reflect.DeepEqual(keptIds, ids[7:]) {
		t.Errorf("wanted kept ids to be %v, got: %v", ids[7:], keptIds)
	}

	if !strings.Contains(output.String(), "total prune") {
		t.Errorf("wanted prune output, got: %s", output.String())
	}
}
