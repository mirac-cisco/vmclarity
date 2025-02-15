// Copyright © 2023 Cisco Systems, Inc. and its affiliates.
// All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"

	"github.com/openclarity/vmclarity/api/models"
	"github.com/openclarity/vmclarity/runtime_scan/pkg/provider"
	"github.com/openclarity/vmclarity/shared/pkg/utils"
)

const (
	instanceIDPartsLength = 9
	resourceGroupPartIdx  = 4
	vmNamePartIdx         = 8
)

type Client struct {
	cred             azcore.TokenCredential
	rgClient         *armresources.ResourceGroupsClient
	vmClient         *armcompute.VirtualMachinesClient
	snapshotsClient  *armcompute.SnapshotsClient
	disksClient      *armcompute.DisksClient
	interfacesClient *armnetwork.InterfacesClient

	azureConfig Config
}

func New(_ context.Context) (*Client, error) {
	config, err := NewConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	err = config.Validate()
	if err != nil {
		return nil, fmt.Errorf("failed to validate configuration: %w", err)
	}

	client := Client{
		azureConfig: config,
	}

	cred, err := azidentity.NewManagedIdentityCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed create managed identity credential: %w", err)
	}
	client.cred = cred

	client.rgClient, err = armresources.NewResourceGroupsClient(config.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource group client: %w", err)
	}

	networkClientFactory, err := armnetwork.NewClientFactory(config.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create network client factory: %w", err)
	}
	client.interfacesClient = networkClientFactory.NewInterfacesClient()

	computeClientFactory, err := armcompute.NewClientFactory(config.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute client factory: %w", err)
	}
	client.vmClient = computeClientFactory.NewVirtualMachinesClient()
	client.disksClient = computeClientFactory.NewDisksClient()
	client.snapshotsClient = computeClientFactory.NewSnapshotsClient()

	return &client, nil
}

func (c Client) Kind() models.CloudProvider {
	return models.Azure
}

// nolint:cyclop
func (c *Client) RunTargetScan(ctx context.Context, config *provider.ScanJobConfig) error {
	vmInfo, err := config.TargetInfo.AsVMInfo()
	if err != nil {
		return provider.FatalErrorf("unable to get vminfo from target: %w", err)
	}

	resourceGroup, vmName, err := resourceGroupAndNameFromInstanceID(vmInfo.InstanceID)
	if err != nil {
		return err
	}

	targetVM, err := c.vmClient.Get(ctx, resourceGroup, vmName, nil)
	if err != nil {
		_, err = handleAzureRequestError(err, "getting target virtual machine %s", vmName)
		return err
	}

	snapshot, err := c.ensureSnapshotForVMRootVolume(ctx, config, targetVM.VirtualMachine)
	if err != nil {
		return fmt.Errorf("failed to ensure snapshot for vm root volume: %w", err)
	}

	var disk armcompute.Disk
	if *targetVM.Location == c.azureConfig.ScannerLocation {
		disk, err = c.ensureManagedDiskFromSnapshot(ctx, config, snapshot)
		if err != nil {
			return fmt.Errorf("failed to ensure managed disk created from snapshot: %w", err)
		}
	} else {
		disk, err = c.ensureManagedDiskFromSnapshotInDifferentRegion(ctx, config, snapshot)
		if err != nil {
			return fmt.Errorf("failed to ensure managed disk from snapshot in different region: %w", err)
		}
	}

	networkInterface, err := c.ensureNetworkInterface(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to ensure scanner network interface: %w", err)
	}

	scannerVM, err := c.ensureScannerVirtualMachine(ctx, config, networkInterface)
	if err != nil {
		return fmt.Errorf("failed to ensure scanner virtual machine: %w", err)
	}

	err = c.ensureDiskAttachedToScannerVM(ctx, scannerVM, disk)
	if err != nil {
		return fmt.Errorf("failed to ensure target disk is attached to virtual machine: %w", err)
	}

	return nil
}

func (c *Client) RemoveTargetScan(ctx context.Context, config *provider.ScanJobConfig) error {
	err := c.ensureScannerVirtualMachineDeleted(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to ensure scanner virtual machine deleted: %w", err)
	}

	err = c.ensureNetworkInterfaceDeleted(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to ensure network interface deleted: %w", err)
	}

	err = c.ensureTargetDiskDeleted(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to ensure target disk deleted: %w", err)
	}

	err = c.ensureBlobDeleted(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to ensure snapshot copy blob deleted: %w", err)
	}

	err = c.ensureSnapshotDeleted(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to ensure snapshot deleted: %w", err)
	}

	return nil
}

func (c *Client) DiscoverScopes(ctx context.Context) (*models.Scopes, error) {
	var ret models.Scopes
	ret.ScopeInfo = &models.ScopeType{}
	resourceGroups := []models.AzureResourceGroup{}

	// discover all resource groups in the user subscription
	res := c.rgClient.NewListPager(nil)
	for res.More() {
		page, err := res.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get next page: %w", err)
		}
		for _, rg := range page.Value {
			resourceGroups = append(resourceGroups, models.AzureResourceGroup{Name: *rg.Name})
		}
	}

	err := ret.ScopeInfo.FromAzureSubscriptionScope(models.AzureSubscriptionScope{
		ResourceGroups: &resourceGroups,
		SubscriptionID: &c.azureConfig.SubscriptionID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to convert from azure subscription scope: %v", err)
	}

	return &ret, nil
}

