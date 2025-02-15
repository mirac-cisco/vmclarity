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

package database

import (
	"context"
	"encoding/json"
	"time"

	"golang.org/x/exp/rand"

	"github.com/openclarity/vmclarity/api/models"
	"github.com/openclarity/vmclarity/backend/pkg/database/types"
	"github.com/openclarity/vmclarity/shared/pkg/log"
	"github.com/openclarity/vmclarity/shared/pkg/utils"
)

const (
	awsRegionEUCentral1    = "eu-central-1"
	awsRegionUSEast1       = "us-east-1"
	awsVPCEUCentral11      = "vpc-1-from-eu-central-1"
	awsVPCEUCentral12      = "vpc-2-from-eu-central-1"
	awsVPCUSEast11         = "vpc-1-from-us-east-1"
	awsVPCUSEast12         = "vpc-2-from-us-east-1"
	awsSGUSEast111         = "sg-1-from-vpc-1-from-us-east-1"
	awsSGUSEast121         = "sg-1-from-vpc-2-from-us-east-1"
	awsSGUSEast122         = "sg-2-from-vpc-2-from-us-east-1"
	awsSGEUCentral111      = "sg-1-from-vpc-1-from-eu-central-1"
	awsSGEUCentral121      = "sg-1-from-vpc-2-from-eu-central-1"
	awsInstanceEUCentral11 = "i-instance-1-from-eu-central-1"
	awsInstanceEUCentral12 = "i-instance-2-from-eu-central-1"
	awsInstanceUSEast11    = "i-instance-1-from-us-east-1"
)

var regions = []models.AwsRegion{
	{
		Name: awsRegionEUCentral1,
		Vpcs: utils.PointerTo([]models.AwsVPC{
			{
				Id: awsVPCEUCentral11,
				SecurityGroups: utils.PointerTo([]models.SecurityGroup{
					{
						Id: awsSGEUCentral111,
					},
				}),
			},
			{
				Id: awsVPCEUCentral12,
				SecurityGroups: utils.PointerTo([]models.SecurityGroup{
					{
						Id: awsSGEUCentral121,
					},
				}),
			},
		}),
	},
	{
		Name: awsRegionUSEast1,
		Vpcs: utils.PointerTo([]models.AwsVPC{
			{
				Id: awsVPCUSEast11,
				SecurityGroups: utils.PointerTo([]models.SecurityGroup{
					{
						Id: awsSGUSEast111,
					},
				}),
			},
			{
				Id: awsVPCUSEast12,
				SecurityGroups: utils.PointerTo([]models.SecurityGroup{
					{
						Id: awsSGUSEast121,
					},
					{
						Id: awsSGUSEast122,
					},
				}),
			},
		}),
	},
}

// nolint:gomnd,maintidx,cyclop
func CreateDemoData(ctx context.Context, db types.Database) {
	logger := log.GetLoggerFromContextOrDiscard(ctx)

	// Create scopes:
	scopes, err := createScopes()
	if err != nil {
		logger.Fatalf("failed to create scopes FromAwsScope: %v", err)
	}
	if _, err := db.ScopesTable().SetScopes(scopes); err != nil {
		logger.Fatalf("failed to save scopes: %v", err)
	}

	// Create scan configs:
	scanConfigs := createScanConfigs(ctx)
	for i, scanConfig := range scanConfigs {
		ret, err := db.ScanConfigsTable().CreateScanConfig(scanConfig)
		if err != nil {
			logger.Fatalf("failed to create scan config [%d]: %v", i, err)
		}
		scanConfigs[i] = ret
	}

	// Create targets:
	targets := createTargets()
	for i, target := range targets {
		retTarget, err := db.TargetsTable().CreateTarget(target)
		if err != nil {
			logger.Fatalf("failed to create target [%d]: %v", i, err)
		}
		targets[i] = retTarget
	}

	// Create scans:
	scans := createScans(targets, scanConfigs)
	for i, scan := range scans {
		ret, err := db.ScansTable().CreateScan(scan)
		if err != nil {
			logger.Fatalf("failed to create scan [%d]: %v", i, err)
		}
		scans[i] = ret
	}

	// Create scan results:
	scanResults := createScanResults(scans)
	for i, scanResult := range scanResults {
		ret, err := db.ScanResultsTable().CreateScanResult(scanResult)
		if err != nil {
			logger.Fatalf("failed to create scan result [%d]: %v", i, err)
		}
		scanResults[i] = ret
	}

	// Create findings
	findings := createFindings(ctx, scanResults)
	for i, finding := range findings {
		ret, err := db.FindingsTable().CreateFinding(finding)
		if err != nil {
			logger.Fatalf("failed to create finding [%d]: %v", i, err)
		}
		findings[i] = ret
	}
}

// nolint:gocognit,prealloc,cyclop
func createFindings(ctx context.Context, scanResults []models.TargetScanResult) []models.Finding {
	var ret []models.Finding
	rand.Seed(uint64(time.Now().Unix()))

	for _, scanResult := range scanResults {
		var foundOn *time.Time
		if scanResult.Scan.StartTime != nil {
			foundOn = scanResult.Scan.StartTime
		} else {
			randMin := rand.Intn(59) + 1
			foundOn = utils.PointerTo(time.Now().Add(time.Duration(-randMin) * time.Minute))
		}
		findingBase := models.Finding{
			Asset: &models.TargetRelationship{
				Id: scanResult.Target.Id,
			},
			FindingInfo: nil,
			FoundOn:     foundOn,
			// InvalidatedOn: utils.PointerTo(foundOn.Add(2 * time.Minute)),
			Scan: &models.ScanRelationship{
				Id: scanResult.Scan.Id,
			},
		}
		if scanResult.Sboms != nil && scanResult.Sboms.Packages != nil {
			ret = append(ret, createPackageFindings(ctx, findingBase, *scanResult.Sboms.Packages)...)
		}
		if scanResult.Vulnerabilities != nil && scanResult.Vulnerabilities.Vulnerabilities != nil {
			ret = append(ret, createVulnerabilityFindings(ctx, findingBase, *scanResult.Vulnerabilities.Vulnerabilities)...)
		}
		if scanResult.Exploits != nil && scanResult.Exploits.Exploits != nil {
			ret = append(ret, createExploitFindings(ctx, findingBase, *scanResult.Exploits.Exploits)...)
		}
		if scanResult.Malware != nil && scanResult.Malware.Malware != nil {
			ret = append(ret, createMalwareFindings(ctx, findingBase, *scanResult.Malware.Malware)...)
		}
		if scanResult.Secrets != nil && scanResult.Secrets.Secrets != nil {
			ret = append(ret, createSecretFindings(ctx, findingBase, *scanResult.Secrets.Secrets)...)
		}
		if scanResult.Misconfigurations != nil && scanResult.Misconfigurations.Misconfigurations != nil {
			ret = append(ret, createMisconfigurationFindings(ctx, findingBase, *scanResult.Misconfigurations.Misconfigurations)...)
		}
		if scanResult.Rootkits != nil && scanResult.Rootkits.Rootkits != nil {
			ret = append(ret, createRootkitFindings(ctx, findingBase, *scanResult.Rootkits.Rootkits)...)
		}
	}

	return ret
}

