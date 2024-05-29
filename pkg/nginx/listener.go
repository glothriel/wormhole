package nginx

import (
	"errors"
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

type allAcceptWg0Listener struct {
	lister networkInterfaceLister
}

// Addrs implements Listener
func (a *allAcceptWg0Listener) Addrs(portNumber int) ([]string, error) {
	interfaces, interfacesErr := a.lister.Interfaces()
	if interfacesErr != nil {
		return []string{}, interfacesErr
	}

	var allAddrs []string

	for _, iface := range interfaces {
		if iface.name == "wg0" {
			continue
		}

		for _, addr := range iface.addresses {
			// Ignore IPv6
			if !strings.Contains(addr, "::") {
				allAddrs = append(allAddrs, fmt.Sprintf("%s:%d", addr, portNumber))
			}
		}
	}

	if len(allAddrs) == 0 {
		return []string{}, errors.New("No network interfaces matching conditions found")
	}

	return allAddrs, nil
}

type givenAddressOnlyListener struct {
	address string
}

// Addrs implements Listener
func (l *givenAddressOnlyListener) Addrs(portNumber int) ([]string, error) {
	return []string{
		fmt.Sprintf("%s:%d", l.address, portNumber),
	}, nil
}

// NewAllAcceptWireguardListener creates a new Listener that listens on all interfaces accept wg0
func NewAllAcceptWireguardListener() Listener {
	return &allAcceptWg0Listener{
		lister: &defaultNetworkInterfaceLister{},
	}
}

// NewOnlyGivenAddressListener creates a new Listener that listens only on given address
func NewOnlyGivenAddressListener(address string) Listener {
	return &givenAddressOnlyListener{
		address: address,
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
