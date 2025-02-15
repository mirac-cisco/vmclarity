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

package families

import (
	"context"
	"fmt"

	"github.com/openclarity/vmclarity/shared/pkg/families/exploits"
	"github.com/openclarity/vmclarity/shared/pkg/families/interfaces"
	"github.com/openclarity/vmclarity/shared/pkg/families/malware"
	"github.com/openclarity/vmclarity/shared/pkg/families/misconfiguration"
	"github.com/openclarity/vmclarity/shared/pkg/families/results"
	"github.com/openclarity/vmclarity/shared/pkg/families/rootkits"
	"github.com/openclarity/vmclarity/shared/pkg/families/sbom"
	"github.com/openclarity/vmclarity/shared/pkg/families/secrets"
	"github.com/openclarity/vmclarity/shared/pkg/families/types"
	"github.com/openclarity/vmclarity/shared/pkg/families/vulnerabilities"
	"github.com/openclarity/vmclarity/shared/pkg/log"
)

type Manager struct {
	config   *Config
	families []interfaces.Family
}

func New(config *Config) *Manager {
	manager := &Manager{
		config: config,
	}

	// Analyzers.
	// SBOM MUST come before vulnerabilities.
	if config.SBOM.Enabled {
		manager.families = append(manager.families, sbom.New(config.SBOM))
	}

	// Scanners.
	// Vulnerabilities MUST be after SBOM to support the case it is configured to use the output from sbom.
	if config.Vulnerabilities.Enabled {
		manager.families = append(manager.families, vulnerabilities.New(config.Vulnerabilities))
	}
	if config.Secrets.Enabled {
		manager.families = append(manager.families, secrets.New(config.Secrets))
	}
	if config.Rootkits.Enabled {
		manager.families = append(manager.families, rootkits.New(config.Rootkits))
	}
	if config.Malware.Enabled {
		manager.families = append(manager.families, malware.New(config.Malware))
	}
	if config.Misconfiguration.Enabled {
		manager.families = append(manager.families, misconfiguration.New(config.Misconfiguration))
	}

	// Enrichers.
	// Exploits MUST be after Vulnerabilities to support the case it is configured to use the output from Vulnerabilities.
	if config.Exploits.Enabled {
		manager.families = append(manager.families, exploits.New(config.Exploits))
	}

	return manager
}

type RunErrors map[types.FamilyType]error

type FamilyResult struct {
	Result     interfaces.IsResults
	FamilyType types.FamilyType
	Err        error
}

type FamilyNotifier interface {
	FamilyStarted(context.Context, types.FamilyType) error
	FamilyFinished(ctx context.Context, res FamilyResult) error
}

func (m *Manager) Run(ctx context.Context, notifier FamilyNotifier) []error {
	var oneOrMoreFamilyFailed bool
	var errors []error
	familyResults := results.New()

	logger := log.GetLoggerFromContextOrDiscard(ctx)

	for _, family := range m.families {
		if err := notifier.FamilyStarted(ctx, family.GetType()); err != nil {
			errors = append(errors, fmt.Errorf("family started notification failed: %v", err))
			continue
		}

		result := make(chan FamilyResult)
		go func() {
			ret, err := family.Run(ctx, familyResults)
			result <- FamilyResult{
				Result:     ret,
				Err:        err,
				FamilyType: family.GetType(),
			}
		}()

		select {
		case <-ctx.Done():
			go func() {
				<-result
				close(result)
			}()
			oneOrMoreFamilyFailed = true
			if err := notifier.FamilyFinished(ctx, FamilyResult{
				Result:     nil,
				FamilyType: family.GetType(),
				Err:        fmt.Errorf("failed to run family %v: aborted", family.GetType()),
			}); err != nil {
				errors = append(errors, fmt.Errorf("family finished notification failed: %v", err))
			}
		case r := <-result:
			logger.Debugf("received result from family %q: %v", family.GetType(), r)
			if r.Err != nil {
				oneOrMoreFamilyFailed = true
			} else {
				familyResults.SetResults(r.Result)
			}
			if err := notifier.FamilyFinished(ctx, r); err != nil {
				errors = append(errors, fmt.Errorf("family finished notification failed: %v", err))
			}
			close(result)
		}
	}

	if oneOrMoreFamilyFailed {
		errors = append(errors, fmt.Errorf("at least one family failed to run"))
	}
	return errors
}