// nolint:gocognit,prealloc
func createExploitFindings(ctx context.Context, base models.Finding, exploits []models.Exploit) []models.Finding {
	logger := log.GetLoggerFromContextOrDiscard(ctx)

	var ret []models.Finding
	for _, exploit := range exploits {
		val := base
		convB, err := json.Marshal(exploit)
		if err != nil {
			logger.Errorf("Failed to marshal: %v", err)
			continue
		}
		conv := models.ExploitFindingInfo{}
		err = json.Unmarshal(convB, &conv)
		if err != nil {
			logger.Errorf("Failed to unmarshal: %v", err)
			continue
		}
		val.FindingInfo = &models.Finding_FindingInfo{}
		err = val.FindingInfo.FromExploitFindingInfo(conv)
		if err != nil {
			logger.Errorf("Failed to convert FromExploitFindingInfo: %v", err)
			continue
		}
		ret = append(ret, val)
	}

	return ret
}

// nolint:gocognit,prealloc
func createPackageFindings(ctx context.Context, base models.Finding, packages []models.Package) []models.Finding {
	logger := log.GetLoggerFromContextOrDiscard(ctx)

	var ret []models.Finding
	for _, pkg := range packages {
		val := base
		convB, err := json.Marshal(pkg)
		if err != nil {
			logger.Errorf("Failed to marshal: %v", err)
			continue
		}
		conv := models.PackageFindingInfo{}
		err = json.Unmarshal(convB, &conv)
		if err != nil {
			logger.Errorf("Failed to unmarshal: %v", err)
			continue
		}
		val.FindingInfo = &models.Finding_FindingInfo{}
		err = val.FindingInfo.FromPackageFindingInfo(conv)
		if err != nil {
			logger.Errorf("Failed to convert FromPackageFindingInfo: %v", err)
			continue
		}
		ret = append(ret, val)
	}

	return ret
}

// nolint:gocognit,prealloc
func createMalwareFindings(ctx context.Context, base models.Finding, malware []models.Malware) []models.Finding {
	logger := log.GetLoggerFromContextOrDiscard(ctx)

	var ret []models.Finding
	for _, mal := range malware {
		val := base
		convB, err := json.Marshal(mal)
		if err != nil {
			logger.Errorf("Failed to marshal: %v", err)
			continue
		}
		conv := models.MalwareFindingInfo{}
		err = json.Unmarshal(convB, &conv)
		if err != nil {
			logger.Errorf("Failed to unmarshal: %v", err)
			continue
		}
		val.FindingInfo = &models.Finding_FindingInfo{}
		err = val.FindingInfo.FromMalwareFindingInfo(conv)
		if err != nil {
			logger.Errorf("Failed to convert FromMalwareFindingInfo: %v", err)
			continue
		}
		ret = append(ret, val)
	}

	return ret
}

// nolint:gocognit,prealloc
func createSecretFindings(ctx context.Context, base models.Finding, secrets []models.Secret) []models.Finding {
	logger := log.GetLoggerFromContextOrDiscard(ctx)

	var ret []models.Finding
	for _, secret := range secrets {
		val := base
		convB, err := json.Marshal(secret)
		if err != nil {
			logger.Errorf("Failed to marshal: %v", err)
			continue
		}
		conv := models.SecretFindingInfo{}
		err = json.Unmarshal(convB, &conv)
		if err != nil {
			logger.Errorf("Failed to unmarshal: %v", err)
			continue
		}
		val.FindingInfo = &models.Finding_FindingInfo{}
		err = val.FindingInfo.FromSecretFindingInfo(conv)
		if err != nil {
			logger.Errorf("Failed to convert FromSecretFindingInfo: %v", err)
			continue
		}
		ret = append(ret, val)
	}

	return ret
}

// nolint:gocognit,prealloc
func createMisconfigurationFindings(ctx context.Context, base models.Finding, misconfigurations []models.Misconfiguration) []models.Finding {
	logger := log.GetLoggerFromContextOrDiscard(ctx)

	var ret []models.Finding
	for _, misconfiguration := range misconfigurations {
		val := base
		convB, err := json.Marshal(misconfiguration)
		if err != nil {
			logger.Errorf("Failed to marshal: %v", err)
			continue
		}
		conv := models.MisconfigurationFindingInfo{}
		err = json.Unmarshal(convB, &conv)
		if err != nil {
			logger.Errorf("Failed to unmarshal: %v", err)
			continue
		}
		val.FindingInfo = &models.Finding_FindingInfo{}
		err = val.FindingInfo.FromMisconfigurationFindingInfo(conv)
		if err != nil {
			logger.Errorf("Failed to convert FromMisconfigurationFindingInfo: %v", err)
			continue
		}
		ret = append(ret, val)
	}

	return ret
}

