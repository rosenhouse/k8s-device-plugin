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

package resource

import (
	"testing"

	"github.com/NVIDIA/go-nvlib/pkg/nvlib/device"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/stretchr/testify/require"
)

// vgpuLicenseDevice is a minimal device.Device test double for the NVML calls
// that GetVGPULicense makes.
type vgpuLicenseDevice struct {
	device.Device
	mode        nvml.GpuVirtualizationMode
	modeRet     nvml.Return
	features    nvml.GridLicensableFeatures
	featuresRet nvml.Return
}

func (d vgpuLicenseDevice) GetVirtualizationMode() (nvml.GpuVirtualizationMode, nvml.Return) {
	return d.mode, d.modeRet
}

func (d vgpuLicenseDevice) GetGridLicensableFeatures() (nvml.GridLicensableFeatures, nvml.Return) {
	return d.features, d.featuresRet
}

func licensableFeature(product string, licensed bool) nvml.GridLicensableFeature {
	feature := nvml.GridLicensableFeature{FeatureEnabled: 1}
	for i, c := range []byte(product) {
		feature.ProductName[i] = int8(c) //nolint:gosec // Test product names are ASCII.
	}
	if licensed {
		feature.FeatureState = 1
	}
	return feature
}

func licensableFeatures(features ...nvml.GridLicensableFeature) nvml.GridLicensableFeatures {
	f := nvml.GridLicensableFeatures{IsGridLicenseSupported: 1}
	for i, feature := range features {
		f.GridLicensableFeatures[i] = feature
		f.LicensableFeaturesCount++
	}
	return f
}

func TestNVMLDeviceGetVGPULicense(t *testing.T) {
	vcs := licensableFeature("NVIDIA Virtual Compute Server", true)
	unlicensedVCS := licensableFeature("NVIDIA Virtual Compute Server", false)
	vws := licensableFeature("NVIDIA RTX Virtual Workstation", true)

	unsupportedByDriver := licensableFeatures(vcs)
	unsupportedByDriver.IsGridLicenseSupported = 0

	uncountedLicense := licensableFeatures(unlicensedVCS)
	uncountedLicense.GridLicensableFeatures[1] = vws

	overstatedCount := licensableFeatures(unlicensedVCS, unlicensedVCS, vws)
	overstatedCount.LicensableFeaturesCount = 4

	testCases := []struct {
		description     string
		device          vgpuLicenseDevice
		expectedLicense *VGPULicense
		expectedError   error
	}{
		{
			description: "bare metal does not query licensing",
			device:      vgpuLicenseDevice{mode: nvml.GPU_VIRTUALIZATION_MODE_NONE, featuresRet: nvml.ERROR_UNKNOWN},
		},
		{
			description: "passthrough is not a vGPU guest",
			device:      vgpuLicenseDevice{mode: nvml.GPU_VIRTUALIZATION_MODE_PASSTHROUGH, features: licensableFeatures(vcs)},
		},
		{
			description: "unsupported virtualization mode query means no vGPU guest",
			device:      vgpuLicenseDevice{mode: nvml.GPU_VIRTUALIZATION_MODE_VGPU, modeRet: nvml.ERROR_NOT_SUPPORTED, features: licensableFeatures(vcs)},
		},
		{
			description:   "virtualization mode error is returned",
			device:        vgpuLicenseDevice{mode: nvml.GPU_VIRTUALIZATION_MODE_VGPU, modeRet: nvml.ERROR_UNKNOWN, features: licensableFeatures(vcs)},
			expectedError: nvml.ERROR_UNKNOWN,
		},
		{
			description: "unsupported license query means licensing does not apply",
			device:      vgpuLicenseDevice{mode: nvml.GPU_VIRTUALIZATION_MODE_VGPU, featuresRet: nvml.ERROR_NOT_SUPPORTED},
		},
		{
			description: "driver without licensing support means licensing does not apply",
			device:      vgpuLicenseDevice{mode: nvml.GPU_VIRTUALIZATION_MODE_VGPU, features: unsupportedByDriver},
		},
		{
			description:   "license query error is returned",
			device:        vgpuLicenseDevice{mode: nvml.GPU_VIRTUALIZATION_MODE_VGPU, featuresRet: nvml.ERROR_UNKNOWN},
			expectedError: nvml.ERROR_UNKNOWN,
		},
		{
			description:     "licensed guest reports its product",
			device:          vgpuLicenseDevice{mode: nvml.GPU_VIRTUALIZATION_MODE_VGPU, features: licensableFeatures(vcs)},
			expectedLicense: &VGPULicense{Licensed: true, Product: "NVIDIA Virtual Compute Server"},
		},
		{
			description:     "unlicensed guest reports no product",
			device:          vgpuLicenseDevice{mode: nvml.GPU_VIRTUALIZATION_MODE_VGPU, features: licensableFeatures(unlicensedVCS)},
			expectedLicense: &VGPULicense{},
		},
		{
			description:     "guest without licensable features is unlicensed",
			device:          vgpuLicenseDevice{mode: nvml.GPU_VIRTUALIZATION_MODE_VGPU, features: licensableFeatures()},
			expectedLicense: &VGPULicense{},
		},
		{
			description:     "any licensed feature licenses the guest",
			device:          vgpuLicenseDevice{mode: nvml.GPU_VIRTUALIZATION_MODE_VGPU, features: licensableFeatures(unlicensedVCS, vws)},
			expectedLicense: &VGPULicense{Licensed: true, Product: "NVIDIA RTX Virtual Workstation"},
		},
		{
			description:     "features beyond the reported count are ignored",
			device:          vgpuLicenseDevice{mode: nvml.GPU_VIRTUALIZATION_MODE_VGPU, features: uncountedLicense},
			expectedLicense: &VGPULicense{},
		},
		{
			description:     "overstated feature count stays within the array",
			device:          vgpuLicenseDevice{mode: nvml.GPU_VIRTUALIZATION_MODE_VGPU, features: overstatedCount},
			expectedLicense: &VGPULicense{Licensed: true, Product: "NVIDIA RTX Virtual Workstation"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			license, err := nvmlDevice{Device: tc.device}.GetVGPULicense()
			require.ErrorIs(t, err, tc.expectedError)
			require.Equal(t, tc.expectedLicense, license)
		})
	}
}