// nolint: cyclop
func (c *Client) DiscoverTargets(ctx context.Context, scanScope *models.ScanScopeType) ([]models.TargetType, error) {
	var ret []models.TargetType

	azureScanScope, err := scanScope.AsAzureScanScope()
	if err != nil {
		return nil, fmt.Errorf("failed to convert as azure scan scope: %v", err)
	}

	if azureScanScope.AllResourceGroups != nil && *azureScanScope.AllResourceGroups {
		// list all vms in all resourceGroups in the subscription
		res := c.vmClient.NewListAllPager(nil)
		for res.More() {
			page, err := res.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get next page: %w", err)
			}
			ts, err := processVirtualMachineListIntoTargetTypes(page.VirtualMachineListResult, azureScanScope)
			if err != nil {
				return nil, err
			}
			ret = append(ret, ts...)
		}
		return ret, nil
	}

	// if scan scope is only for specific resource groups and not all:
	for _, resourceGroup := range *azureScanScope.ResourceGroups {
		res := c.vmClient.NewListPager(resourceGroup.Name, nil)
		for res.More() {
			page, err := res.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get next page: %w", err)
			}
			ts, err := processVirtualMachineListIntoTargetTypes(page.VirtualMachineListResult, azureScanScope)
			if err != nil {
				return nil, err
			}
			ret = append(ret, ts...)
		}
	}
	return ret, nil
}

// Example Instance ID:
//
// /subscriptions/ecad88af-09d5-4725-8d80-906e51fddf02/resourceGroups/vmclarity-sambetts-dev/providers/Microsoft.Compute/virtualMachines/vmclarity-server
//
// Will return "vmclarity-sambetts-dev" and "vmclarity-server".
func resourceGroupAndNameFromInstanceID(instanceID string) (string, string, error) {
	idParts := strings.Split(instanceID, "/")
	if len(idParts) != instanceIDPartsLength {
		return "", "", provider.FatalErrorf("asset instance id in unexpected format got: %s", idParts)
	}
	return idParts[resourceGroupPartIdx], idParts[vmNamePartIdx], nil
}

func processVirtualMachineListIntoTargetTypes(vmList armcompute.VirtualMachineListResult, azureScanScope models.AzureScanScope) ([]models.TargetType, error) {
	ret := make([]models.TargetType, 0, len(vmList.Value))
	for _, vm := range vmList.Value {
		// filter by tags:
		if !hasIncludeTags(vm, azureScanScope.InstanceTagSelector) {
			continue
		}
		if hasExcludeTags(vm, azureScanScope.InstanceTagExclusion) {
			continue
		}
		info, err := getVMInfoFromVirtualMachine(vm)
		if err != nil {
			return nil, fmt.Errorf("unable to convert instance to vminfo: %w", err)
		}
		ret = append(ret, info)
	}
	return ret, nil
}

func getVMInfoFromVirtualMachine(vm *armcompute.VirtualMachine) (models.TargetType, error) {
	targetType := models.TargetType{}
	err := targetType.FromVMInfo(models.VMInfo{
		ObjectType:       "VMInfo",
		InstanceProvider: utils.PointerTo(models.Azure),
		InstanceID:       *vm.ID,
		Image:            createImageURN(vm.Properties.StorageProfile.ImageReference),
		InstanceType:     *vm.Type,
		LaunchTime:       *vm.Properties.TimeCreated,
		Location:         *vm.Location,
		Platform:         string(*vm.Properties.StorageProfile.OSDisk.OSType),
		SecurityGroups:   &[]models.SecurityGroup{},
		Tags:             convertTags(vm.Tags),
	})
	if err != nil {
		err = fmt.Errorf("failed to create TargetType from VMInfo: %w", err)
	}

	return targetType, err
}

// AND logic - if tags = {tag1:val1, tag2:val2},
// then a vm will be excluded/included only if it has ALL of these tags ({tag1:val1, tag2:val2}).
func hasIncludeTags(vm *armcompute.VirtualMachine, tags *[]models.Tag) bool {
	if tags == nil {
		return true
	}
	if len(*tags) == 0 {
		return true
	}
	if len(vm.Tags) == 0 {
		return false
	}

	for _, tag := range *tags {
		val, ok := vm.Tags[tag.Key]
		if !ok {
			return false
		}
		if !(strings.Compare(*val, tag.Value) == 0) {
			return false
		}
	}
	return true
}

// AND logic - if tags = {tag1:val1, tag2:val2},
// then a vm will be excluded/included only if it has ALL of these tags ({tag1:val1, tag2:val2}).
func hasExcludeTags(vm *armcompute.VirtualMachine, tags *[]models.Tag) bool {
	if tags == nil {
		return false
	}
	if len(*tags) == 0 {
		return false
	}
	if len(vm.Tags) == 0 {
		return false
	}

	for _, tag := range *tags {
		val, ok := vm.Tags[tag.Key]
		if !ok {
			return false
		}
		if !(strings.Compare(*val, tag.Value) == 0) {
			return false
		}
	}
	return true
}

func convertTags(tags map[string]*string) *[]models.Tag {
	ret := make([]models.Tag, 0, len(tags))
	for key, val := range tags {
		ret = append(ret, models.Tag{
			Key:   key,
			Value: *val,
		})
	}
	return &ret
}

// https://learn.microsoft.com/en-us/azure/virtual-machines/linux/tutorial-manage-vm#understand-vm-images
func createImageURN(reference *armcompute.ImageReference) string {
	// ImageReference is required only when using platform images, marketplace images, or
	// virtual machine images, but is not used in other creation operations (like managed disks).
	if reference == nil {
		return ""
	}
	return *reference.Publisher + "/" + *reference.Offer + "/" + *reference.SKU + "/" + *reference.Version
}