// nolint:gocognit,prealloc
func createRootkitFindings(ctx context.Context, base models.Finding, rootkits []models.Rootkit) []models.Finding {
	logger := log.GetLoggerFromContextOrDiscard(ctx)

	var ret []models.Finding
	for _, rootkit := range rootkits {
		val := base
		convB, err := json.Marshal(rootkit)
		if err != nil {
			logger.Errorf("Failed to marshal: %v", err)
			continue
		}
		conv := models.RootkitFindingInfo{}
		err = json.Unmarshal(convB, &conv)
		if err != nil {
			logger.Errorf("Failed to unmarshal: %v", err)
			continue
		}
		val.FindingInfo = &models.Finding_FindingInfo{}
		err = val.FindingInfo.FromRootkitFindingInfo(conv)
		if err != nil {
			logger.Errorf("Failed to convert FromRootkitFindingInfo: %v", err)
			continue
		}
		ret = append(ret, val)
	}

	return ret
}

// nolint:gocognit,prealloc
func createVulnerabilityFindings(ctx context.Context, base models.Finding, vulnerabilities []models.Vulnerability) []models.Finding {
	logger := log.GetLoggerFromContextOrDiscard(ctx)

	var ret []models.Finding
	for _, vulnerability := range vulnerabilities {
		val := base
		convB, err := json.Marshal(vulnerability)
		if err != nil {
			logger.Errorf("Failed to marshal: %v", err)
			continue
		}
		conv := models.VulnerabilityFindingInfo{}
		err = json.Unmarshal(convB, &conv)
		if err != nil {
			logger.Errorf("Failed to unmarshal: %v", err)
			continue
		}
		val.FindingInfo = &models.Finding_FindingInfo{}
		err = val.FindingInfo.FromVulnerabilityFindingInfo(conv)
		if err != nil {
			logger.Errorf("Failed to convert FromVulnerabilityFindingInfo: %v", err)
			continue
		}
		ret = append(ret, val)
	}

	return ret
}

func createVMInfo(instanceID, location, image, instanceType, platform string,
	tags []models.Tag, launchTime time.Time, instanceProvider models.CloudProvider,
) *models.TargetType {
	info := models.TargetType{}
	err := info.FromVMInfo(models.VMInfo{
		Image:            image,
		InstanceID:       instanceID,
		InstanceProvider: &instanceProvider,
		InstanceType:     instanceType,
		LaunchTime:       launchTime,
		Location:         location,
		Platform:         platform,
		Tags:             &tags,
	})
	if err != nil {
		panic(err)
	}
	return &info
}

func createScopes() (models.Scopes, error) {
	scopesType := models.ScopeType{}
	err := scopesType.FromAwsAccountScope(models.AwsAccountScope{
		Regions: utils.PointerTo(regions),
	})
	// nolint:wrapcheck
	return models.Scopes{
		ScopeInfo: &scopesType,
	}, err
}

func createTargets() []models.Target {
	return []models.Target{
		{
			ScansCount: utils.PointerTo(1),
			Summary: &models.ScanFindingsSummary{
				TotalExploits:          utils.PointerTo(0),
				TotalMalware:           utils.PointerTo(0),
				TotalMisconfigurations: utils.PointerTo(0),
				TotalPackages:          utils.PointerTo(2),
				TotalRootkits:          utils.PointerTo(0),
				TotalSecrets:           utils.PointerTo(3),
				TotalVulnerabilities: &models.VulnerabilityScanSummary{
					TotalCriticalVulnerabilities:   utils.PointerTo(1),
					TotalHighVulnerabilities:       utils.PointerTo(1),
					TotalLowVulnerabilities:        utils.PointerTo(1),
					TotalMediumVulnerabilities:     utils.PointerTo(0),
					TotalNegligibleVulnerabilities: utils.PointerTo(0),
				},
			},
			TargetInfo: createVMInfo(awsInstanceEUCentral11, awsRegionEUCentral1+"/"+awsVPCEUCentral11+"/"+awsSGEUCentral111,
				"ami-111", "t2.large", "Linux", []models.Tag{{Key: "Name", Value: "target1"}}, time.Now(), models.AWS),
		},
		{
			ScansCount: utils.PointerTo(1),
			Summary: &models.ScanFindingsSummary{
				TotalExploits:          utils.PointerTo(0),
				TotalMalware:           utils.PointerTo(0),
				TotalMisconfigurations: utils.PointerTo(0),
				TotalPackages:          utils.PointerTo(2),
				TotalRootkits:          utils.PointerTo(0),
				TotalSecrets:           utils.PointerTo(3),
				TotalVulnerabilities: &models.VulnerabilityScanSummary{
					TotalCriticalVulnerabilities:   utils.PointerTo(1),
					TotalHighVulnerabilities:       utils.PointerTo(1),
					TotalLowVulnerabilities:        utils.PointerTo(1),
					TotalMediumVulnerabilities:     utils.PointerTo(0),
					TotalNegligibleVulnerabilities: utils.PointerTo(0),
				},
			},
			TargetInfo: createVMInfo(awsInstanceEUCentral12, awsRegionEUCentral1+"/"+awsVPCEUCentral11+"/"+awsSGEUCentral111,
				"ami-111", "t2.large", "Linux", []models.Tag{{Key: "Name", Value: "target2"}}, time.Now(), models.AWS),
		},
		{
			ScansCount: utils.PointerTo(1),
			Summary: &models.ScanFindingsSummary{
				TotalExploits:          utils.PointerTo(2),
				TotalMalware:           utils.PointerTo(3),
				TotalMisconfigurations: utils.PointerTo(3),
				TotalPackages:          utils.PointerTo(0),
				TotalRootkits:          utils.PointerTo(3),
				TotalSecrets:           utils.PointerTo(0),
				TotalVulnerabilities: &models.VulnerabilityScanSummary{
					TotalCriticalVulnerabilities:   utils.PointerTo(0),
					TotalHighVulnerabilities:       utils.PointerTo(0),
					TotalLowVulnerabilities:        utils.PointerTo(0),
					TotalMediumVulnerabilities:     utils.PointerTo(0),
					TotalNegligibleVulnerabilities: utils.PointerTo(0),
				},
			},
			TargetInfo: createVMInfo(awsInstanceUSEast11, awsRegionUSEast1+"/"+awsVPCUSEast11+"/"+awsSGUSEast111,
				"ami-112", "t2.micro", "Linux", []models.Tag{{Key: "Name", Value: "target3"}}, time.Now(), models.AWS),
		},
	}
}

