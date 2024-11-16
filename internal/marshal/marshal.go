package marshal

type Marshal interface {
	JSON() ([]byte, error)
}
