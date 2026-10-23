//go:build linux

package sysinfo

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
	"sync"

	"codeberg.org/puregotk/purego"
)

// VulkanDevice describes a physical device as reported by the
// Vulkan loader.
type VulkanDevice struct {
	Name       string
	VendorID   string // lowercase hexadecimal, as in [Card]
	ProductID  string // lowercase hexadecimal, as in [Card]
	APIVersion uint32
}

// VulkanVersion formats a packed Vulkan API version.
func VulkanVersion(v uint32) string {
	return fmt.Sprintf("%d.%d.%d", (v>>22)&0x7f, (v>>12)&0x3ff, v&0xfff)
}

// MakeVulkanVersion packs a Vulkan API version.
func MakeVulkanVersion(major, minor uint32) uint32 {
	return major<<22 | minor<<12
}

func (d VulkanDevice) String() string {
	return d.Name + " (Vulkan " + VulkanVersion(d.APIVersion) + ")"
}

// VulkanDevices lazily queries the Vulkan loader for the available
// physical devices. An empty list is returned if Vulkan is unavailable.
var VulkanDevices = sync.OnceValue(func() []VulkanDevice {
	devs, _ := queryVulkan()
	return devs
})

// VulkanAPI returns the Vulkan API version supported by the
// given card, or 0 if it is unknown.
func (c *Card) VulkanAPI() uint32 {
	for _, d := range VulkanDevices() {
		if d.VendorID == c.VendorID && d.ProductID == c.ProductID {
			return d.APIVersion
		}
	}
	return 0
}

// C structure layouts from vulkan_core.h, for LP64.
type vkApplicationInfo struct {
	sType              uint32
	pNext              uintptr
	pApplicationName   *byte
	applicationVersion uint32
	pEngineName        *byte
	engineVersion      uint32
	apiVersion         uint32
}

type vkInstanceCreateInfo struct {
	sType                   uint32
	pNext                   uintptr
	flags                   uint32
	pApplicationInfo        *vkApplicationInfo
	enabledLayerCount       uint32
	ppEnabledLayerNames     uintptr
	enabledExtensionCount   uint32
	ppEnabledExtensionNames uintptr
}

// Only the leading members of VkPhysicalDeviceProperties are read;
// the remainder is reserved to cover the full structure size.
type vkPhysicalDeviceProperties struct {
	apiVersion    uint32
	driverVersion uint32
	vendorID      uint32
	deviceID      uint32
	deviceType    uint32
	deviceName    [256]byte
	_             [1024]byte
}

const (
	vkStructureTypeApplicationInfo    = 0
	vkStructureTypeInstanceCreateInfo = 1
	vkSuccess                         = 0
	vkIncomplete                      = 5
)

func queryVulkan() ([]VulkanDevice, error) {
	lib, err := purego.Dlopen("libvulkan.so.1", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, err
	}
	defer purego.Dlclose(lib)

	var (
		createInstance             func(*vkInstanceCreateInfo, uintptr, *uintptr) int32
		destroyInstance            func(uintptr, uintptr)
		enumeratePhysicalDevices   func(uintptr, *uint32, *uintptr) int32
		getPhysicalDeviceProperies func(uintptr, *vkPhysicalDeviceProperties)
	)
	purego.RegisterLibFunc(&createInstance, lib, "vkCreateInstance")
	purego.RegisterLibFunc(&destroyInstance, lib, "vkDestroyInstance")
	purego.RegisterLibFunc(&enumeratePhysicalDevices, lib, "vkEnumeratePhysicalDevices")
	purego.RegisterLibFunc(&getPhysicalDeviceProperies, lib, "vkGetPhysicalDeviceProperties")

	name := []byte("Vinegar\x00")
	app := vkApplicationInfo{
		sType:            vkStructureTypeApplicationInfo,
		pApplicationName: &name[0],
		pEngineName:      &name[0],
		apiVersion:       MakeVulkanVersion(1, 1),
	}
	info := vkInstanceCreateInfo{
		sType:            vkStructureTypeInstanceCreateInfo,
		pApplicationInfo: &app,
	}

	var instance uintptr
	if r := createInstance(&info, 0, &instance); r != vkSuccess {
		return nil, fmt.Errorf("vkCreateInstance: %d", r)
	}
	defer destroyInstance(instance, 0)
	runtime.KeepAlive(name)

	var count uint32
	if r := enumeratePhysicalDevices(instance, &count, nil); r != vkSuccess || count == 0 {
		return nil, fmt.Errorf("vkEnumeratePhysicalDevices: %d", r)
	}
	handles := make([]uintptr, count)
	if r := enumeratePhysicalDevices(instance, &count, &handles[0]); r != vkSuccess && r != vkIncomplete {
		return nil, fmt.Errorf("vkEnumeratePhysicalDevices: %d", r)
	}

	devs := make([]VulkanDevice, 0, count)
	for _, h := range handles[:count] {
		var p vkPhysicalDeviceProperties
		getPhysicalDeviceProperies(h, &p)
		d := VulkanDevice{
			Name:       strings.TrimRight(string(p.deviceName[:]), "\x00"),
			VendorID:   fmt.Sprintf("%04x", p.vendorID),
			ProductID:  fmt.Sprintf("%04x", p.deviceID),
			APIVersion: p.apiVersion,
		}
		// The same device may be exposed by multiple ICDs, such as
		// within Flatpak where host and runtime drivers both exist.
		if !slices.Contains(devs, d) {
			devs = append(devs, d)
		}
	}
	return devs, nil
}
