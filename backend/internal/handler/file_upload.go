package handler

import (
	"mime"
	"path/filepath"
	"strconv"

	"github.com/gofiber/fiber/v2"

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
// @Failure      400  {object} map[string]interface{} "bad_request"
// @Failure      401  {object} map[string]interface{} "unauthorized"
// @Failure      500  {object} map[string]interface{} "upload_failed"
// @Security     BearerAuth
// @Router       /files [post]
func (h *UploadHandler) Upload(c *fiber.Ctx) error {
	userID, ok := auth.GetUserID(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	// Parse multipart form
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "bad_request",
			"message": "field 'file' is required",
		})
	}

	// Open the uploaded file
	f, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "cannot open file"})
	}
	defer f.Close()

	// Detect MIME type from extension (fallback to octet-stream)
	mimeType := mime.TypeByExtension(filepath.Ext(fileHeader.Filename))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Process: split → hash → dedup → upload blocks
	blockIDs, totalBytes, err := h.processor.Process(c.Context(), f)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "upload_failed",
			"message": err.Error(),
		})
	}

	// Create file metadata record
	file, err := h.fileRepo.Create(c.Context(), userID, fileHeader.Filename, mimeType, totalBytes)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "db_error",
			"message": "failed to save file metadata",
		})
	}

	// Link blocks to file
	if err := h.fileRepo.LinkBlocks(c.Context(), file.ID, blockIDs); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "db_error",
			"message": "failed to link blocks",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"file_id":      file.ID,
		"name":         file.Name,
		"mime_type":    file.MimeType,
		"size":         file.TotalSize,
		"blocks_count": len(blockIDs),
		"created_at":   file.CreatedAt,
	})
}

// ListFiles godoc
// @Summary      List files
// @Description  Returns all files owned by the authenticated user
// @Tags         files
// @Produce      json
// @Success      200  {array}  model.File
// @Failure      401  {object} map[string]interface{} "unauthorized"
// @Failure      500  {object} map[string]interface{} "db_error"
// @Security     BearerAuth
// @Router       /files [get]
func (h *UploadHandler) ListFiles(c *fiber.Ctx) error {
	userID, ok := auth.GetUserID(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	files, err := h.fileRepo.ListByUserID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "db_error", Message: "failed to list files"})
	}
	if files == nil {
		files = []*model.File{}
	}
	return c.JSON(files)
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
func (h *UploadHandler) FileInfo(c *fiber.Ctx) error {
	userID, ok := auth.GetUserID(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{Error: "unauthorized", Message: "missing token"})
	}

	fileID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "bad_request", Message: "invalid file id"})
	}

	file, err := h.fileRepo.FindByIDAndUserID(c.Context(), fileID, userID)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{Error: "forbidden", Message: "file not found or unauthorized"})
	}

	return c.JSON(file)
}
