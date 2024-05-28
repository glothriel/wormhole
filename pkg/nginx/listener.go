package nginx

import (
	"fmt"
	"net"
	"strings"
)

// Listener is an interface for NGINX listeners
type Listener interface {
	// Addrs returns a list of addresses that the listener is listening on
	Addrs(portNumber int) ([]string, error)
}

type networkInterface struct {
	name      string
	addresses []string
}

type networkInterfaceLister interface {
	Interfaces() ([]networkInterface, error)
}

type portOnlyListener struct {
}

// Addrs implements Listener
func (p *portOnlyListener) Addrs(portNumber int) ([]string, error) {
	return []string{fmt.Sprintf("%d", portNumber)}, nil
}

// NewPortOnlyListener creates a new Listener that listens on a single port
func NewPortOnlyListener() Listener {
	return &portOnlyListener{}
}

type wg0FilteringListener struct {
	lister     networkInterfaceLister
	includeWg0 bool
}

// Addrs implements Listener
func (a *wg0FilteringListener) Addrs(portNumber int) ([]string, error) {
	interfaces, interfacesErr := a.lister.Interfaces()
	if interfacesErr != nil {
		return []string{}, interfacesErr
	}

	var allAddrs []string

	for _, iface := range interfaces {
		if iface.name == "wg0" && !a.includeWg0 {
			continue
		}
		if iface.name != "wg0" && a.includeWg0 {
			continue
		}

		for _, addr := range iface.addresses {
			// Ignore IPv6
			if !strings.Contains(addr, "::") {
				allAddrs = append(allAddrs, fmt.Sprintf("%s:%d", addr, portNumber))
			}
		}
	}
	return allAddrs, nil
}

// NewAllAcceptWireguardListener creates a new Listener that listens on all interfaces accept wg0
func NewAllAcceptWireguardListener() Listener {
	return &wg0FilteringListener{
		includeWg0: false,
		lister:     &defaultNetworkInterfaceLister{},
	}
}

// NewOnlyWireguardListener creates a new Listener that listens on all interfaces
func NewOnlyWireguardListener() Listener {
	return &wg0FilteringListener{
		includeWg0: true,
		lister:     &defaultNetworkInterfaceLister{},
	}
}

type defaultNetworkInterfaceLister struct {
}

// Interfaces implements networkinterfacelister
func (l *defaultNetworkInterfaceLister) Interfaces() ([]networkInterface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var result []networkInterface
	for _, iface := range ifaces {
		netAddrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}
		var addrs []string
		for _, addr := range netAddrs {
			switch v := addr.(type) {
			case *net.IPNet:
				addrs = append(addrs, v.IP.String())
			case *net.IPAddr:
				addrs = append(addrs, v.IP.String())
			}
		}
		result = append(result, networkInterface{
			name:      iface.Name,
			addresses: addrs,
		})
	}
	return result, nil
}
