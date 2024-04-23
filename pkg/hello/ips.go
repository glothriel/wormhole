package hello

import (
	"net"
	"sync"

	"github.com/sirupsen/logrus"
)

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

func NewIPPool(starting string) IPPool {
	ip := net.ParseIP(starting)
	if ip == nil {
		logrus.Panicf("Invalid IP address passed as starting to IP pool: %s", starting)
	}
	return &ipPool{previous: ip}
}

