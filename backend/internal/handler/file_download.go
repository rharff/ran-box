package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/naratel/naratel-box/backend/internal/auth"
	"github.com/naratel/naratel-box/backend/internal/block"
	"github.com/naratel/naratel-box/backend/internal/logger"
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
		logger.Warn(r.Context(), "Unauthorized download attempt", nil)
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: "missing token"})
		return
	}

	fileID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "invalid file id"})
		return
	}

	logger.Info(r.Context(), "File download initiated", map[string]interface{}{
		"user_id": userID, "file_id": fileID,
	})

	// ── AUTHORIZATION CHECK ──
	file, err := h.fileRepo.FindByIDAndUserID(r.Context(), fileID, userID)
	if err != nil {
		logger.Warn(r.Context(), "Download forbidden - file not found or unauthorized", map[string]interface{}{
			"user_id": userID, "file_id": fileID,
		})
		writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "you do not have access to this file"})
		return
	}

	// Fetch ordered block IDs for this file
	blockIDs, err := h.fileRepo.GetBlockIDs(r.Context(), file.ID)
	if err != nil {
		logger.ErrorLog(r.Context(), "Failed to fetch block IDs for download", logger.ErrorDetails{
			Code: "DB_ERR", Details: err.Error(),
		})
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "db_error", Message: "failed to fetch block ids"})
		return
	}

	// Fetch block metadata (S3 keys)
	blocks, err := h.blockRepo.FindByIDs(r.Context(), blockIDs)
	if err != nil {
		logger.ErrorLog(r.Context(), "Failed to fetch block metadata for download", logger.ErrorDetails{
			Code: "DB_ERR", Details: err.Error(),
		})
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "db_error", Message: "failed to fetch blocks"})
		return
	}

	// Set response headers before streaming
	mimeType := file.MimeType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Support preview mode (inline display for images, PDFs, text)
	if r.URL.Query().Get("preview") == "true" {
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, file.Name))
	} else {
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Name))
	}
	w.Header().Set("Content-Length", strconv.FormatInt(file.TotalSize, 10))

	// Stream blocks directly to response writer
	if err := block.BlocksToStream(r.Context(), blocks, h.s3, w); err != nil {
		logger.ErrorLog(r.Context(), "File download streaming failed", logger.ErrorDetails{
			Code: "S3_STREAM_ERR", Details: err.Error(),
		})
		// Headers already sent; can't change status
		return
	}

	logger.Info(r.Context(), "File downloaded successfully", map[string]interface{}{
		"user_id":    userID,
		"file_id":    file.ID,
		"file_name":  file.Name,
		"total_size": file.TotalSize,
		"blocks":     len(blocks),
	})
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
		logger.Warn(r.Context(), "Unauthorized delete attempt", nil)
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: "missing token"})
		return
	}

	fileID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "invalid file id"})
		return
	}

	logger.Info(r.Context(), "File deletion initiated", map[string]interface{}{
		"user_id": userID, "file_id": fileID,
	})

	// Fetch block IDs before deleting the file (cascade would remove file_blocks)
	blockIDs, err := h.fileRepo.GetBlockIDs(r.Context(), fileID)
	if err != nil {
		logger.ErrorLog(r.Context(), "Failed to fetch block IDs for deletion", logger.ErrorDetails{
			Code: "DB_ERR", Details: err.Error(),
		})
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "db_error", Message: "failed to fetch block ids"})
		return
	}

	// Delete file record (also cascades file_blocks)
	if err := h.fileRepo.Delete(r.Context(), fileID, userID); err != nil {
		logger.Warn(r.Context(), "File deletion failed - not found or unauthorized", map[string]interface{}{
			"user_id": userID, "file_id": fileID, "error": err.Error(),
		})
		writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "file not found or unauthorized"})
		return
	}

	// Decrement ref_count for each block; delete from S3 + DB if orphaned
	blocks, err := h.blockRepo.FindByIDs(r.Context(), blockIDs)
	if err == nil {
		for _, b := range blocks {
			newCount, err := h.blockRepo.DecrementRefCount(r.Context(), b.ID)
			if err != nil {
				logger.ErrorLog(r.Context(), "Failed to decrement block ref count", logger.ErrorDetails{
					Code: "BLOCK_DEREF_ERR", Details: fmt.Sprintf("block_id=%d: %s", b.ID, err.Error()),
				})
				continue
			}
			if newCount <= 0 {
				if err := h.s3.DeleteObject(r.Context(), b.S3Key); err != nil {
					logger.ErrorLog(r.Context(), "Failed to delete orphaned block from S3", logger.ErrorDetails{
						Code: "S3_DELETE_ERR", Details: fmt.Sprintf("s3_key=%s: %s", b.S3Key, err.Error()),
					})
				}
				if err := h.blockRepo.Delete(r.Context(), b.ID); err != nil {
					logger.ErrorLog(r.Context(), "Failed to delete orphaned block from DB", logger.ErrorDetails{
						Code: "DB_DELETE_ERR", Details: fmt.Sprintf("block_id=%d: %s", b.ID, err.Error()),
					})
				}
				logger.Info(r.Context(), "Orphaned block garbage collected", map[string]interface{}{
					"block_id": b.ID, "s3_key": b.S3Key,
				})
			}
		}
	}

	logger.Info(r.Context(), "File deleted successfully", map[string]interface{}{
		"user_id": userID, "file_id": fileID, "blocks_processed": len(blockIDs),
	})
	w.WriteHeader(http.StatusNoContent)
}