func createScanConfigs(ctx context.Context) []models.ScanConfig {
	logger := log.GetLoggerFromContextOrDiscard(ctx)

	// Scan config 1
	scanFamiliesConfig1 := models.ScanFamiliesConfig{
		Exploits: &models.ExploitsConfig{
			Enabled: utils.PointerTo(false),
		},
		Malware: &models.MalwareConfig{
			Enabled: utils.PointerTo(false),
		},
		Misconfigurations: &models.MisconfigurationsConfig{
			Enabled: utils.PointerTo(false),
		},
		Rootkits: &models.RootkitsConfig{
			Enabled: utils.PointerTo(false),
		},
		Sbom: &models.SBOMConfig{
			Enabled: utils.PointerTo(true),
		},
		Secrets: &models.SecretsConfig{
			Enabled: utils.PointerTo(true),
		},
		Vulnerabilities: &models.VulnerabilitiesConfig{
			Enabled: utils.PointerTo(true),
		},
	}
	tag1 := models.Tag{
		Key:   "app",
		Value: "my-app1",
	}
	tag2 := models.Tag{
		Key:   "app",
		Value: "my-app2",
	}
	tag3 := models.Tag{
		Key:   "system",
		Value: "sys1",
	}
	tag4 := models.Tag{
		Key:   "system",
		Value: "sys2",
	}
	ScanConfig1SecurityGroups := []models.SecurityGroup{
		{
			Id: awsSGEUCentral111,
		},
	}
	ScanConfig1VPCs := []models.AwsVPC{
		{
			Id:             awsVPCEUCentral11,
			SecurityGroups: &ScanConfig1SecurityGroups,
		},
	}
	ScanConfig1Regions := []models.AwsRegion{
		{
			Name: awsRegionEUCentral1,
			Vpcs: &ScanConfig1VPCs,
		},
	}
	scanConfig1SelectorTags := []models.Tag{tag1, tag2}
	scanConfig1ExclusionTags := []models.Tag{tag3, tag4}
	scope1 := models.AwsScanScope{
		AllRegions:                 utils.PointerTo(false),
		InstanceTagExclusion:       &scanConfig1ExclusionTags,
		InstanceTagSelector:        &scanConfig1SelectorTags,
		ObjectType:                 "AwsScanScope",
		Regions:                    &ScanConfig1Regions,
		ShouldScanStoppedInstances: utils.PointerTo(false),
	}

	var scanScopeType1 models.ScanScopeType

	err := scanScopeType1.FromAwsScanScope(scope1)
	if err != nil {
		logger.Fatalf("failed to convert scope1: %v", err)
	}

	// Scan config 2
	scanFamiliesConfig2 := models.ScanFamiliesConfig{
		Exploits: &models.ExploitsConfig{
			Enabled: utils.PointerTo(true),
		},
		Malware: &models.MalwareConfig{
			Enabled: utils.PointerTo(true),
		},
		Misconfigurations: &models.MisconfigurationsConfig{
			Enabled: utils.PointerTo(true),
		},
		Rootkits: &models.RootkitsConfig{
			Enabled: utils.PointerTo(true),
		},
		Sbom: &models.SBOMConfig{
			Enabled: utils.PointerTo(false),
		},
		Secrets: &models.SecretsConfig{
			Enabled: utils.PointerTo(false),
		},
		Vulnerabilities: &models.VulnerabilitiesConfig{
			Enabled: utils.PointerTo(false),
		},
	}

	ScanConfig2SecurityGroups := []models.SecurityGroup{
		{
			Id: awsSGUSEast111,
		},
	}
	ScanConfig2VPCs := []models.AwsVPC{
		{
			Id:             awsVPCUSEast11,
			SecurityGroups: &ScanConfig2SecurityGroups,
		},
	}
	ScanConfig2Regions := []models.AwsRegion{
		{
			Name: awsRegionUSEast1,
			Vpcs: &ScanConfig2VPCs,
		},
	}
	scanConfig2SelectorTags := []models.Tag{tag2}
	scanConfig2ExclusionTags := []models.Tag{tag4}
	scanConfig2Scope := models.AwsScanScope{
		AllRegions:                 utils.PointerTo(false),
		InstanceTagExclusion:       &scanConfig2ExclusionTags,
		InstanceTagSelector:        &scanConfig2SelectorTags,
		ObjectType:                 "AwsScanScope",
		Regions:                    &ScanConfig2Regions,
		ShouldScanStoppedInstances: utils.PointerTo(true),
	}

	var scanScopeType2 models.ScanScopeType

	err = scanScopeType2.FromAwsScanScope(scanConfig2Scope)
	if err != nil {
		logger.Fatalf("failed to convert scanConfig2Scope: %v", err)
	}

	return []models.ScanConfig{
		{
			Name:               utils.PointerTo("Scan Config 1"),
			ScanFamiliesConfig: &scanFamiliesConfig1,
			Scheduled: &models.RuntimeScheduleScanConfig{
				OperationTime: utils.PointerTo(time.Now().Add(5 * time.Hour)),
			},
			Scope:               &scanScopeType1,
			MaxParallelScanners: utils.PointerTo(2),
		},
		{
			MaxParallelScanners: utils.PointerTo(3),
			Name:                utils.PointerTo("Scan Config 2"),
			ScanFamiliesConfig:  &scanFamiliesConfig2,
			ScannerInstanceCreationConfig: &models.ScannerInstanceCreationConfig{
				MaxPrice:         utils.PointerTo("1000000"),
				RetryMaxAttempts: utils.PointerTo(4),
				UseSpotInstances: true,
			},
			Scheduled: &models.RuntimeScheduleScanConfig{
				CronLine: utils.PointerTo("0 */4 * * *"),
			},
			Scope: &scanScopeType2,
		},
	}
}

