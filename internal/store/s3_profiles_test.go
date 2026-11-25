package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestS3Profiles_EndpointRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if _, err := s.UpsertS3Profile(ctx, S3Profile{Name: "aws", Bucket: "b1", Region: ptrStr("ap-northeast-2")}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertS3Profile(ctx, S3Profile{
		Name: "minio", Bucket: "b2", Endpoint: ptrStr("http://localhost:9000"), PathStyle: true,
	}); err != nil {
		t.Fatal(err)
	}

	list, err := s.ListS3Profiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("got %d profiles, want 2", len(list))
	}
	aws, minio := list[0], list[1]
	if aws.Endpoint != nil || aws.PathStyle {
		t.Errorf("aws profile endpoint=%v path_style=%v, want unset", aws.Endpoint, aws.PathStyle)
	}
	if minio.Endpoint == nil || *minio.Endpoint != "http://localhost:9000" || !minio.PathStyle {
		t.Errorf("minio profile endpoint=%v path_style=%v", minio.Endpoint, minio.PathStyle)
	}

	// Updating by name can switch a profile back to AWS.
	if _, err := s.UpsertS3Profile(ctx, S3Profile{Name: "minio", Bucket: "b2"}); err != nil {
		t.Fatal(err)
	}
	list, _ = s.ListS3Profiles(ctx)
	if list[1].Endpoint != nil || list[1].PathStyle {
		t.Errorf("after update endpoint=%v path_style=%v, want cleared", list[1].Endpoint, list[1].PathStyle)
	}
}

func TestRestoreMetadata_S3ProfileEndpoint(t *testing.T) {
	ctx := context.Background()
	src := openTestStore(t)
	if _, err := src.UpsertS3Profile(ctx, S3Profile{
		Name: "r2", Bucket: "mail", Endpoint: ptrStr("https://acct.r2.cloudflarestorage.com"), PathStyle: true,
	}); err != nil {
		t.Fatal(err)
	}
	snap := filepath.Join(t.TempDir(), "snap.db")
	if err := src.SnapshotTo(ctx, snap); err != nil {
		t.Fatal(err)
	}

	dst := openTestStore(t)
	if _, err := dst.RestoreMetadata(ctx, snap); err != nil {
		t.Fatal(err)
	}
	list, err := dst.ListS3Profiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Endpoint == nil || *list[0].Endpoint != "https://acct.r2.cloudflarestorage.com" || !list[0].PathStyle {
		t.Fatalf("restored %+v, want endpoint and path style kept", list)
	}
}

// A snapshot taken before the endpoint columns existed still restores.
func TestRestoreMetadata_S3ProfileFromOlderSnapshot(t *testing.T) {
	ctx := context.Background()
	src := openTestStore(t)
	if _, err := src.UpsertS3Profile(ctx, S3Profile{Name: "old", Bucket: "b"}); err != nil {
		t.Fatal(err)
	}
	snap := filepath.Join(t.TempDir(), "snap.db")
	if err := src.SnapshotTo(ctx, snap); err != nil {
		t.Fatal(err)
	}
	old, err := Open(ctx, snap)
	if err != nil {
		t.Fatal(err)
	}
	for _, col := range []string{"endpoint", "path_style"} {
		if _, err := old.DB.ExecContext(ctx, "ALTER TABLE s3_profiles DROP COLUMN "+col); err != nil {
			t.Fatal(err)
		}
	}
	old.Close()

	dst := openTestStore(t)
	sum, err := dst.RestoreMetadata(ctx, snap)
	if err != nil {
		t.Fatal(err)
	}
	if sum.S3Profiles != 1 {
		t.Fatalf("summary=%+v, want 1 s3 profile", sum)
	}
}
