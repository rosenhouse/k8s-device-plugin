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
	"k8s.io/klog/v2"

	"github.com/NVIDIA/k8s-device-plugin/internal/resource"
)

// newVGPULicenseLabeler creates a labeler for the vGPU software license of the
// vGPU guests on the node. A failed query is logged and labeled unknown, so that
// it does not prevent the other labels from being generated.
func newVGPULicenseLabeler(devices []resource.Device) Labeler {
	var unlicensed, unknown bool
	products := make(map[string][]int)
	for i, d := range devices {
		license, err := d.GetVGPULicense()
		switch {
		case err != nil:
			klog.Warningf("Failed to get vGPU license of device %d: %v", i, err)
			unknown = true
		case license == nil:
			continue
		case license.Licensed:
			product := sanitise(license.Product)
			products[product] = append(products[product], i)
		default:
			unlicensed = true
		}
	}

	switch {
	case unlicensed:
		return Labels{"nvidia.com/vgpu.license.status": "unlicensed"}
	case unknown:
		return Labels{"nvidia.com/vgpu.license.status": "unknown"}
	case len(products) == 0:
		return empty{}
	}

	labels := Labels{"nvidia.com/vgpu.license.status": "licensed"}
	if len(products) > 1 {
		klog.Warningf("vGPU license products are non-unique: %v", products)
		return labels
	}
	for product := range products {
		if product != "" {
			labels["nvidia.com/vgpu.license.product"] = product
		}
	}
	return labels
}
