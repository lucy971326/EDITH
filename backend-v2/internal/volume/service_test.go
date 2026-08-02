package volume

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eric642/e2b-go-sdk"
	_ "github.com/mattn/go-sqlite3"
)

func TestReadUserOverviewWithoutVolumeReturnsEmpty(t *testing.T) {
	db := openVolumeTestDB(t, "overview-empty")
	service := &Service{store: &store{db: db}}

	overview, err := service.ReadUserOverview(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ReadUserOverview() error = %v", err)
	}
	if overview != "" {
		t.Fatalf("overview = %q, want empty", overview)
	}
}

func TestReadUserOverviewReadsExistingVolume(t *testing.T) {
	server := newVolumeContentTestServer(t, http.StatusOK, []byte("# 用户 Skills\n- daily-summary"))
	db := openVolumeTestDB(t, "overview-read")
	store := &store{db: db}
	if err := store.save(context.Background(), record{UserID: "user-1", ID: "vol-1", Name: "vol-1", Token: "token-1"}); err != nil {
		t.Fatal(err)
	}
	service := &Service{store: store, config: e2b.Config{APIURL: server.URL, APIKey: "test-key"}}

	overview, err := service.ReadUserOverview(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ReadUserOverview() error = %v", err)
	}
	if overview != "# 用户 Skills\n- daily-summary" {
		t.Fatalf("overview = %q", overview)
	}
}

func TestReadUserOverviewMissingFileReturnsEmpty(t *testing.T) {
	server := newVolumeContentTestServer(t, http.StatusNotFound, nil)
	db := openVolumeTestDB(t, "overview-missing")
	store := &store{db: db}
	if err := store.save(context.Background(), record{UserID: "user-1", ID: "vol-1", Name: "vol-1", Token: "token-1"}); err != nil {
		t.Fatal(err)
	}
	service := &Service{store: store, config: e2b.Config{APIURL: server.URL, APIKey: "test-key"}}

	overview, err := service.ReadUserOverview(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ReadUserOverview() error = %v", err)
	}
	if overview != "" {
		t.Fatalf("overview = %q, want empty", overview)
	}
}

func TestReadUserOverviewPropagatesRemoteError(t *testing.T) {
	server := newVolumeContentTestServer(t, http.StatusInternalServerError, []byte("failed"))
	db := openVolumeTestDB(t, "overview-error")
	store := &store{db: db}
	if err := store.save(context.Background(), record{UserID: "user-1", ID: "vol-1", Name: "vol-1", Token: "token-1"}); err != nil {
		t.Fatal(err)
	}
	service := &Service{store: store, config: e2b.Config{APIURL: server.URL, APIKey: "test-key"}}

	if _, err := service.ReadUserOverview(context.Background(), "user-1"); err == nil {
		t.Fatal("ReadUserOverview() accepted a remote error")
	}
}

func openVolumeTestDB(t *testing.T, name string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+name+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if err := createSchema(context.Background(), db); err != nil {
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newVolumeContentTestServer(t *testing.T, fileStatus int, body []byte) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/volumes/vol-1":
			writer.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(writer).Encode(map[string]string{"volumeID": "vol-1", "name": "vol-1"})
		case "/volumecontent/vol-1/file":
			if request.URL.Query().Get("path") != UserOverviewPath {
				t.Errorf("overview path = %q, want %q", request.URL.Query().Get("path"), UserOverviewPath)
			}
			writer.WriteHeader(fileStatus)
			_, _ = writer.Write(body)
		default:
			t.Errorf("unexpected request path: %s", request.URL.Path)
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server
}
