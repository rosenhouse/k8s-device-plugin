/**
# Copyright (c) NVIDIA CORPORATION.  All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
**/

package lm

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/k8s-device-plugin/internal/resource"
)

func newVGPULicenseMock(license *resource.VGPULicense, err error) *resource.DeviceMock {
	return &resource.DeviceMock{
		GetVGPULicenseFunc: func() (*resource.VGPULicense, error) { return license, err },
	}
}

func TestVGPULicenseLabeler(t *testing.T) {
	notGuest := newVGPULicenseMock(nil, nil)
	vcs := newVGPULicenseMock(&resource.VGPULicense{Licensed: true, Product: "NVIDIA Virtual Compute Server"}, nil)
	vws := newVGPULicenseMock(&resource.VGPULicense{Licensed: true, Product: "NVIDIA RTX Virtual Workstation"}, nil)
	noProduct := newVGPULicenseMock(&resource.VGPULicense{Licensed: true}, nil)
	unlicensed := newVGPULicenseMock(&resource.VGPULicense{}, nil)
	failed := newVGPULicenseMock(nil, errors.New("license query failed"))

	testCases := []struct {
		description    string
		devices        []resource.Device
		expectedLabels map[string]string
	}{
		{
			description: "devices that are not vGPU guests produce no labels",
			devices:     []resource.Device{notGuest, notGuest},
		},
		{
			description: "licensed guest is labeled with its product",
			devices:     []resource.Device{notGuest, vcs},
			expectedLabels: map[string]string{
				"nvidia.com/vgpu.license.status":  "licensed",
				"nvidia.com/vgpu.license.product": "NVIDIA-Virtual-Compute-Server",
			},
		},
		{
			description: "unlicensed guest makes the node unlicensed",
			devices:     []resource.Device{vcs, unlicensed, vcs},
			expectedLabels: map[string]string{
				"nvidia.com/vgpu.license.status": "unlicensed",
			},
		},
		{
			description: "failed query makes the node unknown",
			devices:     []resource.Device{notGuest, failed},
			expectedLabels: map[string]string{
				"nvidia.com/vgpu.license.status": "unknown",
			},
		},
		{
			description: "unknown takes precedence over licensed",
			devices:     []resource.Device{vcs, failed, vcs},
			expectedLabels: map[string]string{
				"nvidia.com/vgpu.license.status": "unknown",
			},
		},
		{
			description: "unlicensed takes precedence over unknown",
			devices:     []resource.Device{failed, unlicensed, failed},
			expectedLabels: map[string]string{
				"nvidia.com/vgpu.license.status": "unlicensed",
			},
		},
		{
			description: "different products omit the product label",
			devices:     []resource.Device{vcs, vws},
			expectedLabels: map[string]string{
				"nvidia.com/vgpu.license.status": "licensed",
			},
		},
		{
			description: "empty product omits the product label",
			devices:     []resource.Device{noProduct},
			expectedLabels: map[string]string{
				"nvidia.com/vgpu.license.status": "licensed",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			labels, err := newVGPULicenseLabeler(tc.devices).Labels()
			require.NoError(t, err)
			require.EqualValues(t, tc.expectedLabels, labels)
		})
	}
}
