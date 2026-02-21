package handler

import (
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/naratel/naratel-box/backend/internal/auth"
	"github.com/naratel/naratel-box/backend/internal/block"
	"github.com/naratel/naratel-box/backend/internal/logger"
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
// @Description  Upload a file using multipart/form-data. Optionally specify folder_id form field.
// @Tags         files
// @Accept       mpfd
// @Produce      json
// @Param        file      formData file   true  "File to upload"
// @Param        folder_id formData int    false "Target folder ID"
// @Success      201  {object} UploadResponse
// @Failure      400  {object} ErrorResponse
// @Failure      401  {object} ErrorResponse
// @Failure      500  {object} ErrorResponse
// @Security     BearerAuth
// @Router       /files [post]
func (h *UploadHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		logger.Warn(r.Context(), "Unauthorized upload attempt", nil)
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	// 256MB in RAM; larger files spill to /tmp on disk to avoid OOMKill (pod limit: 512Mi)
	if err := r.ParseMultipartForm(256 << 20); err != nil {
		logger.Warn(r.Context(), "Failed to parse multipart form", map[string]interface{}{
			"user_id": userID, "error": err.Error(),
		})
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Message: "failed to parse multipart form: " + err.Error(),
		})
		return
	}
	defer r.MultipartForm.RemoveAll()

	f, fileHeader, err := r.FormFile("file")
	if err != nil {
		logger.Warn(r.Context(), "Missing file field in upload", map[string]interface{}{"user_id": userID})
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Message: "field 'file' is required",
		})
		return
	}
	defer f.Close()

	// Parse optional folder_id
	var folderID *int64
	if fid := r.FormValue("folder_id"); fid != "" {
		parsed, err := strconv.ParseInt(fid, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "invalid folder_id"})
			return
		}
		folderID = &parsed
	}

	mimeType := mime.TypeByExtension(filepath.Ext(fileHeader.Filename))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	logger.Info(r.Context(), "File upload started", map[string]interface{}{
		"user_id":   userID,
		"file_name": fileHeader.Filename,
		"mime_type": mimeType,
		"file_size": fileHeader.Size,
	})

	ctx, ctxCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer ctxCancel()

	// Propagate request context values to the new context
	ctx = logger.WithRequestID(ctx, logger.GetRequestID(r.Context()))
	ctx = logger.WithMethod(ctx, logger.GetMethod(r.Context()))
	ctx = logger.WithPath(ctx, logger.GetPath(r.Context()))

	blockIDs, totalBytes, err := h.processor.Process(ctx, f)
	if err != nil {
		logger.ErrorLog(r.Context(), "File upload block processing failed", logger.ErrorDetails{
			Code: "UPLOAD_PROCESS_ERR", Details: err.Error(),
		})
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "upload_failed",
			Message: err.Error(),
		})
		return
	}

	file, err := h.fileRepo.Create(ctx, userID, fileHeader.Filename, mimeType, totalBytes, folderID)
	if err != nil {
		logger.ErrorLog(r.Context(), "Failed to save file metadata", logger.ErrorDetails{
			Code: "DB_ERR", Details: err.Error(),
		})
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "db_error",
			Message: "failed to save file metadata",
		})
		return
	}

	if err := h.fileRepo.LinkBlocks(ctx, file.ID, blockIDs); err != nil {
		logger.ErrorLog(r.Context(), "Failed to link blocks to file", logger.ErrorDetails{
			Code: "DB_ERR", Details: err.Error(),
		})
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "db_error",
			Message: "failed to link blocks",
		})
		return
	}

	logger.Info(r.Context(), "File uploaded successfully", map[string]interface{}{
		"user_id":     userID,
		"file_id":     file.ID,
		"file_name":   file.Name,
		"total_size":  totalBytes,
		"blocks_count": len(blockIDs),
	})

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
// @Description  Returns files in a folder (or root). Use ?folder_id=N or omit for root. Use ?search=term to search.
// @Tags         files
// @Produce      json
// @Param        folder_id query int    false "Folder ID (omit for root)"
// @Param        search    query string false "Search query"
// @Success      200  {object} FolderContentsResponse
// @Failure      401  {object} ErrorResponse
// @Failure      500  {object} ErrorResponse
// @Security     BearerAuth
// @Router       /files [get]
func (h *UploadHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	// Search mode
	if q := r.URL.Query().Get("search"); q != "" {
		logger.Info(r.Context(), "File search initiated", map[string]interface{}{
			"user_id": userID, "search_query": q,
		})
		files, err := h.fileRepo.Search(r.Context(), userID, q)
		if err != nil {
			logger.ErrorLog(r.Context(), "File search failed", logger.ErrorDetails{
				Code: "DB_ERR", Details: err.Error(),
			})
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "db_error", Message: "search failed"})
			return
		}
		if files == nil {
			files = []*model.File{}
		}
		writeJSON(w, http.StatusOK, FolderContentsResponse{
			Files:   files,
			Folders: []*model.Folder{},
		})
		return
	}

	// Folder listing mode
	var folderID *int64
	if fid := r.URL.Query().Get("folder_id"); fid != "" {
		parsed, err := strconv.ParseInt(fid, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "invalid folder_id"})
			return
		}
		folderID = &parsed
	}

	files, err := h.fileRepo.ListByFolder(r.Context(), userID, folderID)
	if err != nil {
		logger.ErrorLog(r.Context(), "Failed to list files", logger.ErrorDetails{
			Code: "DB_ERR", Details: err.Error(),
		})
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
// @Description  Returns metadata for a single file
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

// RenameRequest is the payload for PATCH /files/{id}/rename.
type RenameRequest struct {
	Name string `json:"name"`
}

// RenameFile godoc
// @Summary      Rename a file
// @Tags         files
// @Accept       json
// @Produce      json
// @Param        id   path     int           true "File ID"
// @Param        body body     RenameRequest true "New name"
// @Success      200  {object} model.File
// @Security     BearerAuth
// @Router       /files/{id}/rename [patch]
func (h *UploadHandler) RenameFile(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	fileID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "invalid file id"})
		return
	}

	var req RenameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "name is required"})
		return
	}

	file, err := h.fileRepo.Rename(r.Context(), fileID, userID, req.Name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "file not found"})
		return
	}

	writeJSON(w, http.StatusOK, file)
}

// MoveRequest is the payload for PATCH /files/{id}/move.
type MoveRequest struct {
	FolderID *int64 `json:"folder_id"` // null = move to root
}

// MoveFile godoc
// @Summary      Move a file to a different folder
// @Tags         files
// @Accept       json
// @Produce      json
// @Param        id   path     int         true "File ID"
// @Param        body body     MoveRequest true "Target folder"
// @Success      200  {object} model.File
// @Security     BearerAuth
// @Router       /files/{id}/move [patch]
func (h *UploadHandler) MoveFile(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	fileID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "invalid file id"})
		return
	}

	var req MoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "invalid JSON body"})
		return
	}

	file, err := h.fileRepo.Move(r.Context(), fileID, userID, req.FolderID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "file not found"})
		return
	}

	writeJSON(w, http.StatusOK, file)
}

// FolderContentsResponse wraps files and subfolders for a directory listing.
type FolderContentsResponse struct {
	Folders []*model.Folder `json:"folders"`
	Files   []*model.File   `json:"files"`
}
