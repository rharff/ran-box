package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/naratel/naratel-box/backend/internal/auth"
	"github.com/naratel/naratel-box/backend/internal/block"
	"github.com/naratel/naratel-box/backend/internal/logger"
	"github.com/naratel/naratel-box/backend/internal/repository"
	"github.com/naratel/naratel-box/backend/internal/storage"
)

type ShareHandler struct {
	shareRepo *repository.ShareLinkRepository
	fileRepo  *repository.FileRepository
	blockRepo *repository.BlockRepository
	s3        *storage.S3Client
}

func NewShareHandler(
	shareRepo *repository.ShareLinkRepository,
	fileRepo *repository.FileRepository,
	blockRepo *repository.BlockRepository,
	s3 *storage.S3Client,
) *ShareHandler {
	return &ShareHandler{
		shareRepo: shareRepo,
		fileRepo:  fileRepo,
		blockRepo: blockRepo,
		s3:        s3,
	}
}

// ShareLinkResponse is returned when creating a share link.
type ShareLinkResponse struct {
	ID        int64      `json:"id"`
	FileID    int64      `json:"file_id"`
	Token     string     `json:"token"`
	URL       string     `json:"url"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// CreateShareLink godoc
// @Summary      Create a share link for a file
// @Tags         share
// @Produce      json
// @Param        id path int true "File ID"
// @Success      201  {object} ShareLinkResponse
// @Security     BearerAuth
// @Router       /files/{id}/share [post]
func (h *ShareHandler) CreateShareLink(w http.ResponseWriter, r *http.Request) {
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

	// Verify ownership
	_, err = h.fileRepo.FindByIDAndUserID(r.Context(), fileID, userID)
	if err != nil {
		logger.Warn(r.Context(), "Share link creation forbidden", map[string]interface{}{
			"user_id": userID, "file_id": fileID,
		})
		writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "file not found or unauthorized"})
		return
	}

	// Generate a random token
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		logger.ErrorLog(r.Context(), "Failed to generate share token", logger.ErrorDetails{
			Code: "CRYPTO_ERR", Details: err.Error(),
		})
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "failed to generate token"})
		return
	}
	token := hex.EncodeToString(tokenBytes)

	// 7-day expiry by default
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	link, err := h.shareRepo.Create(r.Context(), fileID, userID, token, &expiresAt)
	if err != nil {
		logger.ErrorLog(r.Context(), "Failed to create share link", logger.ErrorDetails{
			Code: "DB_ERR", Details: err.Error(),
		})
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "db_error", Message: "failed to create share link"})
		return
	}

	logger.Info(r.Context(), "Share link created successfully", map[string]interface{}{
		"user_id": userID, "file_id": fileID, "link_id": link.ID, "expires_at": expiresAt.Format(time.RFC3339),
	})

	writeJSON(w, http.StatusCreated, ShareLinkResponse{
		ID:        link.ID,
		FileID:    link.FileID,
		Token:     link.Token,
		URL:       fmt.Sprintf("/api/v1/share/%s", link.Token),
		ExpiresAt: link.ExpiresAt,
		CreatedAt: link.CreatedAt,
	})
}

// GetShareLinks godoc
// @Summary      Get share links for a file
// @Tags         share
// @Produce      json
// @Param        id path int true "File ID"
// @Success      200  {array} ShareLinkResponse
// @Security     BearerAuth
// @Router       /files/{id}/share [get]
func (h *ShareHandler) GetShareLinks(w http.ResponseWriter, r *http.Request) {
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

	links, err := h.shareRepo.FindByFileID(r.Context(), fileID, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "db_error", Message: "failed to fetch share links"})
		return
	}

	responses := make([]ShareLinkResponse, 0, len(links))
	for _, l := range links {
		responses = append(responses, ShareLinkResponse{
			ID:        l.ID,
			FileID:    l.FileID,
			Token:     l.Token,
			URL:       fmt.Sprintf("/api/v1/share/%s", l.Token),
			ExpiresAt: l.ExpiresAt,
			CreatedAt: l.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, responses)
}

// DeleteShareLink godoc
// @Summary      Delete a share link
// @Tags         share
// @Param        linkId path int true "Share Link ID"
// @Success      204
// @Security     BearerAuth
// @Router       /share/{linkId} [delete]
func (h *ShareHandler) DeleteShareLink(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	linkID, err := strconv.ParseInt(chi.URLParam(r, "linkId"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "invalid link id"})
		return
	}

	if err := h.shareRepo.Delete(r.Context(), linkID, userID); err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "share link not found"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DownloadShared godoc
// @Summary      Download a file via share link (public)
// @Tags         share
// @Produce      application/octet-stream
// @Param        token path string true "Share token"
// @Success      200 {file} binary
// @Failure      404 {object} ErrorResponse
// @Failure      410 {object} ErrorResponse
// @Router       /share/{token} [get]
func (h *ShareHandler) DownloadShared(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	logger.Info(r.Context(), "Public share download initiated", map[string]interface{}{
		"token": token,
	})

	link, err := h.shareRepo.FindByToken(r.Context(), token)
	if err != nil || link == nil {
		logger.Warn(r.Context(), "Share link not found", map[string]interface{}{"token": token})
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "share link not found"})
		return
	}

	// Check expiry
	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		logger.Warn(r.Context(), "Expired share link accessed", map[string]interface{}{
			"token": token, "link_id": link.ID, "expired_at": link.ExpiresAt.Format(time.RFC3339),
		})
		writeJSON(w, http.StatusGone, ErrorResponse{Error: "expired", Message: "share link has expired"})
		return
	}

	// Fetch file (no user check — public share)
	file, err := h.fileRepo.FindByID(r.Context(), link.FileID)
	if err != nil {
		logger.ErrorLog(r.Context(), "Shared file not found", logger.ErrorDetails{
			Code: "FILE_NOT_FOUND", Details: err.Error(),
		})
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "file not found"})
		return
	}

	blockIDs, err := h.fileRepo.GetBlockIDs(r.Context(), file.ID)
	if err != nil {
		logger.ErrorLog(r.Context(), "Failed to fetch block IDs for shared download", logger.ErrorDetails{
			Code: "DB_ERR", Details: err.Error(),
		})
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "db_error", Message: "failed to fetch block ids"})
		return
	}

	blocks, err := h.blockRepo.FindByIDs(r.Context(), blockIDs)
	if err != nil {
		logger.ErrorLog(r.Context(), "Failed to fetch blocks for shared download", logger.ErrorDetails{
			Code: "DB_ERR", Details: err.Error(),
		})
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "db_error", Message: "failed to fetch blocks"})
		return
	}

	mimeType := file.MimeType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Check if preview is requested (inline display)
	if r.URL.Query().Get("preview") == "true" {
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, file.Name))
	} else {
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Name))
	}
	w.Header().Set("Content-Length", strconv.FormatInt(file.TotalSize, 10))

	if err := block.BlocksToStream(r.Context(), blocks, h.s3, w); err != nil {
		logger.ErrorLog(r.Context(), "Shared file streaming failed", logger.ErrorDetails{
			Code: "S3_STREAM_ERR", Details: err.Error(),
		})
		return
	}

	logger.Info(r.Context(), "Shared file downloaded successfully", map[string]interface{}{
		"token": token, "file_id": file.ID, "file_name": file.Name, "total_size": file.TotalSize,
	})
}
