package handler

import (
	"context"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/naratel/naratel-box/backend/internal/auth"
	"github.com/naratel/naratel-box/backend/internal/block"
	"github.com/naratel/naratel-box/backend/internal/model"
	"github.com/naratel/naratel-box/backend/internal/repository"
)

// UploadResponse is returned on a successful file upload.
type UploadResponse struct {
	FileID      int64  `json:"file_id"      example:"42"`
	Name        string `json:"name"         example:"report.pdf"`
	MimeType    string `json:"mime_type"    example:"application/pdf"`
	Size        int64  `json:"size"         example:"8388608"`
	BlocksCount int    `json:"blocks_count" example:"3"`
	CreatedAt   string `json:"created_at"   example:"2026-02-18T12:00:00Z"`
}

type UploadHandler struct {
	fileRepo  *repository.FileRepository
	processor *block.Processor
}

func NewUploadHandler(fileRepo *repository.FileRepository, processor *block.Processor) *UploadHandler {
	return &UploadHandler{
		fileRepo:  fileRepo,
		processor: processor,
	}
}

// Upload godoc
// @Summary      Upload a file
// @Description  Upload a file using multipart/form-data. The backend splits it into 8MB blocks, deduplicates, and stores in S3.
// @Tags         files
// @Accept       mpfd
// @Produce      json
// @Param        file formData file                  true "File to upload"
// @Success      201  {object} UploadResponse
// @Failure      400  {object} ErrorResponse "bad_request"
// @Failure      401  {object} ErrorResponse "unauthorized"
// @Failure      500  {object} ErrorResponse "upload_failed"
// @Security     BearerAuth
// @Router       /files [post]
func (h *UploadHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	// Parse multipart form — 512 MB max memory; excess goes to temp files.
	// net/http handles this natively without the issues Fiber/fasthttp had.
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Message: "failed to parse multipart form: " + err.Error(),
		})
		return
	}
	defer r.MultipartForm.RemoveAll()

	f, fileHeader, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Message: "field 'file' is required",
		})
		return
	}
	defer f.Close()

	// Detect MIME type from extension (fallback to octet-stream)
	mimeType := mime.TypeByExtension(filepath.Ext(fileHeader.Filename))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Use a long-lived context for S3 uploads + DB operations.
	ctx, ctxCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer ctxCancel()

	// Process: split → hash → dedup → upload blocks
	blockIDs, totalBytes, err := h.processor.Process(ctx, f)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "upload_failed",
			Message: err.Error(),
		})
		return
	}

	// Create file metadata record
	file, err := h.fileRepo.Create(ctx, userID, fileHeader.Filename, mimeType, totalBytes)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "db_error",
			Message: "failed to save file metadata",
		})
		return
	}

	// Link blocks to file
	if err := h.fileRepo.LinkBlocks(ctx, file.ID, blockIDs); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "db_error",
			Message: "failed to link blocks",
		})
		return
	}

	writeJSON(w, http.StatusCreated, UploadResponse{
		FileID:      file.ID,
		Name:        file.Name,
		MimeType:    file.MimeType,
		Size:        file.TotalSize,
		BlocksCount: len(blockIDs),
		CreatedAt:   file.CreatedAt.Format(time.RFC3339),
	})
}

// ListFiles godoc
// @Summary      List files
// @Description  Returns all files owned by the authenticated user
// @Tags         files
// @Produce      json
// @Success      200  {array}  model.File
// @Failure      401  {object} ErrorResponse "unauthorized"
// @Failure      500  {object} ErrorResponse "db_error"
// @Security     BearerAuth
// @Router       /files [get]
func (h *UploadHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	files, err := h.fileRepo.ListByUserID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "db_error", Message: "failed to list files"})
		return
	}
	if files == nil {
		files = []*model.File{}
	}
	writeJSON(w, http.StatusOK, files)
}

// FileInfo godoc
// @Summary      Get file metadata
// @Description  Returns metadata for a single file without downloading it
// @Tags         files
// @Produce      json
// @Param        id  path     int true "File ID"
// @Success      200 {object} model.File
// @Failure      400 {object} ErrorResponse
// @Failure      401 {object} ErrorResponse
// @Failure      403 {object} ErrorResponse
// @Security     BearerAuth
// @Router       /files/{id}/info [get]
func (h *UploadHandler) FileInfo(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: "missing token"})
		return
	}

	fileID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "invalid file id"})
		return
	}

	file, err := h.fileRepo.FindByIDAndUserID(r.Context(), fileID, userID)
	if err != nil {
		writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "file not found or unauthorized"})
		return
	}

	writeJSON(w, http.StatusOK, file)
}