func createScans(targets []models.Target, scanConfigs []models.ScanConfig) []models.Scan {
	// Create scan 1: already ended
	scan1Start := time.Now().Add(-10 * time.Hour)
	scan1End := scan1Start.Add(5*time.Hour + 27*time.Minute + 56*time.Second)
	scan1Targets := []string{*targets[0].Id, *targets[1].Id}

	scan1Summary := &models.ScanSummary{
		JobsCompleted:          utils.PointerTo[int](2),
		JobsLeftToRun:          utils.PointerTo[int](0),
		TotalExploits:          utils.PointerTo[int](0),
		TotalMalware:           utils.PointerTo[int](0),
		TotalMisconfigurations: utils.PointerTo[int](0),
		TotalPackages:          utils.PointerTo[int](4),
		TotalRootkits:          utils.PointerTo[int](0),
		TotalSecrets:           utils.PointerTo[int](6),
		TotalVulnerabilities: &models.VulnerabilityScanSummary{
			TotalCriticalVulnerabilities:   utils.PointerTo[int](2),
			TotalHighVulnerabilities:       utils.PointerTo[int](2),
			TotalLowVulnerabilities:        utils.PointerTo[int](2),
			TotalMediumVulnerabilities:     utils.PointerTo[int](0),
			TotalNegligibleVulnerabilities: utils.PointerTo[int](0),
		},
	}

	scan1ConfigSnapshot := &models.ScanConfigSnapshot{
		MaxParallelScanners: scanConfigs[0].MaxParallelScanners,
		Name:                utils.PointerTo[string]("Scan Config 1"),
		ScanFamiliesConfig:  scanConfigs[0].ScanFamiliesConfig,
		Scheduled:           scanConfigs[0].Scheduled,
		Scope:               scanConfigs[0].Scope,
	}

	// Create scan 2: Running
	scan2Start := time.Now().Add(-5 * time.Minute)
	scan2Targets := []string{*targets[2].Id}

	scan2Summary := &models.ScanSummary{
		JobsCompleted:          utils.PointerTo[int](1),
		JobsLeftToRun:          utils.PointerTo[int](1),
		TotalExploits:          utils.PointerTo[int](2),
		TotalMalware:           utils.PointerTo[int](3),
		TotalMisconfigurations: utils.PointerTo[int](3),
		TotalPackages:          utils.PointerTo[int](0),
		TotalRootkits:          utils.PointerTo[int](3),
		TotalSecrets:           utils.PointerTo[int](0),
		TotalVulnerabilities: &models.VulnerabilityScanSummary{
			TotalCriticalVulnerabilities:   utils.PointerTo[int](0),
			TotalHighVulnerabilities:       utils.PointerTo[int](0),
			TotalLowVulnerabilities:        utils.PointerTo[int](0),
			TotalMediumVulnerabilities:     utils.PointerTo[int](0),
			TotalNegligibleVulnerabilities: utils.PointerTo[int](0),
		},
	}

	scan2ConfigSnapshot := &models.ScanConfigSnapshot{
		MaxParallelScanners: scanConfigs[1].MaxParallelScanners,
		Name:                utils.PointerTo[string]("Scan Config 2"),
		ScanFamiliesConfig:  scanConfigs[1].ScanFamiliesConfig,
		Scheduled:           scanConfigs[1].Scheduled,
		Scope:               scanConfigs[1].Scope,
	}

	return []models.Scan{
		{
			EndTime: &scan1End,
			ScanConfig: &models.ScanConfigRelationship{
				Id: *scanConfigs[0].Id,
			},
			ScanConfigSnapshot: scan1ConfigSnapshot,
			StartTime:          &scan1Start,
			State:              utils.PointerTo(models.ScanStateDone),
			StateMessage:       utils.PointerTo("Scan was completed successfully"),
			StateReason:        utils.PointerTo(models.ScanStateReasonSuccess),
			Summary:            scan1Summary,
			TargetIDs:          &scan1Targets,
		},
		{
			ScanConfig: &models.ScanConfigRelationship{
				Id: *scanConfigs[1].Id,
			},
			ScanConfigSnapshot: scan2ConfigSnapshot,
			StartTime:          &scan2Start,
			State:              utils.PointerTo(models.ScanStateInProgress),
			StateMessage:       utils.PointerTo("Scan is in progress"),
			StateReason:        nil,
			Summary:            scan2Summary,
			TargetIDs:          &scan2Targets,
		},
	}
}

// nolint:gocognit
func createScanResults(scans []models.Scan) []models.TargetScanResult {
	var scanResults []models.TargetScanResult
	for _, scan := range scans {
		for _, targetID := range *scan.TargetIDs {
			result := models.TargetScanResult{
				Id: nil,
				Scan: &models.ScanRelationship{
					Id: *scan.Id,
				},
				Secrets: nil,
				Status:  nil,
				Summary: &models.ScanFindingsSummary{},
				Target: &models.TargetRelationship{
					Id: targetID,
				},
			}
			// Create Exploits if needed
			if *scan.ScanConfigSnapshot.ScanFamiliesConfig.Exploits.Enabled {
				result.Exploits = &models.ExploitScan{
					Exploits: createExploitsResult(),
				}
				result.Summary.TotalExploits = utils.PointerTo(len(*result.Exploits.Exploits))
			} else {
				result.Summary.TotalExploits = utils.PointerTo(0)
			}

			// Create Malware if needed
			if *scan.ScanConfigSnapshot.ScanFamiliesConfig.Malware.Enabled {
				result.Malware = &models.MalwareScan{
					Malware: createMalwareResult(),
				}
				result.Summary.TotalMalware = utils.PointerTo(len(*result.Malware.Malware))
			} else {
				result.Summary.TotalMalware = utils.PointerTo(0)
			}

			// Create Misconfigurations if needed
			if *scan.ScanConfigSnapshot.ScanFamiliesConfig.Misconfigurations.Enabled {
				result.Misconfigurations = &models.MisconfigurationScan{
					Misconfigurations: createMisconfigurationsResult(),
				}
				result.Summary.TotalMisconfigurations = utils.PointerTo(len(*result.Misconfigurations.Misconfigurations))
			} else {
				result.Summary.TotalMisconfigurations = utils.PointerTo(0)
			}

			// Create Packages if needed
			if *scan.ScanConfigSnapshot.ScanFamiliesConfig.Sbom.Enabled {
				result.Sboms = &models.SbomScan{
					Packages: createPackagesResult(),
				}
				result.Summary.TotalPackages = utils.PointerTo(len(*result.Sboms.Packages))
			} else {
				result.Summary.TotalPackages = utils.PointerTo(0)
			}

			// Create Rootkits if needed
			if *scan.ScanConfigSnapshot.ScanFamiliesConfig.Rootkits.Enabled {
				result.Rootkits = &models.RootkitScan{
					Rootkits: createRootkitsResult(),
				}
				result.Summary.TotalRootkits = utils.PointerTo(len(*result.Rootkits.Rootkits))
			} else {
				result.Summary.TotalRootkits = utils.PointerTo(0)
			}

			// Create Secrets if needed
			if *scan.ScanConfigSnapshot.ScanFamiliesConfig.Secrets.Enabled {
				result.Secrets = &models.SecretScan{
					Secrets: createSecretsResult(),
				}
				result.Summary.TotalSecrets = utils.PointerTo(len(*result.Secrets.Secrets))
			} else {
				result.Summary.TotalSecrets = utils.PointerTo(0)
			}

			// Create Vulnerabilities if needed
			if *scan.ScanConfigSnapshot.ScanFamiliesConfig.Vulnerabilities.Enabled {
				result.Vulnerabilities = &models.VulnerabilityScan{
					Vulnerabilities: createVulnerabilitiesResult(),
				}
				result.Summary.TotalVulnerabilities = utils.GetVulnerabilityTotalsPerSeverity(result.Vulnerabilities.Vulnerabilities)
			} else {
				result.Summary.TotalVulnerabilities = utils.GetVulnerabilityTotalsPerSeverity(nil)
			}

			scanResults = append(scanResults, result)
		}
	}
	return scanResults
}

