package gre

import (
	"fmt"
	v1 "multi-vpc/api/v1"
	"multi-vpc/internal/tunnel"
)

type GreOperation struct {
	tunnel *v1.VpcNatTunnel
}

func NewGreOp(tunnel *v1.VpcNatTunnel) tunnel.TunnelOperation {
	return &GreOperation{
		tunnel: tunnel,
	}
}

func (g *GreOperation) CreateCmd() string {
	tunnel := g.tunnel

	createCmd := fmt.Sprintf("ip tunnel add %s mode gre remote %s local %s ttl 255", tunnel.Name, tunnel.Spec.RemoteIP, tunnel.Status.InternalIP)
	setUpCmd := fmt.Sprintf("ip link set %s up", tunnel.Name)
	addrCmd := fmt.Sprintf("ip addr add %s dev %s", tunnel.Spec.InterfaceAddr, tunnel.Name)
	return createCmd + ";" + setUpCmd + ";" + addrCmd
}

func (g *GreOperation) DeleteCmd() string {
	delCmd := fmt.Sprintf("ip tunnel del %s", g.tunnel.Name)
	return delCmd
}
