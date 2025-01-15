// Copyright (c) 2022 Xored Software Inc and others.
//
// Copyright (c) 2023 Cisco and/or its affiliates.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build linux
// +build linux

package routelocalnet

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/networkservicemesh/api/pkg/api/networkservice"
	"github.com/networkservicemesh/api/pkg/api/networkservice/mechanisms/kernel"
	"github.com/pkg/errors"

	"github.com/ljkiraly/sdk-kernel/pkg/kernel/tools/nshandle"
)

func setRouteLocalNet(conn *networkservice.Connection) error {
	mechanism := kernel.ToMechanism(conn.GetMechanism())
	if mechanism != nil && mechanism.GetRouteLocalNet() {
		currentNsHandler, err := nshandle.Current()
		if err != nil {
			return err
		}
		defer func() { _ = currentNsHandler.Close() }()

		targetHsHandler, err := nshandle.FromURL(mechanism.GetNetNSURL())
		if err != nil {
			return err
		}
		defer func() { _ = targetHsHandler.Close() }()

		err = nshandle.RunIn(currentNsHandler, targetHsHandler, func() error {
			file := filepath.Clean(fmt.Sprintf("/proc/sys/net/ipv4/conf/%s/route_localnet", mechanism.GetInterfaceName()))
			fo, fileErr := os.Create(file)
			if fileErr != nil {
				return errors.Wrapf(fileErr, "failed to create file %s", file)
			}

			defer func() { _ = fo.Close() }()

			_, fileErr = fo.WriteString("1")
			if fileErr != nil {
				return errors.Wrapf(fileErr, "failed to write to file %s", file)
			}

			return nil
		})

		if err != nil {
			return err
		}
	}

	return nil
}
