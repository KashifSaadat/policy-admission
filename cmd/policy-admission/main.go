/*
Copyright 2017 Home Office All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/UKHomeOffice/policy-admission/pkg/api"
	"github.com/UKHomeOffice/policy-admission/pkg/authorize"
	"github.com/UKHomeOffice/policy-admission/pkg/server"

	"github.com/urfave/cli"
)

var (
	// Version is the version of the service
	Version = "v0.0.8"
	// GitSHA is the git sha this was built off
	GitSHA = "unknown"
)

func main() {
	app := &cli.App{
		Name:    "policy-admission",
		Author:  "Rohith Jayawardene",
		Email:   "gambol99@gmail.com",
		Usage:   "is a service used to enforce secuirty policy within a cluster",
		Version: fmt.Sprintf("%s (git+sha: %s)", Version, GitSHA),

		OnUsageError: func(context *cli.Context, err error, isSubcommand bool) error {
			fmt.Fprintf(os.Stderr, "[error] invalid options, %s\n", err)
			return err
		},

		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "listen",
				Usage:  "network interace the service should listen on `INTERFACE`",
				Value:  ":8443",
				EnvVar: "LISTEN",
			},
			cli.StringFlag{
				Name:   "tls-cert",
				Usage:  "path to a file containing the tls certificate `PATH`",
				EnvVar: "TLS_CERT",
			},
			cli.StringFlag{
				Name:   "tls-key",
				Usage:  "path to a file containing the tls key `PATH`",
				EnvVar: "TLS_KEY",
			},
			cli.StringSliceFlag{
				Name:  "authorizer",
				Usage: "enable a admission authorizer, the format is name=config_path (i.e securitycontext=config.yaml)",
			},
			cli.StringFlag{
				Name:   "namespace",
				Usage:  "namespace to create denial events (optional as we can try and discover) `NAME`",
				EnvVar: "KUBE_NAMESPACE",
				Value:  "kube-admission",
			},
			cli.BoolFlag{
				Name:   "enable-logging",
				Usage:  "indicates you wish to log the admission requests for debugging `BOOL`",
				EnvVar: "ENABLE_LOGGING",
			},
			cli.BoolFlag{
				Name:   "enable-events",
				Usage:  "indicates you wish to log kubernetes events on denials `BOOL`",
				EnvVar: "ENABLE_EVENTS",
			},
		},

		Action: func(cx *cli.Context) error {
			var authorizers []api.Authorize
			// @step: configure the authorizers
			for _, config := range cx.StringSlice("authorizer") {
				authorizer, err := configureAuthorizer(config)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[error] unable to enable authorizer: %s", err)
					os.Exit(1)
				}
				authorizers = append(authorizers, authorizer)
			}

			config := &server.Config{
				EnableEvents:  cx.Bool("enable-events"),
				EnableLogging: cx.Bool("enable-logging"),
				Listen:        cx.String("listen"),
				Namespace:     cx.String("namespace"),
				TLSCert:       cx.String("tls-cert"),
				TLSKey:        cx.String("tls-key"),
			}

			// @step: create the server
			ctl, err := server.New(config, authorizers)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[error] unable to initialize controller, %q\n", err)
				os.Exit(1)
			}

			// @step: start the service
			if err := ctl.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "[error] unable to start controller, %q\n", err)
				os.Exit(1)
			}

			// @step setup the termination signals
			signalChannel := make(chan os.Signal)
			signal.Notify(signalChannel, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
			<-signalChannel

			return nil
		},
	}

	app.Run(os.Args)
}

// configureAuthorizer is responsible for creating an authorizer
func configureAuthorizer(cfg string) (api.Authorize, error) {
	items := strings.Split(cfg, "=")
	if len(items) > 2 {
		return nil, errors.New("invalid authorizer config, should be name:config_path")
	}

	providerName := items[0]
	providerConfig := ""
	if len(items) == 2 {
		providerConfig = items[1]
	}

	return authorize.New(providerName, providerConfig, true)
}
