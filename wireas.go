package bowline

type Wire struct{}

func WireAs[T, W any]() Wire {
	return Wire{}
}
