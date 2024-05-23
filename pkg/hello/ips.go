package hello

import (
	"net"
	"sync"

	"github.com/sirupsen/logrus"
)

type reservedAddressLister interface {
	ReservedAddresses() ([]string, error)
}

type ipPool struct {
	previous net.IP
	lock     sync.Mutex
}

func (p *ipPool) Next() (string, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	i := p.previous.To4()
	v := uint(i[0])<<24 + uint(i[1])<<16 + uint(i[2])<<8 + uint(i[3])
	v += 1
	v3 := byte(v & 0xFF)
	v2 := byte((v >> 8) & 0xFF)
	v1 := byte((v >> 16) & 0xFF)
	v0 := byte((v >> 24) & 0xFF)
	p.previous = net.IPv4(v0, v1, v2, v3)
	return p.previous.String(), nil

}

type reservedAddressesValidatingIpPool struct {
	child             IPPool
	reservedAddresses reservedAddressLister
}

func (p *reservedAddressesValidatingIpPool) Next() (string, error) {
	for {
		ip, err := p.child.Next()
		if err != nil {
			return "", err
		}
		reserved, err := p.reservedAddresses.ReservedAddresses()
		if err != nil {
			return "", err
		}
		doContinue := false
		for _, r := range reserved {
			if r == ip {
				logrus.Debugf("IP %s is reserved, skipping", ip)
				doContinue = true
			}
		}
		if doContinue {
			continue
		}
		logrus.Debugf("IP %s is not reserved, assigning", ip)
		return ip, nil
	}
}

func NewIPPool(starting string, reserved reservedAddressLister) IPPool {
	ip := net.ParseIP(starting)
	if ip == nil {
		logrus.Panicf("Invalid IP address passed as starting to IP pool: %s", starting)
	}
	return &reservedAddressesValidatingIpPool{
		child:             &ipPool{previous: ip},
		reservedAddresses: reserved,
	}
}

type storageToReservedAddressListerAdapter struct {
	storage PeerStorage
}

func (s *storageToReservedAddressListerAdapter) ReservedAddresses() ([]string, error) {
	peers, err := s.storage.List()
	if err != nil {
		return nil, err
	}
	var ips []string
	for _, p := range peers {
		ips = append(ips, p.IP)
	}
	return ips, nil
}

func NewReservedAddressLister(storage PeerStorage) reservedAddressLister {
	return &storageToReservedAddressListerAdapter{storage: storage}
}
