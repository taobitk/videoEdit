package ffmpeg

import (
	"fmt"
	"os"
	"strings"
)

var vulkanGlobalArgs = []string{"-init_hw_device", "vulkan=vk:1", "-filter_hw_device", "vk"}

var colorShaderMatrices = map[string]string{
	"sepia": "    float r = clamp(0.393*c.r + 0.769*c.g + 0.189*c.b, 0.0, 1.0);\n" +
		"    float g = clamp(0.349*c.r + 0.686*c.g + 0.168*c.b, 0.0, 1.0);\n" +
		"    float b = clamp(0.272*c.r + 0.534*c.g + 0.131*c.b, 0.0, 1.0);",
	"warm": "    float r = clamp(1.12*c.r, 0.0, 1.0);\n" +
		"    float g = clamp(1.02*c.g, 0.0, 1.0);\n" +
		"    float b = clamp(0.86*c.b, 0.0, 1.0);",
	"cool": "    float r = clamp(0.86*c.r, 0.0, 1.0);\n" +
		"    float g = clamp(1.00*c.g, 0.0, 1.0);\n" +
		"    float b = clamp(1.14*c.b, 0.0, 1.0);",
}

// BuildGPUColorFilter builds a libplacebo Vulkan GPU filter (and optional temp GLSL shader) matching ColorFilterExpr.
func BuildGPUColorFilter(name string) (filterExpr string, tempFile string, globalArgs []string, err error) {
	key := strings.ToLower(strings.TrimSpace(name))
	switch key {
	case "", "none":
		return "", "", nil, nil
	case "grayscale":
		return "libplacebo=saturation=0:format=nv12", "", vulkanGlobalArgs, nil
	case "vibrant":
		return "libplacebo=saturation=1.4:format=nv12", "", vulkanGlobalArgs, nil
	}

	body, ok := colorShaderMatrices[key]
	if !ok {
		return "", "", nil, fmt.Errorf("unknown video_filter %q", name)
	}

	f, err := os.CreateTemp("", "ve_color_*.glsl")
	if err != nil {
		return "", "", nil, fmt.Errorf("create color shader file: %w", err)
	}
	tmpPath := f.Name()
	shader := "//!HOOK MAIN\n//!BIND HOOKED\nvec4 hook() {\n    vec4 c = HOOKED_tex(HOOKED_pos);\n" +
		body + "\n    return vec4(r, g, b, c.a);\n}\n"
	if _, err := f.WriteString(shader); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return "", "", nil, fmt.Errorf("write color shader file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", "", nil, fmt.Errorf("close color shader file: %w", err)
	}

	return fmt.Sprintf("libplacebo=custom_shader_path=%s:format=nv12", EscapeFilterPath(tmpPath)), tmpPath, vulkanGlobalArgs, nil
}
