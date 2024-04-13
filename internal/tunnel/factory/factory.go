package factory

import (
	v1 "multi-vpc/api/v1"
	"multi-vpc/internal/tunnel"
	"multi-vpc/internal/tunnel/gre"
	"multi-vpc/internal/tunnel/vxlan"
)

type TunnelOperationFactory struct {
}

func NewTunnelOpFactory() *TunnelOperationFactory {
	return &TunnelOperationFactory{}
}

type TunnelType string

const (
	VXLAN = "vxlan"
	GRE   = "gre"
)

func (f *TunnelOperationFactory) CreateTunnelOperation(tunnel *v1.VpcNatTunnel) tunnel.TunnelOperation {
	switch tunnel.Spec.Type {
	case "vxlan":
		return vxlan.NewVxlanOp(tunnel)
	case "gre":
		return gre.NewGreOp(tunnel)
	default:
		return gre.NewGreOp(tunnel)
	}
}
