package model

// QualitySpec defines the specifications for a video quality level
type QualitySpec struct {
	Quality      string
	Resolution   string
	VideoBitrate string
	Bandwidth    int
}

// QualitySpecs contains all supported video quality specifications
var QualitySpecs = map[string]QualitySpec{
	"360p": {
		Quality:      "360p",
		Resolution:   "640x360",
		VideoBitrate: "800k",
		Bandwidth:    800000,
	},
	"480p": {
		Quality:      "480p",
		Resolution:   "854x480",
		VideoBitrate: "1200k",
		Bandwidth:    1200000,
	},
	"720p": {
		Quality:      "720p",
		Resolution:   "1280x720",
		VideoBitrate: "2500k",
		Bandwidth:    2500000,
	},
	"1080p": {
		Quality:      "1080p",
		Resolution:   "1920x1080",
		VideoBitrate: "5000k",
		Bandwidth:    5000000,
	},
	"1440p": {
		Quality:      "1440p",
		Resolution:   "2560x1440",
		VideoBitrate: "8000k",
		Bandwidth:    8000000,
	},
	"2160p": {
		Quality:      "2160p",
		Resolution:   "3840x2160",
		VideoBitrate: "15000k",
		Bandwidth:    15000000,
	},
}

// GetQualitySpec returns the quality specification for a given quality level
func GetQualitySpec(quality string) (QualitySpec, bool) {
	spec, ok := QualitySpecs[quality]
	return spec, ok
}

// GetBandwidth returns the bandwidth for a given quality level
func GetBandwidth(quality string) int {
	if spec, ok := QualitySpecs[quality]; ok {
		return spec.Bandwidth
	}
	return 800000 // default
}
