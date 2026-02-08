// Package fastfetch provides system information collection via the fastfetch CLI.
// It parses fastfetch's JSON output to extract OS, hardware, and resource metrics.
package fastfetch

// FastfetchData holds parsed system information from fastfetch JSON output.
// This structure maps to fastfetch's --json output format.
type FastfetchData struct {
	OS        FastfetchModule `json:"os"`
	Host      FastfetchModule `json:"host"`
	Kernel    FastfetchModule `json:"kernel"`
	Uptime    FastfetchModule `json:"uptime"`
	Packages  FastfetchModule `json:"packages"`
	Shell     FastfetchModule `json:"shell"`
	Terminal  FastfetchModule `json:"terminal"`
	CPU       FastfetchModule `json:"cpu"`
	GPU       FastfetchModule `json:"gpu"`
	Memory    FastfetchModule `json:"memory"`
	Disk      FastfetchModule `json:"disk"`
	LocalIP   FastfetchModule `json:"localIP"`
	Battery   FastfetchModule `json:"battery,omitempty"`
	WM        FastfetchModule `json:"wm,omitempty"`
	Theme     FastfetchModule `json:"theme,omitempty"`
	Icons     FastfetchModule `json:"icons,omitempty"`
	Font      FastfetchModule `json:"font,omitempty"`
	Cursor    FastfetchModule `json:"cursor,omitempty"`
	Locale    FastfetchModule `json:"locale,omitempty"`
	DateTime  FastfetchModule `json:"dateTime,omitempty"`
	PublicIP  FastfetchModule `json:"publicIP,omitempty"`
	Weather   FastfetchModule `json:"weather,omitempty"`
	Player    FastfetchModule `json:"player,omitempty"`
	Media     FastfetchModule `json:"media,omitempty"`
	Processes FastfetchModule `json:"processes,omitempty"`
	Swap      FastfetchModule `json:"swap,omitempty"`
}

// FastfetchModule represents a single module from fastfetch output.
// Each module has a type identifier and key-value pairs.
type FastfetchModule struct {
	Type   string `json:"type"`
	Key    string `json:"key,omitempty"`
	KeyRaw string `json:"keyRaw,omitempty"`
	Result string `json:"result,omitempty"`
}

// FastfetchRawOutput represents the raw JSON array output from fastfetch.
// Fastfetch outputs an array of modules, each with type, key, and result fields.
type FastfetchRawOutput struct {
	Modules []FastfetchRawModule `json:"modules"`
}

// FastfetchRawModule represents a single module in the raw fastfetch JSON array.
type FastfetchRawModule struct {
	Type   string `json:"type"`
	Key    string `json:"key,omitempty"`
	KeyRaw string `json:"keyRaw,omitempty"`
	Result string `json:"result,omitempty"`
}

// parseRawModules converts the raw fastfetch JSON array into a structured FastfetchData.
// It maps module types to their corresponding fields.
func parseRawModules(modules []FastfetchRawModule) *FastfetchData {
	data := &FastfetchData{}

	for _, m := range modules {
		module := FastfetchModule{
			Type:   m.Type,
			Key:    m.Key,
			KeyRaw: m.KeyRaw,
			Result: m.Result,
		}

		switch m.Type {
		case "OS", "os":
			data.OS = module
		case "Host", "host":
			data.Host = module
		case "Kernel", "kernel":
			data.Kernel = module
		case "Uptime", "uptime":
			data.Uptime = module
		case "Packages", "packages":
			data.Packages = module
		case "Shell", "shell":
			data.Shell = module
		case "Terminal", "terminal":
			data.Terminal = module
		case "CPU", "cpu":
			data.CPU = module
		case "GPU", "gpu":
			data.GPU = module
		case "Memory", "memory":
			data.Memory = module
		case "Disk", "disk":
			data.Disk = module
		case "LocalIP", "localip", "localIp":
			data.LocalIP = module
		case "Battery", "battery":
			data.Battery = module
		case "WM", "wm":
			data.WM = module
		case "Theme", "theme":
			data.Theme = module
		case "Icons", "icons":
			data.Icons = module
		case "Font", "font":
			data.Font = module
		case "Cursor", "cursor":
			data.Cursor = module
		case "Locale", "locale":
			data.Locale = module
		case "DateTime", "datetime", "dateTime":
			data.DateTime = module
		case "PublicIP", "publicip", "publicIp":
			data.PublicIP = module
		case "Weather", "weather":
			data.Weather = module
		case "Player", "player":
			data.Player = module
		case "Media", "media":
			data.Media = module
		case "Processes", "processes":
			data.Processes = module
		case "Swap", "swap":
			data.Swap = module
		}
	}

	return data
}

// IsEmpty returns true if no modules have been populated.
func (d *FastfetchData) IsEmpty() bool {
	return d.OS.Type == "" &&
		d.Host.Type == "" &&
		d.Kernel.Type == "" &&
		d.CPU.Type == "" &&
		d.Memory.Type == ""
}

// GetCoreModules returns the 12 core modules for banner display.
// These are the most commonly displayed system info fields.
func (d *FastfetchData) GetCoreModules() []FastfetchModule {
	modules := make([]FastfetchModule, 0, 12)

	if d.OS.Type != "" {
		modules = append(modules, d.OS)
	}
	if d.Host.Type != "" {
		modules = append(modules, d.Host)
	}
	if d.Kernel.Type != "" {
		modules = append(modules, d.Kernel)
	}
	if d.Uptime.Type != "" {
		modules = append(modules, d.Uptime)
	}
	if d.Packages.Type != "" {
		modules = append(modules, d.Packages)
	}
	if d.Shell.Type != "" {
		modules = append(modules, d.Shell)
	}
	if d.Terminal.Type != "" {
		modules = append(modules, d.Terminal)
	}
	if d.CPU.Type != "" {
		modules = append(modules, d.CPU)
	}
	if d.GPU.Type != "" {
		modules = append(modules, d.GPU)
	}
	if d.Memory.Type != "" {
		modules = append(modules, d.Memory)
	}
	if d.Disk.Type != "" {
		modules = append(modules, d.Disk)
	}
	if d.LocalIP.Type != "" {
		modules = append(modules, d.LocalIP)
	}

	return modules
}

// FormatForDisplay returns a slice of key-value pairs suitable for banner display.
// Each string is formatted as "Key: Value".
func (d *FastfetchData) FormatForDisplay() []string {
	var lines []string

	addLine := func(m FastfetchModule) {
		if m.Type == "" || m.Result == "" {
			return
		}
		key := m.Type
		if m.Key != "" {
			key = m.Key
		}
		lines = append(lines, key+": "+m.Result)
	}

	// Core system info in display order
	addLine(d.OS)
	addLine(d.Host)
	addLine(d.Kernel)
	addLine(d.Uptime)
	addLine(d.CPU)
	addLine(d.GPU)
	addLine(d.Memory)
	addLine(d.Disk)
	addLine(d.Packages)
	addLine(d.Shell)
	addLine(d.Terminal)
	addLine(d.LocalIP)

	return lines
}

// FormatCompact returns a condensed view with only essential system info.
// Suitable for narrow banner columns.
func (d *FastfetchData) FormatCompact() []string {
	var lines []string

	addLine := func(label, value string) {
		if value == "" {
			return
		}
		lines = append(lines, label+": "+value)
	}

	addLine("OS", d.OS.Result)
	addLine("Kernel", d.Kernel.Result)
	addLine("CPU", d.CPU.Result)
	addLine("RAM", d.Memory.Result)
	addLine("Disk", d.Disk.Result)
	addLine("Uptime", d.Uptime.Result)

	return lines
}
