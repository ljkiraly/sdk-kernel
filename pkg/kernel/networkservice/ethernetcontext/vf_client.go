// Copyright (c) 2021-2022 Nordix Foundation.
//
// Copyright (c) 2021-2022 Doc.ai and/or its affiliates.
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

package ethernetcontext

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/networkservicemesh/api/pkg/api/networkservice"
	"github.com/ljkiraly/sdk/pkg/networkservice/core/next"
	"github.com/ljkiraly/sdk/pkg/tools/log"
	"github.com/ljkiraly/sdk/pkg/tools/postpone"

	"github.com/ljkiraly/sdk-kernel/pkg/kernel/networkservice/vfconfig"
)

type vfEthernetClient struct{}

// NewVFClient returns a new VF ethernet context client chain element
func NewVFClient() networkservice.NetworkServiceClient {
	return &vfEthernetClient{}
}

func (i *vfEthernetClient) Request(ctx context.Context, request *networkservice.NetworkServiceRequest, opts ...grpc.CallOption) (*networkservice.Connection, error) {
	postponeCtxFunc := postpone.ContextWithValues(ctx)

	conn, err := next.Client(ctx).Request(ctx, request, opts...)
	if err != nil {
		return nil, err
	}

	if vfConfig, ok := vfconfig.Load(ctx, true); ok {
		if err := vfCreate(ctx, vfConfig, conn, true); err != nil {
			closeCtx, cancelClose := postponeCtxFunc()
			defer cancelClose()

			if _, closeErr := i.Close(closeCtx, conn, opts...); closeErr != nil {
				err = errors.Wrapf(err, "connection closed with error: %s", closeErr.Error())
			}

			return nil, err
		}
	} else {
		if err := setKernelHwAddress(ctx, conn, true); err != nil {
			closeCtx, cancelClose := postponeCtxFunc()
			defer cancelClose()

			if _, closeErr := i.Close(closeCtx, conn); closeErr != nil {
				err = errors.Wrapf(err, "connection closed with error: %s", closeErr.Error())
			}

			return nil, err
		}
	}

	return conn, nil
}

func (i *vfEthernetClient) Close(ctx context.Context, conn *networkservice.Connection, opts ...grpc.CallOption) (*empty.Empty, error) {
	if vfConfig, ok := vfconfig.Load(ctx, true); ok {
		if err := vfCleanup(ctx, vfConfig); err != nil {
			log.FromContext(ctx).Errorf("vfEthernetClient vfClear: %v", err.Error())
		}
	}
	return next.Client(ctx).Close(ctx, conn, opts...)
}
