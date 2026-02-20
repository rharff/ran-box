package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/naratel/naratel-box/backend/internal/auth"
	"github.com/naratel/naratel-box/backend/internal/model"
	"github.com/naratel/naratel-box/backend/internal/repository"
)

type FolderHandler struct {
	folderRepo *repository.FolderRepository
	fileRepo   *repository.FileRepository
}

func NewFolderHandler(folderRepo *repository.FolderRepository, fileRepo *repository.FileRepository) *FolderHandler {
	return &FolderHandler{
		folderRepo: folderRepo,
		fileRepo:   fileRepo,
	}
}

// CreateFolderRequest is the payload for POST /folders.
type CreateFolderRequest struct {
	Name     string `json:"name"`
	ParentID *int64 `json:"parent_id,omitempty"`
}

// CreateFolder godoc
// @Summary      Create a folder
// @Tags         folders
// @Accept       json
// @Produce      json
// @Param        body body     CreateFolderRequest true "Folder details"
// @Success      201  {object} model.Folder
// @Security     BearerAuth
// @Router       /folders [post]
func (h *FolderHandler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	var req CreateFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "name is required"})
		return
	}

	folder, err := h.folderRepo.Create(r.Context(), userID, req.ParentID, req.Name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "db_error", Message: "failed to create folder"})
		return
	}

	writeJSON(w, http.StatusCreated, folder)
}

// ListFolderContents godoc
// @Summary      List folder contents
// @Description  Returns subfolders and files within a folder. Omit folder_id for root.
// @Tags         folders
// @Produce      json
// @Param        folder_id query int false "Folder ID (omit for root)"
// @Success      200  {object} FolderContentsResponse
// @Security     BearerAuth
// @Router       /folders/contents [get]
func (h *FolderHandler) ListFolderContents(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	var folderID *int64
	if fid := r.URL.Query().Get("folder_id"); fid != "" {
		parsed, err := strconv.ParseInt(fid, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "invalid folder_id"})
			return
		}
		folderID = &parsed
	}

	folders, err := h.folderRepo.ListByParent(r.Context(), userID, folderID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "db_error", Message: "failed to list folders"})
		return
	}
	if folders == nil {
		folders = []*model.Folder{}
	}

	files, err := h.fileRepo.ListByFolder(r.Context(), userID, folderID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "db_error", Message: "failed to list files"})
		return
	}
	if files == nil {
		files = []*model.File{}
	}

	writeJSON(w, http.StatusOK, FolderContentsResponse{
		Folders: folders,
		Files:   files,
	})
}

// RenameFolderRequest is the payload for PATCH /folders/{id}/rename.
type RenameFolderRequest struct {
	Name string `json:"name"`
}

// RenameFolder godoc
// @Summary      Rename a folder
// @Tags         folders
// @Accept       json
// @Produce      json
// @Param        id   path     int                  true "Folder ID"
// @Param        body body     RenameFolderRequest   true "New name"
// @Success      200  {object} model.Folder
// @Security     BearerAuth
// @Router       /folders/{id}/rename [patch]
func (h *FolderHandler) RenameFolder(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	folderID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "invalid folder id"})
		return
	}

	var req RenameFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "name is required"})
		return
	}

	folder, err := h.folderRepo.Rename(r.Context(), folderID, userID, req.Name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "folder not found"})
		return
	}

	writeJSON(w, http.StatusOK, folder)
}

// MoveFolderRequest is the payload for PATCH /folders/{id}/move.
type MoveFolderRequest struct {
	ParentID *int64 `json:"parent_id"` // null = move to root
}

// MoveFolder godoc
// @Summary      Move a folder
// @Tags         folders
// @Accept       json
// @Produce      json
// @Param        id   path     int              true "Folder ID"
// @Param        body body     MoveFolderRequest true "New parent"
// @Success      200  {object} model.Folder
// @Security     BearerAuth
// @Router       /folders/{id}/move [patch]
func (h *FolderHandler) MoveFolder(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	folderID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "invalid folder id"})
		return
	}

	var req MoveFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "invalid JSON body"})
		return
	}

	// Prevent moving a folder into itself
	if req.ParentID != nil && *req.ParentID == folderID {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "cannot move folder into itself"})
		return
	}

	folder, err := h.folderRepo.Move(r.Context(), folderID, userID, req.ParentID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "folder not found"})
		return
	}

	writeJSON(w, http.StatusOK, folder)
}

// DeleteFolder godoc
// @Summary      Delete a folder
// @Description  Deletes a folder and all its contents recursively.
// @Tags         folders
// @Produce      json
// @Param        id path int true "Folder ID"
// @Success      204
// @Security     BearerAuth
// @Router       /folders/{id} [delete]
func (h *FolderHandler) DeleteFolder(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	folderID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "invalid folder id"})
		return
	}

	if err := h.folderRepo.Delete(r.Context(), folderID, userID); err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "folder not found or unauthorized"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Breadcrumb godoc
// @Summary      Get breadcrumb for a folder
// @Tags         folders
// @Produce      json
// @Param        id path int true "Folder ID"
// @Success      200  {array} model.Folder
// @Security     BearerAuth
// @Router       /folders/{id}/breadcrumb [get]
func (h *FolderHandler) Breadcrumb(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	folderID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "invalid folder id"})
		return
	}

	crumbs, err := h.folderRepo.GetBreadcrumb(r.Context(), folderID, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "db_error", Message: "failed to get breadcrumb"})
		return
	}
	if crumbs == nil {
		crumbs = []*model.Folder{}
	}

	writeJSON(w, http.StatusOK, crumbs)
}

// ListAllFolders godoc
// @Summary      List all folders for move dialog
// @Tags         folders
// @Produce      json
// @Success      200  {array} model.Folder
// @Security     BearerAuth
// @Router       /folders/all [get]
func (h *FolderHandler) ListAllFolders(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	folders, err := h.folderRepo.ListAllByUser(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "db_error", Message: "failed to list folders"})
		return
	}
	if folders == nil {
		folders = []*model.Folder{}
	}

	writeJSON(w, http.StatusOK, folders)
}
