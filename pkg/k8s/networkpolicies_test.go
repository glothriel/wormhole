package k8s

import (
	"testing"

	"github.com/glothriel/wormhole/pkg/apps"
	"github.com/stretchr/testify/assert"
)

func TestConsumesNpLabelKey(t *testing.T) {
	tests := []struct {
		name     string
		appName  string
		expected func(string) bool
	}{
		{
			name:    "short app name",
			appName: "nginx",
			expected: func(result string) bool {
				return result == "consumes.wormhole.glothriel.github.com/nginx"
			},
		},
		{
			name:    "app name with hyphen",
			appName: "default-nginx",
			expected: func(result string) bool {
				return result == "consumes.wormhole.glothriel.github.com/default-nginx"
			},
		},
		{
			name:    "very long app name gets hashed",
			appName: "alpha-beta-gamma-delta-epsilon-zeta-eta-iota-kappa-lambda-mi-ni-xi-omikron-pi-rho-sigma",
			expected: func(result string) bool {
				// Should have the prefix and a hashed suffix
				if !assert.ObjectsAreEqual(result[:len(consumesNpLabelPrefix)], consumesNpLabelPrefix) {
					return false
				}
				labelPart := result[len(consumesNpLabelPrefix):]
				// Should be at most 63 chars and contain a hyphen followed by 8-char hash
				if len(labelPart) > 63 {
					return false
				}
				parts := result[len(consumesNpLabelPrefix):]
				hyphenPos := len(parts) - 9
				if hyphenPos < 0 || parts[hyphenPos] != '-' {
					return false
				}
				return true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := consumesNpLabelKey(tt.appName)
			assert.True(t, tt.expected(result))
		})
	}
}

func TestNpDefinitionDualFormat(t *testing.T) {
	// given
	m := &managedK8sNetworkPolicy{
		namespace: "test-ns",
		selectors: map[string]string{"app": "wormhole-client"},
	}

	metadata := k8sResourceMetadata{
		entityName: "client-nginx-nginx",
		originalApp: apps.App{
			Name:         "nginx-nginx",
			Peer:         "client",
			OriginalPort: 80,
		},
		afterExposedApp: apps.App{
			Name:         "nginx-nginx",
			Peer:         "client",
			OriginalPort: 80,
			Address:      "client-nginx-nginx.test-ns:25001",
		},
	}

	// when
	np := m.npDefinition(25001, metadata)

	// then
	assert.NotNil(t, np)
	assert.Len(t, np.Spec.Ingress, 1)
	assert.Len(t, np.Spec.Ingress[0].From, 2, "should have two From entries for dual format support")

	// Check new format entry
	newFormatPeer := np.Spec.Ingress[0].From[0]
	assert.NotNil(t, newFormatPeer.PodSelector)
	assert.Equal(t, "true", newFormatPeer.PodSelector.MatchLabels[consumesNpLabelKey("nginx-nginx")])

	// Check old format entry
	oldFormatPeer := np.Spec.Ingress[0].From[1]
	assert.NotNil(t, oldFormatPeer.PodSelector)
	assert.Equal(t, "nginx-nginx", oldFormatPeer.PodSelector.MatchLabels[consumesNpLabel])
}
