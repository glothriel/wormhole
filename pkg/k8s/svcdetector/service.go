package svcdetector

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/glothriel/wormhole/pkg/apps"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

type serviceWrapper interface {
	id() string
	shouldBeExposed() bool
	name() string
	apps() []apps.App
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

func (wrapper defaultServiceWrapper) targetLabels() string {
	labels, labelsOk := wrapper.k8sSvc.ObjectMeta.GetAnnotations()["wormhole.glothriel.github.com/labels"]
	if !labelsOk {
		return ""
	}
	return labels
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
				portAsInt32, portErr := safePortConversion(portAsNumber)
				if portErr != nil {
					logrus.Errorf("invalid port number: %v", portErr)
					continue
				}

				if portDefinition.Port == portAsInt32 {
					thePorts = append(thePorts, *portDefinition.DeepCopy())
				}
			}
		}
	}
	return thePorts
}

func safePortConversion(portNumber int64) (int32, error) {
	// Check lower bound
	if portNumber < 0 {
		return 0, fmt.Errorf("port number cannot be negative: %d", portNumber)
	}

	// Check upper bound
	if portNumber > math.MaxInt32 {
		return 0, fmt.Errorf("port number exceeds maximum int32 value: %d", portNumber)
	}

	return int32(portNumber), nil // nolint: gosec
}

func (wrapper defaultServiceWrapper) apps() []apps.App {
	theApps := make([]apps.App, 0)
	exposedPorts := wrapper.ports()
	for _, portDefinition := range exposedPorts {
		if portDefinition.Protocol != "TCP" {
			continue
		}
		portName := wrapper.name()
		if len(exposedPorts) > 1 {
			portName = fmt.Sprintf("%s-%s", wrapper.name(), portDefinition.Name)
		}
		theApps = append(theApps, apps.App{
			Name: portName,
			Address: fmt.Sprintf(
				"%s.%s:%d",
				wrapper.k8sSvc.ObjectMeta.Name,
				wrapper.k8sSvc.ObjectMeta.Namespace,
				portDefinition.Port,
			),
			TargetLabels: wrapper.targetLabels(),
			OriginalPort: portDefinition.Port,
		})
	}
	return theApps
}

func newDefaultServiceWrapper(svc *corev1.Service) defaultServiceWrapper {
	return defaultServiceWrapper{k8sSvc: svc}
}
