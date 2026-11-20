package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"velocityengine/internal/model"
	"velocityengine/internal/syswin"
)

// ProbeInfo extends ProbeResponse with internal stream metadata needed for merge compatibility.
type ProbeInfo struct {
	model.ProbeResponse
	PixFmt          string
	AudioSampleRate int
	AudioChannels   int
}

type ffprobeJSON struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	CodecType    string            `json:"codec_type"`
	CodecName    string            `json:"codec_name"`
	Width        int               `json:"width"`
	Height       int               `json:"height"`
	PixFmt       string            `json:"pix_fmt"`
	RFrameRate   string            `json:"r_frame_rate"`
	AvgFrameRate string            `json:"avg_frame_rate"`
	Duration     string            `json:"duration"`
	StartTime    string            `json:"start_time"`
	BitRate      string            `json:"bit_rate"`
	SampleRate   string            `json:"sample_rate"`
	Channels     int               `json:"channels"`
	Tags         map[string]string `json:"tags"`
	SideDataList []ffprobeSideData `json:"side_data_list"`
}

type ffprobeSideData struct {
	SideDataType string   `json:"side_data_type"`
	Rotation     *float64 `json:"rotation"`
}

type ffprobeFormat struct {
	Duration  string `json:"duration"`
	StartTime string `json:"start_time"`
	Size      string `json:"size"`
	BitRate   string `json:"bit_rate"`
}

// ProbeFile inspects a media file with ffprobe and computes display resolution after rotation.
func ProbeFile(ctx context.Context, ffprobePath, filePath string) (ProbeInfo, error) {
	var out ProbeInfo
	if ffprobePath == "" {
		return out, fmt.Errorf("ffprobe executable not found")
	}
	absPath, err := filepath.Abs(strings.TrimSpace(filePath))
	if err != nil {
		return out, fmt.Errorf("invalid path: %w", err)
	}
	stat, err := os.Stat(absPath)
	if err != nil {
		return out, err
	}
	if stat.IsDir() {
		return out, fmt.Errorf("path is a directory: %s", absPath)
	}

	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cctx, ffprobePath,
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		absPath,
	)
	cmd.SysProcAttr = syswin.HiddenProcAttr()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return out, fmt.Errorf("ffprobe failed: %s", msg)
	}

	var parsed ffprobeJSON
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		return out, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	return buildProbeInfo(absPath, stat.Size(), parsed)
}

func buildProbeInfo(absPath string, fileSize int64, parsed ffprobeJSON) (ProbeInfo, error) {
	var vStream *ffprobeStream
	var aStream *ffprobeStream
	for i := range parsed.Streams {
		st := &parsed.Streams[i]
		switch st.CodecType {
		case "video":
			if vStream == nil {
				vStream = st
			}
		case "audio":
			if aStream == nil {
				aStream = st
			}
		}
	}
	if vStream == nil {
		return ProbeInfo{}, fmt.Errorf("no video stream found in %s", filepath.Base(absPath))
	}

	rot := parseStreamRotation(vStream)
	dispW, dispH := vStream.Width, vStream.Height
	absRot := rot
	if absRot < 0 {
		absRot = -absRot
	}
	if absRot%180 == 90 {
		dispW, dispH = vStream.Height, vStream.Width
	}

	dur := parseFloatStr(parsed.Format.Duration)
	if dur <= 0 {
		dur = parseFloatStr(vStream.Duration)
	}
	startTime := parseFloatStr(parsed.Format.StartTime)
	if startTime == 0 {
		startTime = parseFloatStr(vStream.StartTime)
	}

	fps := parseFractionFPS(vStream.AvgFrameRate)
	if fps <= 0 {
		fps = parseFractionFPS(vStream.RFrameRate)
	}

	bps := parseFloatStr(vStream.BitRate)
	if bps <= 0 {
		bps = parseFloatStr(parsed.Format.BitRate)
	}
	bitrateKbps := 0
	if bps > 0 {
		bitrateKbps = int(math.Round(bps / 1000.0))
	}
	if bitrateKbps <= 0 && dur > 0 && fileSize > 0 {
		bitrateKbps = int(math.Round((float64(fileSize) * 8.0) / (dur * 1000.0)))
	}

	info := ProbeInfo{
		ProbeResponse: model.ProbeResponse{
			Success:     true,
			Path:        absPath,
			Width:       dispW,
			Height:      dispH,
			RawWidth:    vStream.Width,
			RawHeight:   vStream.Height,
			Rotation:    rot,
			Duration:    dur,
			FPS:         fps,
			VideoCodec:  vStream.CodecName,
			StartTime:   startTime,
			SizeMB:      math.Round((float64(fileSize)/(1024*1024))*100) / 100,
			BitrateKbps: bitrateKbps,
		},
		PixFmt: vStream.PixFmt,
	}

	if aStream != nil {
		info.HasAudio = true
		info.AudioCodec = aStream.CodecName
		info.AudioSampleRate = int(parseFloatStr(aStream.SampleRate))
		info.AudioChannels = aStream.Channels
	}
	return info, nil
}

func parseStreamRotation(s *ffprobeStream) int {
	for _, sd := range s.SideDataList {
		if strings.Contains(strings.ToLower(sd.SideDataType), "display matrix") && sd.Rotation != nil {
			return int(math.Round(*sd.Rotation))
		}
	}
	for k, v := range s.Tags {
		if strings.EqualFold(k, "rotate") {
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				return n
			}
		}
	}
	return 0
}

func parseFloatStr(raw string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

func parseFractionFPS(raw string) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "0/0" {
		return 0
	}
	if parts := strings.Split(raw, "/"); len(parts) == 2 {
		num := parseFloatStr(parts[0])
		den := parseFloatStr(parts[1])
		if den > 0 && num > 0 {
			return math.Round((num/den)*1000) / 1000
		}
		return 0
	}
	return parseFloatStr(raw)
}
