package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/blackdark-org/travo-greenfield/internal/firmware"
)

// Firmware stages and flashes a sysupgrade image (internal/firmware).
type Firmware interface {
	Prepare(ctx context.Context, size int64) error
	Upload(ctx context.Context, src io.Reader) (firmware.Image, error)
	Status(ctx context.Context) (firmware.Status, error)
	Apply(ctx context.Context, sha string, keepSettings bool, phrase string) error
}

// firmwareFormSlack covers the multipart boundary and part headers around the image.
const firmwareFormSlack = 64 << 10

type firmwareUploadInput struct {
	RawBody huma.MultipartFormFiles[struct {
		Image huma.FormFile `form:"image" contentType:"application/octet-stream" required:"true" doc:"OpenWrt sysupgrade image"`
	}]
}

type firmwareImageOutput struct{ Body firmware.Image }

type firmwareStatusOutput struct{ Body firmware.Status }

type firmwareApplyInput struct {
	Body struct {
		SHA256       string `json:"sha256" pattern:"^[0-9a-f]{64}$" doc:"Checksum of the staged image"`
		KeepSettings bool   `json:"keep_settings" doc:"Keep configuration (sysupgrade without -n)"`
		Confirm      string `json:"confirm" maxLength:"64" doc:"Must be exactly \"upgrade firmware\", typed by the operator"`
	}
}

func (s *server) registerFirmware(a huma.API) {
	// Flashing the router is never something an API token may do; the image path stays private too.
	registerSessionOnly(s, op("getFirmware", http.MethodGet, "/api/v1/system/firmware", "Staged firmware image and room for an upload", "system", 500, 503), a, s.firmwareStatus)
	up := op("postFirmware", http.MethodPost, "/api/v1/system/firmware", "Upload and validate a sysupgrade image (never flashes)", "system", 400, 409, 411, 413, 500, 503, 507)
	up.MaxBodyBytes = firmware.MaxImageBytes + firmwareFormSlack
	up.BodyReadTimeout = 10 * time.Minute
	up.Middlewares = huma.Middlewares{s.firmwarePrepare}
	registerSessionOnly(s, up, a, s.firmwareUpload)
	apply := op("postFirmwareApply", http.MethodPost, "/api/v1/system/firmware/apply", "Flash the staged image and restart (sysupgrade)", "system", 400, 409, 500, 503)
	apply.DefaultStatus = http.StatusAccepted
	apply.MaxBodyBytes = 1024
	registerSessionOnly(s, apply, a, s.firmwareApply)
}

// firmwarePrepare runs after the auth gate and before the body is read, so an image that
// cannot fit is refused before it lands in /tmp.
func (s *server) firmwarePrepare(ctx huma.Context, next func(huma.Context)) {
	if s.Firmware == nil {
		writeError(ctx, http.StatusServiceUnavailable, "firmware_unavailable", "firmware upgrade is unavailable")
		return
	}
	r, _ := humago.Unwrap(ctx)
	size := r.ContentLength
	if size < 0 {
		writeError(ctx, http.StatusLengthRequired, "length_required", "the upload needs a Content-Length")
		return
	}
	if size > firmware.MaxImageBytes+firmwareFormSlack {
		writeError(ctx, http.StatusRequestEntityTooLarge, "firmware_too_large", "the image is larger than 64 MiB")
		return
	}
	if err := s.Firmware.Prepare(ctx.Context(), size); err != nil {
		e := firmwareError(err)
		var se *apiError
		if errors.As(e, &se) {
			writeError(ctx, se.Status, se.Code, se.Message)
			return
		}
		writeError(ctx, http.StatusInternalServerError, "firmware_failed", "could not prepare the upload")
		return
	}
	next(ctx)
}

func (s *server) firmwareStatus(ctx context.Context, _ *struct{}) (*firmwareStatusOutput, error) {
	if s.Firmware == nil {
		return nil, fail(http.StatusServiceUnavailable, "firmware_unavailable", "firmware upgrade is unavailable")
	}
	st, err := s.Firmware.Status(ctx)
	if err != nil {
		return nil, firmwareError(err)
	}
	return &firmwareStatusOutput{Body: st}, nil
}

func (s *server) firmwareUpload(ctx context.Context, in *firmwareUploadInput) (*firmwareImageOutput, error) {
	if s.Firmware == nil {
		return nil, fail(http.StatusServiceUnavailable, "firmware_unavailable", "firmware upgrade is unavailable")
	}
	f := in.RawBody.Data().Image
	img, err := s.Firmware.Upload(ctx, f.File)
	if err != nil {
		return nil, firmwareError(err)
	}
	return &firmwareImageOutput{Body: img}, nil
}

func (s *server) firmwareApply(ctx context.Context, in *firmwareApplyInput) (*struct{ Body okBody }, error) {
	if s.Firmware == nil {
		return nil, fail(http.StatusServiceUnavailable, "firmware_unavailable", "firmware upgrade is unavailable")
	}
	if err := s.Firmware.Apply(ctx, in.Body.SHA256, in.Body.KeepSettings, in.Body.Confirm); err != nil {
		return nil, firmwareError(err)
	}
	out := &struct{ Body okBody }{}
	out.Body.Status = "ok"
	return out, nil
}

func firmwareError(err error) error {
	for _, m := range []struct {
		err    error
		status int
		code   string
		msg    string
	}{
		{firmware.ErrConfirm, http.StatusBadRequest, "confirmation_required", `type "upgrade firmware" to confirm`},
		{firmware.ErrTooLarge, http.StatusRequestEntityTooLarge, "firmware_too_large", "the image is larger than 64 MiB"},
		{firmware.ErrNoRoom, http.StatusInsufficientStorage, "firmware_no_room", "not enough free memory to hold the image"},
		{firmware.ErrPending, http.StatusConflict, "firmware_upgrade_pending", "a firmware upgrade was already started"},
		{firmware.ErrBusy, http.StatusConflict, "operation_in_progress", "another change, restart or update is in progress"},
		{firmware.ErrNotStaged, http.StatusConflict, "firmware_not_staged", "upload and validate the image again"},
		{firmware.ErrInvalid, http.StatusConflict, "firmware_invalid", "the image failed validation"},
		{firmware.ErrNoBackup, http.StatusConflict, "firmware_no_backup", "settings cannot be kept with this image"},
		{firmware.ErrUnavailable, http.StatusServiceUnavailable, "firmware_unavailable", "firmware upgrade is unavailable"},
	} {
		if errors.Is(err, m.err) {
			return fail(m.status, m.code, m.msg)
		}
	}
	slog.Warn("firmware", "err", err)
	return fail(http.StatusInternalServerError, "firmware_failed", "the firmware step failed")
}
