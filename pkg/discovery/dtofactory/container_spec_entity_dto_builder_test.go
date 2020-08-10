package dtofactory

import (
	"github.com/stretchr/testify/assert"
	"github.com/turbonomic/kubeturbo/pkg/discovery/metrics"
	"github.com/turbonomic/kubeturbo/pkg/discovery/repository"
	"github.com/turbonomic/kubeturbo/pkg/discovery/worker/aggregation"
	"testing"
)

func Test_containerSpecDTOBuilder_getCommoditiesSold(t *testing.T) {
	namespace := "namespace"
	controllerUID := "controllerUID"
	containerSpecName := "containerSpecName"
	containerSpecId := "containerSpecId"
	containerSpecMetrics := repository.ContainerSpecMetrics{
		Namespace:         namespace,
		ControllerUID:     controllerUID,
		ContainerSpecName: containerSpecName,
		ContainerSpecId:   containerSpecId,
		ContainerReplicas: 2,
		ContainerMetrics: map[metrics.ResourceType]*repository.ContainerMetrics{
			metrics.CPU: {
				Capacity: 4.0,
				Used: []metrics.Point{
					createContainerMetricPoint(1.0, 1),
					createContainerMetricPoint(3.0, 2),
				},
			},
			metrics.Memory: {
				Capacity: 4.0,
				Used: []metrics.Point{
					createContainerMetricPoint(1.0, 1),
					createContainerMetricPoint(3.0, 2),
				},
			},
			metrics.MemoryRequest: {
				Capacity: 4.0,
				Used: []metrics.Point{
					createContainerMetricPoint(1.0, 1),
					createContainerMetricPoint(3.0, 2),
				},
			},
		},
	}

	builder := &containerSpecDTOBuilder{
		containerSpecMetricsMap:            map[string]*repository.ContainerSpecMetrics{containerSpecId: &containerSpecMetrics},
		containerUtilizationDataAggregator: aggregation.ContainerUtilizationDataAggregators[aggregation.DefaultContainerUtilizationDataAggStrategy],
		containerUsageDataAggregator:       aggregation.ContainerUsageDataAggregators[aggregation.DefaultContainerUsageDataAggStrategy],
	}
	commodityDTOs, err := builder.getCommoditiesSold(&containerSpecMetrics)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(commodityDTOs))
	for _, commodityDTO := range commodityDTOs {
		assert.Equal(t, true, *commodityDTO.Active)
		assert.Equal(t, true, *commodityDTO.Resizable)
		// Parse values to int to avoid tolerance of float values
		assert.Equal(t, 2, int(*commodityDTO.Used))
		assert.Equal(t, 3, int(*commodityDTO.Peak))
		assert.Equal(t, 4, int(*commodityDTO.Capacity))
		assert.Equal(t, 2, len(commodityDTO.UtilizationData.Point))
	}
}

func createContainerMetricPoint(value float64, timestamp int64) metrics.Point {
	return metrics.Point{
		Value:     value,
		Timestamp: timestamp,
	}
}
