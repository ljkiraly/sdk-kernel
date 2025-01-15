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

// Package heal contains an implementation of LivenessChecker.
package heal

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"github.com/go-ping/ping"
	"github.com/networkservicemesh/api/pkg/api/networkservice"
	"github.com/networkservicemesh/api/pkg/api/networkservice/mechanisms/kernel"
	"github.com/ljkiraly/sdk/pkg/tools/log"
)

const (
	defaultTimeout = time.Second
	packetCount    = 4
)

type options struct {
	pingerFactory PingerFactory
}

// Option is an option pattern for LivelinessChecker
type Option func(o *options)

// WithPingerFactory - sets any custom pinger factory
func WithPingerFactory(pf PingerFactory) Option {
	return func(o *options) {
		o.pingerFactory = pf
	}
}

// KernelLivenessCheck is an implementation of heal.LivenessCheck
func KernelLivenessCheck(deadlineCtx context.Context, conn *networkservice.Connection) bool {
	return KernelLivenessCheckWithOptions(deadlineCtx, conn)
}

// KernelLivenessCheckWithOptions is an implementation with options of heal.LivenessCheck. It sends ICMP
// ping and checks reply. Returns false if didn't get reply.
func KernelLivenessCheckWithOptions(deadlineCtx context.Context, conn *networkservice.Connection, opts ...Option) bool {
	// Apply options
	o := &options{
		pingerFactory: &defaultPingerFactory{},
	}
	for _, opt := range opts {
		opt(o)
	}
	var pingerFactory = o.pingerFactory

	if mechanism := conn.GetMechanism().GetType(); mechanism != kernel.MECHANISM {
		log.FromContext(deadlineCtx).Warnf("ping is not supported for mechanism %v", mechanism)
		return true
	}
	ipContext := conn.GetContext().GetIpContext()
	combinationCount := len(ipContext.GetDstIpAddrs()) * len(ipContext.GetSrcIpAddrs())
	if combinationCount == 0 {
		log.FromContext(deadlineCtx).Debug("No IP address")
		return true
	}

	deadline, ok := deadlineCtx.Deadline()
	if !ok {
		deadline = time.Now().Add(defaultTimeout)
	}
	timeout := time.Until(deadline)

	// Start ping for all Src/DstIPs combination
	responseCh := make(chan error, combinationCount)
	defer close(responseCh)
	for _, srcIPNet := range ipContext.GetSrcIPNets() {
		for _, dstIPNet := range ipContext.GetDstIPNets() {
			// Skip if IPs don't belong to the same family
			if (srcIPNet.IP.To4() != nil) != (dstIPNet.IP.To4() != nil) {
				responseCh <- nil
				continue
			}

			go func(srcIP, dstIP string) {
				logger := log.FromContext(deadlineCtx).WithField("srcIP", srcIP).WithField("dstIP", dstIP)
				pinger := pingerFactory.CreatePinger(srcIP, dstIP, timeout, packetCount)

				err := pinger.Run()
				if err != nil {
					logger.Errorf("Ping failed: %s", err.Error())
					responseCh <- err
					return
				}

				if pinger.GetReceivedPackets() == 0 {
					err = errors.New("No packets received")
					logger.Errorf(err.Error())
					responseCh <- err
					return
				}
				responseCh <- nil
			}(srcIPNet.IP.String(), dstIPNet.IP.String())
		}
	}

	// Waiting for all ping results. If at least one fails - return false
	return waitForResponses(responseCh)
}

func waitForResponses(responseCh <-chan error) bool {
	respCount := cap(responseCh)
	success := true
	for {
		resp, ok := <-responseCh
		if !ok {
			return false
		}
		if resp != nil {
			success = false
		}
		respCount--
		if respCount == 0 {
			return success
		}
	}
}

// PingerFactory - factory interface for creating pingers
type PingerFactory interface {
	CreatePinger(srcIP, dstIP string, timeout time.Duration, count int) Pinger
}

// Pinger - pinger interface
type Pinger interface {
	Run() error
	GetReceivedPackets() int
}

type defaultPingerFactory struct{}

func (p *defaultPingerFactory) CreatePinger(srcIP, dstIP string, timeout time.Duration, count int) Pinger {
	pi := ping.New(dstIP)
	pi.Source = srcIP
	pi.Timeout = timeout
	pi.Count = count
	if count != 0 {
		pi.Interval = timeout / time.Duration(count)
	}

	return &defaultPinger{pinger: pi}
}

type defaultPinger struct {
	pinger *ping.Pinger
}

func (p *defaultPinger) Run() error {
	return p.pinger.Run()
}

func (p *defaultPinger) GetReceivedPackets() int {
	return p.pinger.Statistics().PacketsRecv
}
