package backend

type Backend uint

const (
	BackendHID Backend = iota
	BackendKernel
)
