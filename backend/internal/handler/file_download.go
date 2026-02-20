package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/naratel/naratel-box/backend/internal/auth"
	"github.com/naratel/naratel-box/backend/internal/block"
	"github.com/naratel/naratel-box/backend/internal/repository"
	"github.com/naratel/naratel-box/backend/internal/storage"
)

type DownloadHandler struct {
	fileRepo  *repository.FileRepository
	blockRepo *repository.BlockRepository
	s3        *storage.S3Client
}

func NewDownloadHandler(
	fileRepo *repository.FileRepository,
	blockRepo *repository.BlockRepository,
	s3 *storage.S3Client,
) *DownloadHandler {
	return &DownloadHandler{
		fileRepo:  fileRepo,
		blockRepo: blockRepo,
		s3:        s3,
	}
}

// Download godoc
// @Summary      Download a file
// @Description  Stream a file by ID. Returns 403 if the file does not belong to the authenticated user.
// @Tags         files
// @Produce      application/octet-stream
// @Param        id  path     int true "File ID"
// @Success      200 {file}   binary "File stream"
// @Failure      400 {object} ErrorResponse
// @Failure      401 {object} ErrorResponse
// @Failure      403 {object} ErrorResponse
// @Failure      500 {object} ErrorResponse
// @Security     BearerAuth
// @Router       /files/{id} [get]
func (h *DownloadHandler) Download(w http.ResponseWriter, r *http.Request) {
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

	// ── AUTHORIZATION CHECK ──
	file, err := h.fileRepo.FindByIDAndUserID(r.Context(), fileID, userID)
	if err != nil {
		writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "you do not have access to this file"})
		return
	}

	// Fetch ordered block IDs for this file
	blockIDs, err := h.fileRepo.GetBlockIDs(r.Context(), file.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "db_error", Message: "failed to fetch block ids"})
		return
	}

	// Fetch block metadata (S3 keys)
	blocks, err := h.blockRepo.FindByIDs(r.Context(), blockIDs)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "db_error", Message: "failed to fetch blocks"})
		return
	}

	// Set response headers before streaming
	mimeType := file.MimeType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Name))
	w.Header().Set("Content-Length", strconv.FormatInt(file.TotalSize, 10))

	// Stream blocks directly to response writer
	if err := block.BlocksToStream(r.Context(), blocks, h.s3, w); err != nil {
		// Headers already sent; log the error but can't change status
		return
	}
}

// DeleteFile godoc
// @Summary      Delete a file
// @Description  Delete a file by ID. Decrements block ref counts and removes orphaned blocks from S3.
// @Tags         files
// @Produce      json
// @Param        id  path     int true "File ID"
// @Success      204 "No Content"
// @Failure      400 {object} ErrorResponse
// @Failure      401 {object} ErrorResponse
// @Failure      403 {object} ErrorResponse
// @Failure      500 {object} ErrorResponse
// @Security     BearerAuth
// @Router       /files/{id} [delete]
func (h *DownloadHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
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

	// Fetch block IDs before deleting the file (cascade would remove file_blocks)
	blockIDs, err := h.fileRepo.GetBlockIDs(r.Context(), fileID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "db_error", Message: "failed to fetch block ids"})
		return
	}

	// Delete file record (also cascades file_blocks)
	if err := h.fileRepo.Delete(r.Context(), fileID, userID); err != nil {
		writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "file not found or unauthorized"})
		return
	}

	// Decrement ref_count for each block; delete from S3 + DB if orphaned
	blocks, err := h.blockRepo.FindByIDs(r.Context(), blockIDs)
	if err == nil {
		for _, b := range blocks {
			newCount, err := h.blockRepo.DecrementRefCount(r.Context(), b.ID)
			if err != nil {
				continue // log in production
			}
			if newCount <= 0 {
				_ = h.s3.DeleteObject(r.Context(), b.S3Key)
				_ = h.blockRepo.Delete(r.Context(), b.ID)
			}
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
