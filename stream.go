package bowline

type Stream[Out any] struct {
	emit func(v any) error
}

func (s *Stream[Out]) Send(v Out) error {
	return s.emit(v)
}
