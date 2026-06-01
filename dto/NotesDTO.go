package dto

type CreateNoteDTO struct {
	Title   string `json:"title" binding:"required"`
	Content string `json:"content" binding:"required"`
}

type UpdateNoteDTO struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}
