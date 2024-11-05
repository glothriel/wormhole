package k8s

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

type managedK8sNetworkPolicy struct {
	namespace string
	selectors map[string]string
}

const consumesNpLabel = "wormhole.glothriel.github.com/network-policy-consumes-app"

func (m *managedK8sNetworkPolicy) Add(metadata k8sResourceMetadata, clientset *kubernetes.Clientset) error {
	networkPoliciesClient := clientset.NetworkingV1().NetworkPolicies(m.namespace)
	port, portErr := extractPortFromAddr(metadata.afterExposedApp.Address)
	if portErr != nil {
		return portErr
	}
	np := m.npDefinition(port, metadata)
	var upsertErr error
	previousNP, getErr := networkPoliciesClient.Get(context.Background(), metadata.entityName, metav1.GetOptions{})
	if errors.IsNotFound(getErr) {
		logrus.Infof("Creating network policy %s", metadata.entityName)
		_, upsertErr = networkPoliciesClient.Create(context.Background(), np, metav1.CreateOptions{})
	} else if getErr != nil {
		return getErr
	} else {
		logrus.Infof("Updating network policy %s", metadata.entityName)
		np.SetResourceVersion(previousNP.GetResourceVersion())
		_, upsertErr = networkPoliciesClient.Update(context.Background(), np, metav1.UpdateOptions{})
	}
	if upsertErr != nil {
		return upsertErr
	}
	return nil
}

func (m *managedK8sNetworkPolicy) npDefinition(port int, metadata k8sResourceMetadata) *networkingv1.NetworkPolicy {
	protoTCP := v1.ProtocolTCP
	convertedPort := intstr.FromInt(port)

	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      metadata.entityName,
			Namespace: m.namespace,
			Labels:    resourceLabels(metadata.afterExposedApp),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: m.selectors,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &protoTCP,
							Port:     &convertedPort,
						},
					},
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									consumesNpLabel: metadata.originalApp.Name,
								},
							},
							NamespaceSelector: &metav1.LabelSelector{},
						},
					},
				},
			},
		},
	}
}

func (m *managedK8sNetworkPolicy) Remove(entityName string, clientset *kubernetes.Clientset) error {
	networkPoliciesClient := clientset.NetworkingV1().NetworkPolicies(m.namespace)
	deleteErr := networkPoliciesClient.Delete(context.Background(), entityName, metav1.DeleteOptions{})
	if deleteErr != nil {
		return fmt.Errorf("Could not delete network policy %s: %v", entityName, deleteErr)
	}
	logrus.Infof("Deleted network policy %s", entityName)
	return nil
}

func (m *managedK8sNetworkPolicy) RemoveAll(clientset *kubernetes.Clientset) error { // nolint:dupl
	networkPoliciesClient := clientset.NetworkingV1().NetworkPolicies(m.namespace)
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", exposedByLabel, "wormhole"),
	}
	nps, listErr := networkPoliciesClient.List(context.Background(), listOptions)
	if listErr != nil {
		return listErr
	}
	for _, np := range nps.Items {
		deleteErr := networkPoliciesClient.Delete(context.Background(), np.Name, metav1.DeleteOptions{})
		if deleteErr != nil {
			return fmt.Errorf("Could not delete network policy %s: %v", np.Name, deleteErr)
		}
		logrus.Infof("Deleted network policy %s", np.Name)
	}
	return nil
}

func newManagedK8sNetworkPolicy(namespace string, selectors map[string]string) managedK8sResource {
	return &managedK8sNetworkPolicy{
		namespace: namespace,
		selectors: selectors,
	}
}
