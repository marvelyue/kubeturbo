package dtofactory

import (
	"github.com/stretchr/testify/assert"
	"github.com/turbonomic/kubeturbo/pkg/discovery/repository"
	"github.com/turbonomic/kubeturbo/pkg/discovery/worker/aggregation"
	"github.com/turbonomic/turbo-go-sdk/pkg/proto"
	"testing"
)

func Test_containerSpecDTOBuilder_getCommoditiesSold(t *testing.T) {
	namespace := "namespace"
	controllerUID := "controllerUID"
	containerSpecName := "containerSpecName"
	containerSpecId := "containerSpecId"
	containerSpecs := repository.ContainerSpec{
		Namespace:         namespace,
		ControllerUID:     controllerUID,
		ContainerSpecName: containerSpecName,
		ContainerSpecId:   containerSpecId,
		ContainerReplicas: 2,
		ContainerCommodities: map[proto.CommodityDTO_CommodityType][]*proto.CommodityDTO{
			cpuCommType: {
				createCommodityDTO(proto.CommodityDTO_VCPU, 1.0, 1.0, 2.0),
				createCommodityDTO(proto.CommodityDTO_VCPU, 2.0, 2.0, 3.0),
			},
			memCommType: {
				createCommodityDTO(proto.CommodityDTO_VMEM, 1.0, 1.0, 2.0),
				createCommodityDTO(proto.CommodityDTO_VMEM, 2.0, 2.0, 3.0),
			},
		},
	}

	builder := &containerSpecDTOBuilder{
		containerSpecMap:                   map[string]*repository.ContainerSpec{containerSpecId: &containerSpecs},
		containerUtilizationDataAggregator: aggregation.ContainerUtilizationDataAggregators[aggregation.DefaultContainerUtilizationDataAggStrategy],
		containerUsageDataAggregator:       aggregation.ContainerUsageDataAggregators[aggregation.DefaultContainerUsageDataAggStrategy],
	}
	commodityDTOs, err := builder.getCommoditiesSold(&containerSpecs)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(commodityDTOs))
	for _, commodityDTO := range commodityDTOs {
		assert.Equal(t, false, *commodityDTO.Active)
		// TODO test utilizationData and usage data
	}
}

func createCommodityDTO(commodityType proto.CommodityDTO_CommodityType, used, peak, capacity float64) *proto.CommodityDTO {
	return &proto.CommodityDTO{
		CommodityType: &commodityType,
		Used:          &used,
		Peak:          &peak,
		Capacity:      &capacity,
	}
}