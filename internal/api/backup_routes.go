package api

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/danielgtaylor/huma/v2"
)

const backupMediaType = "application/gzip"

type backupOutput struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	CacheControl       string `header:"Cache-Control"`
	ContentLength      int64  `header:"Content-Length"`
	Body               func(huma.Context)
}

func (s *server) registerBackup(a huma.API) {
	operation := op("getSystemBackup", http.MethodGet, "/api/v1/system/backup", "Download router configuration backup", "system", 500, 503)
	operation.Responses = map[string]*huma.Response{
		"200": {Content: map[string]*huma.MediaType{
			backupMediaType: {Schema: &huma.Schema{Type: "string", Format: "binary"}},
		}},
	}
	operation.Metadata = map[string]any{adminOnlyKey: true} // holds WiFi keys and the shadow file
	register(s, false, operation, a, s.getSystemBackup)
}

func (s *server) getSystemBackup(ctx context.Context, _ *struct{}) (*backupOutput, error) {
	if s.Backup == nil {
		return nil, fail(http.StatusServiceUnavailable, "backup_unavailable", "Configuration backup is unavailable")
	}
	backupFailed := fail(http.StatusInternalServerError, "backup_failed", "Could not create configuration backup")
	dir, err := os.MkdirTemp("/tmp", "travo-backup-")
	if err != nil {
		return nil, backupFailed
	}
	// Once opened, the archive can be streamed from its descriptor after unlinking.
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "config.tar.gz")
	if err := s.Backup.BackupConfig(ctx, path); err != nil || ctx.Err() != nil {
		return nil, backupFailed
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, backupFailed
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		file.Close()
		return nil, backupFailed
	}
	return &backupOutput{
		ContentType:        backupMediaType,
		ContentDisposition: `attachment; filename="travo-config.tar.gz"`,
		CacheControl:       "no-store",
		ContentLength:      info.Size(),
		Body: func(ctx huma.Context) {
			defer file.Close()
			_, _ = io.Copy(ctx.BodyWriter(), file)
		},
	}, nil
}
