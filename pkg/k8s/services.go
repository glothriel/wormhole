package k8s

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

type managedK8sService struct {
	namespace string
	selectors map[string]string
}

func (m *managedK8sService) Add(metadata k8sResourceMetadata, clientset *kubernetes.Clientset) error {
	servicesClient := clientset.CoreV1().Services(m.namespace)

	port, portErr := extractPortFromAddr(metadata.afterExposedApp.Address)
	if portErr != nil {
		return portErr
	}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      metadata.entityName,
			Namespace: m.namespace,
			Labels:    resourceLabels(metadata.afterExposedApp),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Port:       metadata.originalApp.OriginalPort,
				TargetPort: intstr.FromInt(port),
			}},
			Selector: m.selectors,
		},
	}
	var upsertErr error
	previousService, getErr := servicesClient.Get(context.Background(), metadata.entityName, metav1.GetOptions{})
	if errors.IsNotFound(getErr) {
		logrus.Infof("Creating service %s", metadata.entityName)
		_, upsertErr = servicesClient.Create(context.Background(), service, metav1.CreateOptions{})
	} else if getErr != nil {
		return getErr
	} else {
		logrus.Infof("Updating service %s", metadata.entityName)
		service.SetResourceVersion(previousService.GetResourceVersion())
		_, upsertErr = servicesClient.Update(context.Background(), service, metav1.UpdateOptions{})
	}
	if upsertErr != nil {
		return upsertErr
	}
	return nil
}

func (m *managedK8sService) Remove(entityName string, clientset *kubernetes.Clientset) error {
	servicesClient := clientset.CoreV1().Services(m.namespace)
	deleteErr := servicesClient.Delete(context.Background(), capName(
		entityName,
	), metav1.DeleteOptions{})
	if deleteErr != nil {
		return fmt.Errorf("Could not delete service %s: %v", capName(entityName), deleteErr)
	}
	logrus.Infof("Deleted service %s", capName(entityName))
	return nil
}

func (m *managedK8sService) RemoveAll(clientset *kubernetes.Clientset) error { // nolint:dupl
	servicesClient := clientset.CoreV1().Services(m.namespace)
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", exposedByLabel, "wormhole"),
	}
	services, listErr := servicesClient.List(context.Background(), listOptions)
	if listErr != nil {
		return listErr
	}
	for _, service := range services.Items {
		deleteErr := servicesClient.Delete(context.Background(), service.Name, metav1.DeleteOptions{})
		if deleteErr != nil {
			return fmt.Errorf("Could not delete service %s: %v", service.Name, deleteErr)
		}
		logrus.Infof("Deleted service %s", service.Name)
	}
	return nil
}

func newManagedK8sService(namespace string, selectors map[string]string) managedK8sResource {
	return &managedK8sService{
		namespace: namespace,
		selectors: selectors,
	}
}