func createSecretsResult() *[]models.Secret {
	return &[]models.Secret{
		{
			Description: utils.PointerTo("AWS Credentials"),
			EndColumn:   utils.PointerTo(8),
			EndLine:     utils.PointerTo(43),
			FilePath:    utils.PointerTo("/.aws/credentials"),
			Fingerprint: utils.PointerTo("credentials:aws-access-token:4"),
			StartColumn: utils.PointerTo(7),
			StartLine:   utils.PointerTo(43),
		},
		{
			Description: utils.PointerTo("export BUNDLE_ENTERPRISE__CONTRIBSYS__COM=cafebabe:deadbeef"),
			EndColumn:   utils.PointerTo(10),
			EndLine:     utils.PointerTo(26),
			FilePath:    utils.PointerTo("cmd/generate/config/rules/sidekiq.go"),
			Fingerprint: utils.PointerTo("cd5226711335c68be1e720b318b7bc3135a30eb2:cmd/generate/config/rules/sidekiq.go:sidekiq-secret:23"),
			StartColumn: utils.PointerTo(7),
			StartLine:   utils.PointerTo(23),
		},
		{
			Description: utils.PointerTo("GitLab Personal Access Token"),
			EndColumn:   utils.PointerTo(22),
			EndLine:     utils.PointerTo(7),
			FilePath:    utils.PointerTo("Applications/Firefox.app/Contents/Resources/browser/omni.ja"),
			Fingerprint: utils.PointerTo("Applications/Firefox.app/Contents/Resources/browser/omni.ja:generic-api-key:sfs2"),
			StartColumn: utils.PointerTo(20),
			StartLine:   utils.PointerTo(7),
		},
	}
}

func createRootkitsResult() *[]models.Rootkit {
	return &[]models.Rootkit{
		{
			Message:     utils.PointerTo("/usr/lwp-request"),
			RootkitName: utils.PointerTo("Ambient's Rootkit (ARK)"),
			RootkitType: utils.PointerTo(models.RootkitType("ARK")),
		},
		{
			Message:     utils.PointerTo("Possible Linux/Ebury 1.4 - Operation Windigo installed"),
			RootkitName: utils.PointerTo("Linux/Ebury - Operation Windigo ssh"),
			RootkitType: utils.PointerTo(models.RootkitType("Malware")),
		},
		{
			Message:     utils.PointerTo("/var/adm/wtmpx"),
			RootkitName: utils.PointerTo("Mumblehard backdoor/botnet"),
			RootkitType: utils.PointerTo(models.RootkitType("Botnet")),
		},
	}
}

func createPackagesResult() *[]models.Package {
	return &[]models.Package{
		{
			Cpes:     utils.PointerTo([]string{"cpe:2.3:a:curl:curl:7.74.0-1.3+deb11u3:*:*:*:*:*:*:*"}),
			Language: utils.PointerTo(""),
			Licenses: utils.PointerTo([]string{"BSD-3-Clause", "BSD-4-Clause"}),
			Name:     utils.PointerTo("curl"),
			Purl:     utils.PointerTo("pkg:deb/debian/curl@7.74.0-1.3+deb11u3?arch=amd64&distro=debian-11"),
			Type:     utils.PointerTo("deb"),
			Version:  utils.PointerTo("7.74.0-1.3+deb11u3"),
		},
		{
			Cpes:     utils.PointerTo([]string{"cpe:2.3:a:libtasn1-6:libtasn1-6:4.16.0-2:*:*:*:*:*:*:*", "cpe:2.3:a:libtasn1-6:libtasn1_6:4.16.0-2:*:*:*:*:*:*:*"}),
			Language: utils.PointerTo("python"),
			Licenses: utils.PointerTo([]string{"GFDL-1.3-only", "GPL-3.0-only", "LGPL-2.1-only"}),
			Name:     utils.PointerTo("libtasn1-6"),
			Purl:     utils.PointerTo("pkg:deb/debian/libtasn1-6@4.16.0-2?arch=amd64&distro=debian-11"),
			Type:     utils.PointerTo("deb"),
			Version:  utils.PointerTo("4.16.0-2"),
		},
	}
}

