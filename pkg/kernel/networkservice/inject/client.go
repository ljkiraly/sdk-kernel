// Copyright (c) 2022 Cisco and/or its affiliates.
//
// Copyright (c) 2021-2023 Nordix Foundation.
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

package inject

import (
	"context"
	"sync"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/networkservicemesh/api/pkg/api/networkservice"
	"github.com/ljkiraly/sdk/pkg/networkservice/core/next"
	"github.com/ljkiraly/sdk/pkg/networkservice/utils/metadata"
	"github.com/ljkiraly/sdk/pkg/tools/postpone"

	"github.com/ljkiraly/sdk-kernel/pkg/kernel/networkservice/vfconfig"
)

type injectClient struct {
	vfRefCountMap   map[string]int
	vfRefCountMutex sync.Mutex
}

// NewClient - returns a new networkservice.NetworkServiceClient that moves given network
// interface into the Endpoint's pod network namespace on Request and back to Forwarder's
// network namespace on Close
func NewClient() networkservice.NetworkServiceClient {
	return &injectClient{vfRefCountMap: make(map[string]int)}
}

func (c *injectClient) Request(ctx context.Context, request *networkservice.NetworkServiceRequest, opts ...grpc.CallOption) (*networkservice.Connection, error) {
	var isEstablished bool
	if vfConfig, ok := vfconfig.Load(ctx, metadata.IsClient(c)); ok {
		isEstablished = int(vfConfig.ContNetNS) != 0
	}

	postponeCtxFunc := postpone.ContextWithValues(ctx)

	conn, err := next.Client(ctx).Request(ctx, request, opts...)
	if err != nil {
		return nil, err
	}

	if !isEstablished {
		if err := move(ctx, conn, c.vfRefCountMap, &c.vfRefCountMutex, metadata.IsClient(c), false); err != nil {
			closeCtx, cancelClose := postponeCtxFunc()
			defer cancelClose()

			if _, closeErr := c.Close(closeCtx, conn, opts...); closeErr != nil {
				err = errors.Wrapf(err, "connection closed with error: %s", closeErr.Error())
			}

			return nil, err
		}
	}

	return conn, nil
}

func (c *injectClient) Close(ctx context.Context, conn *networkservice.Connection, opts ...grpc.CallOption) (*empty.Empty, error) {
	injectErr := move(ctx, conn, c.vfRefCountMap, &c.vfRefCountMutex, metadata.IsClient(c), true)
	if injectErr != nil {
		return nil, injectErr
	}
	return next.Client(ctx).Close(ctx, conn, opts...)
}
