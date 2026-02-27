package driver

type Backend uint

const (
	BackendHID Backend = iota
	BackendKernel
)
