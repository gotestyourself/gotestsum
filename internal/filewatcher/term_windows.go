package filewatcher

import "context"

type redoHandler struct{}

func newRedoHandler() *redoHandler {
	return nil
}

func (r *redoHandler) Run(_ context.Context) {}

func (r *redoHandler) Ch() <-chan string {
	return nil
}

func (r *redoHandler) Reset() {}

func (r *redoHandler) Save(_ string) {}
