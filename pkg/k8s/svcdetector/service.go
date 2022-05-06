package svcdetector

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/glothriel/wormhole/pkg/peers"
	corev1 "k8s.io/api/core/v1"
)

type serviceWrapper interface {
	id() string
	shouldBeExposed() bool
	name() string
	apps() []peers.App
}

type defaultServiceWrapper struct {
	k8sSvc *corev1.Service
}

func (wrapper defaultServiceWrapper) id() string {
	return fmt.Sprintf("%s-%s", wrapper.k8sSvc.ObjectMeta.Namespace, wrapper.k8sSvc.ObjectMeta.Name)
}

func (wrapper defaultServiceWrapper) shouldBeExposed() bool {
	annotation, annotationOK := wrapper.k8sSvc.ObjectMeta.GetAnnotations()["wormhole.glothriel.github.com/exposed"]
	if !annotationOK {
		return false
	}
	if annotation == "1" || annotation == "true" || annotation == "yes" {
		return true
	}
	return false
}

func (wrapper defaultServiceWrapper) name() string {
	exposeName, exposeOk := wrapper.k8sSvc.ObjectMeta.GetAnnotations()["wormhole.glothriel.github.com/name"]
	if !exposeOk {
		return wrapper.id()
	}
	return exposeName
}

func (wrapper defaultServiceWrapper) ports() []corev1.ServicePort {
	ports, portsOk := wrapper.k8sSvc.ObjectMeta.GetAnnotations()["wormhole.glothriel.github.com/ports"]
	if !portsOk {
		return wrapper.k8sSvc.Spec.Ports
	}
	thePorts := make([]corev1.ServicePort, 0)
	for _, rawPortID := range strings.Split(ports, ",") {
		portAsNumber, atoiErr := strconv.ParseInt(rawPortID, 10, 32)
		if atoiErr != nil {
			for _, portDefinition := range wrapper.k8sSvc.Spec.Ports {
				if portDefinition.Name == rawPortID {
					thePorts = append(thePorts, *portDefinition.DeepCopy())
				}
			}
		} else {
			for _, portDefinition := range wrapper.k8sSvc.Spec.Ports {
				if portDefinition.Port == int32(portAsNumber) {
					thePorts = append(thePorts, *portDefinition.DeepCopy())
				}
			}
		}
	}
	return thePorts
}

func (wrapper defaultServiceWrapper) apps() []peers.App {
	apps := make([]peers.App, 0)
	exposedPorts := wrapper.ports()
	for _, portDefinition := range exposedPorts {
		if portDefinition.Protocol != "TCP" {
			continue
		}
		portName := wrapper.name()
		if len(exposedPorts) > 1 {
			portName = fmt.Sprintf("%s-%s", wrapper.name(), portDefinition.Name)
		}
		apps = append(apps, peers.App{
			Name: portName,
			Address: fmt.Sprintf(
				"%s.%s:%d",
				wrapper.k8sSvc.ObjectMeta.Name,
				wrapper.k8sSvc.ObjectMeta.Namespace,
				portDefinition.Port,
			),
		})
	}
	return apps
}

func newDefaultServiceWrapper(svc *corev1.Service) defaultServiceWrapper {
	return defaultServiceWrapper{k8sSvc: svc}
}
