package sysinfo

import "testing"

func TestVulkanVersion(t *testing.T) {
	v := MakeVulkanVersion(1, 3)
	if got := VulkanVersion(v); got != "1.3.0" {
		t.Fatalf("VulkanVersion(%d) = %s, want 1.3.0", v, got)
	}
}

func TestVulkanDevices(t *testing.T) {
	// Must never crash, even without a Vulkan loader present.
	for _, d := range VulkanDevices() {
		t.Log(d, d.VendorID, d.ProductID)
	}
	for _, c := range Cards {
		t.Log(c, "vulkan", VulkanVersion(c.VulkanAPI()))
	}
}
