package http

type Handler struct {
}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) Close() {
}
