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

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/urfave/cli"

	"github.com/openclarity/vmclarity/backend/pkg/backend"
	"github.com/openclarity/vmclarity/backend/pkg/config"
	databaseTypes "github.com/openclarity/vmclarity/backend/pkg/database/types"
	"github.com/openclarity/vmclarity/backend/pkg/version"
	"github.com/openclarity/vmclarity/shared/pkg/log"
)

const (
	LogLevelFlag         = "log-level"
	LogLevelDefaultValue = "warning"
)

func run(cliCtx *cli.Context) {
	log.InitLogger(cliCtx.String(LogLevelFlag), os.Stderr)

	ctx := context.Background()
	logger := logrus.WithContext(ctx)
	ctx = log.SetLoggerForContext(ctx, logger)
	backend.Run(ctx)
}

func versionCommand(_ *cli.Context) {
	fmt.Printf("Version: %s \nCommit: %s\nBuild Time: %s",
		version.Version, version.CommitHash, version.BuildTimestamp)
}

func main() {
	viper.SetDefault(config.HealthCheckAddress, ":8081")
	viper.SetDefault(config.BackendRestPort, "8888")
	viper.SetDefault(config.DatabaseDriver, databaseTypes.DBDriverTypeLocal)
	viper.SetDefault(config.DisableOrchestrator, "false")
	viper.SetDefault(config.UISitePath, "/app/site")
	viper.AutomaticEnv()
	app := cli.NewApp()
	app.Usage = ""
	app.Name = "VMClarity"
	app.Version = version.Version

	runCommand := cli.Command{
		Name:   "run",
		Usage:  "Starts VMClarity",
		Action: run,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  LogLevelFlag,
				Value: LogLevelDefaultValue,
				Usage: fmt.Sprintf("Set log level %s", logrus.AllLevels),
			},
		},
	}
	runCommand.UsageText = runCommand.Name

	versionCommand := cli.Command{
		Name:   "version",
		Usage:  "VMClarity Version Details",
		Action: versionCommand,
	}
	versionCommand.UsageText = versionCommand.Name

	app.Commands = []cli.Command{
		runCommand,
		versionCommand,
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