func createExploitsResult() *[]models.Exploit {
	return &[]models.Exploit{
		{
			CveID:       utils.PointerTo("CVE-2009-4091"),
			Description: utils.PointerTo("Simplog 0.9.3.2 - Multiple Vulnerabilities"),
			Name:        utils.PointerTo("10180"),
			SourceDB:    utils.PointerTo("OffensiveSecurity"),
			Title:       utils.PointerTo("10180"),
			Urls:        utils.PointerTo([]string{"https://www.exploit-db.com/exploits/10180"}),
		},
		{
			CveID:       utils.PointerTo("CVE-2006-2896"),
			Description: utils.PointerTo("FunkBoard CF0.71 - 'profile.php' Remote User Pass Change"),
			Name:        utils.PointerTo("1875"),
			SourceDB:    utils.PointerTo("OffensiveSecurity"),
			Title:       utils.PointerTo("1875"),
			Urls:        utils.PointerTo([]string{"https://gitlab.com/exploit-database/exploitdb/-/tree/main/exploits/php/webapps/1875.html"}),
		},
	}
}

func createMalwareResult() *[]models.Malware {
	return &[]models.Malware{
		{
			MalwareName: utils.PointerTo("Pdf.Exploit.CVE_2009_4324-1"),
			MalwareType: utils.PointerTo("WORM"),
			Path:        utils.PointerTo("/test/metasploit-framework/modules/exploits/windows/browser/asus_net4switch_ipswcom.rb"),
		},
		{
			MalwareName: utils.PointerTo("Xml.Malware.Squiblydoo-6728833-0"),
			MalwareType: utils.PointerTo("SPYWARE"),
			Path:        utils.PointerTo("/test/metasploit-framework/modules/exploits/windows/fileformat/office_ms17_11882.rb"),
		},
		{
			MalwareName: utils.PointerTo("Unix.Trojan.MSShellcode-27"),
			MalwareType: utils.PointerTo("TROJAN"),
			Path:        utils.PointerTo("/test/metasploit-framework/documentation/modules/exploit/multi/http/makoserver_cmd_exec.md"),
		},
	}
}

func createMisconfigurationsResult() *[]models.Misconfiguration {
	return &[]models.Misconfiguration{
		{
			Message:         utils.PointerTo("Install a PAM module for password strength testing like pam_cracklib or pam_passwdqc. Details: /lib/x86_64-linux-gnu/security/pam_access.so"),
			Remediation:     utils.PointerTo("remediation2"),
			ScannedPath:     utils.PointerTo("/home/ubuntu/debian11"),
			ScannerName:     utils.PointerTo("scanner2"),
			Severity:        utils.PointerTo(models.MisconfigurationHighSeverity),
			TestCategory:    utils.PointerTo("AUTH"),
			TestDescription: utils.PointerTo("Checking presence password strength testing tools (PAM)"),
			TestID:          utils.PointerTo("AUTH-9262"),
		},
		{
			Message:         utils.PointerTo("Set the sticky bit on /home/ubuntu/debian11/tmp, to prevent users deleting (by other owned) files in the /tmp directory. Details: /tmp"),
			Remediation:     utils.PointerTo("remediation1"),
			ScannedPath:     utils.PointerTo("/home/ubuntu/debian11"),
			ScannerName:     utils.PointerTo("scanner1"),
			Severity:        utils.PointerTo(models.MisconfigurationMediumSeverity),
			TestCategory:    utils.PointerTo("FILE"),
			TestDescription: utils.PointerTo("Checking /tmp sticky bit"),
			TestID:          utils.PointerTo("FILE-6362"),
		},
		{
			Message:         utils.PointerTo("Disable drivers like USB storage when not used, to prevent unauthorized storage or data theft. Details: /etc/cron.d/e2scrub_all"),
			Remediation:     utils.PointerTo("remediation1"),
			ScannedPath:     utils.PointerTo("/home/ubuntu/debian11"),
			ScannerName:     utils.PointerTo("scanner1"),
			Severity:        utils.PointerTo(models.MisconfigurationLowSeverity),
			TestCategory:    utils.PointerTo("USB"),
			TestDescription: utils.PointerTo("Check if USB storage is disabled"),
			TestID:          utils.PointerTo("USB-1000"),
		},
	}
}

