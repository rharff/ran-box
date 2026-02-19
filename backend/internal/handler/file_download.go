package handler

import (
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"

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
// @Failure      400 {object} map[string]interface{} "invalid file id"
// @Failure      401 {object} map[string]interface{} "unauthorized"
// @Failure      403 {object} map[string]interface{} "forbidden"
// @Failure      500 {object} map[string]interface{} "db_error"
// @Security     BearerAuth
// @Router       /files/{id} [get]
func (h *DownloadHandler) Download(c *fiber.Ctx) error {
	userID, ok := auth.GetUserID(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	fileID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "bad_request",
			"message": "invalid file id",
		})
	}

	// ── AUTHORIZATION CHECK ──
	// FindByIDAndUserID returns error/nil if file doesn't belong to this user
	file, err := h.fileRepo.FindByIDAndUserID(c.Context(), fileID, userID)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error":   "forbidden",
			"message": "you do not have access to this file",
		})
	}

	// Fetch ordered block IDs for this file
	blockIDs, err := h.fileRepo.GetBlockIDs(c.Context(), file.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "db_error"})
	}

	// Fetch block metadata (S3 keys)
	blocks, err := h.blockRepo.FindByIDs(c.Context(), blockIDs)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "db_error"})
	}

	// Set response headers before streaming
	mimeType := file.MimeType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	c.Set("Content-Type", mimeType)
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Name))
	c.Set("Content-Length", strconv.FormatInt(file.TotalSize, 10))

	// Stream blocks directly to response writer
	if err := block.BlocksToStream(c.Context(), blocks, h.s3, c.Response().BodyWriter()); err != nil {
		// Headers already sent; log the error but can't change status
		return err
	}

	return nil
}

// DeleteFile godoc
// @Summary      Delete a file
// @Description  Delete a file by ID. Decrements block ref counts and removes orphaned blocks from S3.
// @Tags         files
// @Produce      json
// @Param        id  path     int true "File ID"
// @Success      204 "No Content"
// @Failure      400 {object} map[string]interface{} "invalid file id"
// @Failure      401 {object} map[string]interface{} "unauthorized"
// @Failure      403 {object} map[string]interface{} "file not found or unauthorized"
// @Security     BearerAuth
// @Router       /files/{id} [delete]
func (h *DownloadHandler) DeleteFile(c *fiber.Ctx) error {
	userID, ok := auth.GetUserID(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	fileID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "bad_request",
			"message": "invalid file id",
		})
	}

	// Fetch block IDs before deleting the file (cascade would remove file_blocks)
	blockIDs, err := h.fileRepo.GetBlockIDs(c.Context(), fileID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "db_error"})
	}

	// Delete file record (also cascades file_blocks)
	if err := h.fileRepo.Delete(c.Context(), fileID, userID); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error":   "forbidden",
			"message": "file not found or unauthorized",
		})
	}

	// Decrement ref_count for each block; delete from S3 + DB if orphaned
	blocks, err := h.blockRepo.FindByIDs(c.Context(), blockIDs)
	if err == nil {
		for _, b := range blocks {
			newCount, err := h.blockRepo.DecrementRefCount(c.Context(), b.ID)
			if err != nil {
				continue // log in production
			}
			if newCount <= 0 {
				_ = h.s3.DeleteObject(c.Context(), b.S3Key)
				_ = h.blockRepo.Delete(c.Context(), b.ID)
			}
		}
	}

	return c.SendStatus(fiber.StatusNoContent)
}
