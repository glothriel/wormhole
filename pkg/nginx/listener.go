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
	includeWg0 bool
}

// Addrs implements Listener
func (a *wg0FilteringListener) Addrs(portNumber int) ([]string, error) {
	interfaces, interfacesErr := net.Interfaces()
	if interfacesErr != nil {
		return []string{}, interfacesErr
	}

	var allAddrs []string

	for _, iface := range interfaces {
		if iface.Name == "wg0" && !a.includeWg0 {
			continue
		}
		if iface.Name != "wg0" && a.includeWg0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			fmt.Println("Error retrieving addresses for interface:", iface.Name, err)
			continue
		}

		for _, addr := range addrs {
			switch v := addr.(type) {
			// Ignore ipv6
			case *net.IPNet:
				if strings.Contains(v.IP.String(), ":") {
					continue
				} else {
					allAddrs = append(allAddrs, formatIP(v.IP, portNumber))
				}

			case *net.IPAddr:
				if strings.Contains(v.IP.String(), ":") {
					continue
				} else {
					allAddrs = append(allAddrs, formatIP(v.IP, portNumber))
				}
			}
		}
	}
	return allAddrs, nil
}

func formatIP(ip fmt.Stringer, portNumber int) string {
	return fmt.Sprintf("%s:%d", ip.String(), portNumber)
}

// NewAllAcceptWireguardListener creates a new Listener that listens on all interfaces accept wg0
func NewAllAcceptWireguardListener() Listener {
	return &wg0FilteringListener{
		includeWg0: false,
	}
}

// NewOnlyWireguardListener creates a new Listener that listens on all interfaces
func NewOnlyWireguardListener() Listener {
	return &wg0FilteringListener{
		includeWg0: true,
	}
}
