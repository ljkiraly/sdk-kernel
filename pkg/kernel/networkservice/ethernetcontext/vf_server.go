// Copyright (c) 2020-2022 Doc.ai and/or its affiliates.
//
// Copyright (c) 2021-2022 Nordix Foundation.
//
// Copyright (c) 2022-2023 Cisco and/or its affiliates.
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

// Package ethernetcontext provides chain element for setup link ethernet properties
package ethernetcontext

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"

	"github.com/networkservicemesh/api/pkg/api/networkservice"
	"github.com/ljkiraly/sdk/pkg/networkservice/core/next"
	"github.com/ljkiraly/sdk/pkg/tools/log"
	"github.com/ljkiraly/sdk/pkg/tools/postpone"

	"github.com/ljkiraly/sdk-kernel/pkg/kernel/networkservice/vfconfig"
)

type vfEthernetContextServer struct{}

// NewVFServer returns a new VF ethernet context server chain element
func NewVFServer() networkservice.NetworkServiceServer {
	return &vfEthernetContextServer{}
}

func (s *vfEthernetContextServer) Request(ctx context.Context, request *networkservice.NetworkServiceRequest) (*networkservice.Connection, error) {
	postponeCtxFunc := postpone.ContextWithValues(ctx)

	conn, err := next.Server(ctx).Request(ctx, request)
	if err != nil {
		return nil, err
	}

	if vfConfig, ok := vfconfig.Load(ctx, false); ok {
		if err := vfCreate(ctx, vfConfig, conn, false); err != nil {
			closeCtx, cancelClose := postponeCtxFunc()
			defer cancelClose()

			if _, closeErr := s.Close(closeCtx, conn); closeErr != nil {
				err = errors.Wrapf(err, "connection closed with error: %s", closeErr.Error())
			}

			return nil, err
		}
	} else {
		if err := setKernelHwAddress(ctx, conn, false); err != nil {
			closeCtx, cancelClose := postponeCtxFunc()
			defer cancelClose()

			if _, closeErr := s.Close(closeCtx, conn); closeErr != nil {
				err = errors.Wrapf(err, "connection closed with error: %s", closeErr.Error())
			}

			return nil, err
		}
	}
	return conn, nil
}

func (s *vfEthernetContextServer) Close(ctx context.Context, conn *networkservice.Connection) (*empty.Empty, error) {
	if vfConfig, ok := vfconfig.Load(ctx, false); ok {
		if err := vfCleanup(ctx, vfConfig); err != nil {
			log.FromContext(ctx).Errorf("vfEthernetContextServer vfClear: %v", err.Error())
		}
	}
	return next.Server(ctx).Close(ctx, conn)
}