func createVulnerabilitiesResult() *[]models.Vulnerability {
	return &[]models.Vulnerability{
		{
			Cvss: utils.PointerTo([]models.VulnerabilityCvss{
				{
					Metrics: &models.VulnerabilityCvssMetrics{
						BaseScore:           utils.PointerTo[float32](7.5),
						ExploitabilityScore: utils.PointerTo[float32](3.9),
						ImpactScore:         utils.PointerTo[float32](3.6),
					},
					Vector:  utils.PointerTo("CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:N/A:N"),
					Version: utils.PointerTo("3.1"),
				},
			}),
			Description: utils.PointerTo("A vulnerability exists in curl <7.87.0 HSTS check that could be bypassed to trick it to keep using HTTP. Using its HSTS support, curl can be instructed to use HTTPS instead of using an insecure clear-text HTTP step even when HTTP is provided in the\nURL. However, the HSTS mechanism could be bypassed if the host name in the given URL first uses IDN characters that get replaced to ASCII counterparts as part of the IDN conversion. Like using the character UTF-8 U+3002 (IDEOGRAPHIC FULL STOP) instead of the common ASCI\nI full stop (U+002E) `.`. Then in a subsequent request, it does not detect the HSTS state and makes a clear text transfer. Because it would store the info IDN encoded but look for it IDN decoded."),
			Distro: &models.VulnerabilityDistro{
				IDLike:  utils.PointerTo([]string{"debian"}),
				Name:    utils.PointerTo("ubuntu"),
				Version: utils.PointerTo("11"),
			},
			Fix: &models.VulnerabilityFix{
				State:    utils.PointerTo("wont-fix"),
				Versions: utils.PointerTo([]string{}),
			},
			LayerId: utils.PointerTo(""),
			Links:   utils.PointerTo([]string{"https://security-tracker.debian.org/tracker/CVE-2022-43551"}),
			Package: &models.Package{
				Cpes:     utils.PointerTo([]string{"cpe:2.3:a:curl:curl:7.74.0-1.3+deb11u3:*:*:*:*:*:*:*"}),
				Language: utils.PointerTo("pl1"),
				Licenses: utils.PointerTo([]string{"BSD-3-Clause", "BSD-4-Clause"}),
				Name:     utils.PointerTo("curl"),
				Purl:     utils.PointerTo("pkg:deb/debian/curl@7.74.0-1.3+deb11u3?arch=amd64&distro=debian-11"),
				Type:     utils.PointerTo("deb"),
				Version:  utils.PointerTo("7.74.0-1.3+deb11u3"),
			},
			Path:              utils.PointerTo("/var/lib/dpkg/status"),
			Severity:          utils.PointerTo[models.VulnerabilitySeverity](models.HIGH),
			VulnerabilityName: utils.PointerTo("CVE-2022-43551"),
		},
		{
			Cvss: utils.PointerTo([]models.VulnerabilityCvss{
				{
					Metrics: &models.VulnerabilityCvssMetrics{
						BaseScore:           utils.PointerTo[float32](9.1),
						ExploitabilityScore: utils.PointerTo[float32](3.9),
						ImpactScore:         utils.PointerTo[float32](5.2),
					},
					Vector:  utils.PointerTo("CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:N/A:H"),
					Version: utils.PointerTo("3.1"),
				},
				{
					Metrics: &models.VulnerabilityCvssMetrics{
						BaseScore:           utils.PointerTo[float32](4),
						ExploitabilityScore: utils.PointerTo[float32](4.1),
						ImpactScore:         utils.PointerTo[float32](4.2),
					},
					Vector:  utils.PointerTo("AV:N/AC:L/Au:N/C:P/I:P/A:P"),
					Version: utils.PointerTo("2.0"),
				},
			}),
			Description: utils.PointerTo("GNU Libtasn1 before 4.19.0 has an ETYPE_OK off-by-one array size check that affects asn1_encode_simple_der."),
			Distro: &models.VulnerabilityDistro{
				IDLike:  utils.PointerTo([]string{"debian"}),
				Name:    utils.PointerTo("ubuntu"),
				Version: utils.PointerTo("11"),
			},
			Fix: &models.VulnerabilityFix{
				State:    utils.PointerTo("fixed"),
				Versions: utils.PointerTo([]string{"4.16.0-2+deb11u1"}),
			},
			LayerId: utils.PointerTo(""),
			Links:   utils.PointerTo([]string{"https://security-tracker.debian.org/tracker/CVE-2021-46848", "https://security-tracker.debian.org/tracker/CVE-2021-46848_new"}),
			Package: &models.Package{
				Cpes:     utils.PointerTo([]string{"cpe:2.3:a:libtasn1-6:libtasn1-6:4.16.0-2:*:*:*:*:*:*:*", "cpe:2.3:a:libtasn1-6:libtasn1_6:4.16.0-2:*:*:*:*:*:*:*"}),
				Language: utils.PointerTo(""),
				Licenses: utils.PointerTo([]string{"GFDL-1.3-only", "GPL-3.0-only", "LGPL-2.1-only"}),
				Name:     utils.PointerTo("libtasn1-6"),
				Purl:     utils.PointerTo("pkg:deb/debian/libtasn1-6@4.16.0-2?arch=amd64&distro=debian-11"),
				Type:     utils.PointerTo("deb"),
				Version:  utils.PointerTo("4.16.0-2"),
			},
			Path:              utils.PointerTo("/var/lib/dpkg/status"),
			Severity:          utils.PointerTo[models.VulnerabilitySeverity](models.CRITICAL),
			VulnerabilityName: utils.PointerTo("CVE-2021-46848"),
		},
		{
			Cvss: utils.PointerTo([]models.VulnerabilityCvss{
				{
					Metrics: &models.VulnerabilityCvssMetrics{
						BaseScore:           utils.PointerTo[float32](9.8),
						ExploitabilityScore: utils.PointerTo[float32](3.9),
						ImpactScore:         utils.PointerTo[float32](5.9),
					},
					Vector:  utils.PointerTo("CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"),
					Version: utils.PointerTo("3.1"),
				},
				{
					Metrics: &models.VulnerabilityCvssMetrics{
						BaseScore:           utils.PointerTo[float32](7.5),
						ExploitabilityScore: utils.PointerTo[float32](10),
						ImpactScore:         utils.PointerTo[float32](6.4),
					},
					Vector:  utils.PointerTo("AV:N/AC:L/Au:N/C:P/I:P/A:P"),
					Version: utils.PointerTo("2.0"),
				},
			}),
			Description: utils.PointerTo("SQLite3 from 3.6.0 to and including 3.27.2 is vulnerable to heap out-of-bound read in the rtreenode() function when handling invalid rtree tables."),
			Distro: &models.VulnerabilityDistro{
				IDLike:  utils.PointerTo([]string{"debian"}),
				Name:    utils.PointerTo("ubuntu"),
				Version: utils.PointerTo("11"),
			},
			Fix: &models.VulnerabilityFix{
				State:    utils.PointerTo("wont-fix"),
				Versions: utils.PointerTo([]string{""}),
			},
			LayerId: utils.PointerTo(""),
			Links:   utils.PointerTo([]string{"https://security-tracker.debian.org/tracker/CVE-2019-8457"}),
			Package: &models.Package{
				Cpes:     utils.PointerTo([]string{"cpe:2.3:a:libdb5.3:libdb5.3:5.3.28+dfsg1-0.8:*:*:*:*:*:*:*"}),
				Language: utils.PointerTo(""),
				Licenses: utils.PointerTo([]string{}),
				Name:     utils.PointerTo("libdb5.3"),
				Purl:     utils.PointerTo("pkg:deb/debian/libdb5.3@5.3.28+dfsg1-0.8?arch=amd64&upstream=db5.3&distro=debian-11"),
				Type:     utils.PointerTo("deb"),
				Version:  utils.PointerTo("5.3.28+dfsg1-0.8"),
			},
			Path:              utils.PointerTo("/var/lib/dpkg/status"),
			Severity:          utils.PointerTo[models.VulnerabilitySeverity](models.LOW),
			VulnerabilityName: utils.PointerTo("CVE-2019-8457"),
		},
	}
}
