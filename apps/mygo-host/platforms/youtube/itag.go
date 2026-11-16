// itag 静态映射表(参照 NewPipe 的 ItagItem):itag 是 YouTube 格式代号,
// 比从 URL/字段猜类型可靠。只维护播放栈可播的 H.264/AAC 子集,未知 itag 返回 false。

package youtube

type itagType string

const (
	itagAudio     itagType = "audio"
	itagVideo     itagType = "video"
	itagVideoOnly itagType = "video_only"
)

type itagInfo struct {
	id         int
	kind       itagType
	container  string
	codec      string
	audioCodec string
	resolution string
	height     int
	fps        int
}

var itags = map[int]itagInfo{
	// 渐进式(mp4,音视频合一)
	18: {id: 18, kind: itagVideo, container: "mp4", codec: "avc1", audioCodec: "mp4a", resolution: "360p", height: 360, fps: 30},
	22: {id: 22, kind: itagVideo, container: "mp4", codec: "avc1", audioCodec: "mp4a", resolution: "720p", height: 720, fps: 30},
	37: {id: 37, kind: itagVideo, container: "mp4", codec: "avc1", audioCodec: "mp4a", resolution: "1080p", height: 1080, fps: 30},
	38: {id: 38, kind: itagVideo, container: "mp4", codec: "avc1", audioCodec: "mp4a", resolution: "1080p", height: 1080, fps: 30},
	// 纯视频(DASH,无音轨)
	160: {id: 160, kind: itagVideoOnly, container: "mp4", codec: "avc1", resolution: "144p", height: 144, fps: 30},
	133: {id: 133, kind: itagVideoOnly, container: "mp4", codec: "avc1", resolution: "240p", height: 240, fps: 30},
	134: {id: 134, kind: itagVideoOnly, container: "mp4", codec: "avc1", resolution: "360p", height: 360, fps: 30},
	135: {id: 135, kind: itagVideoOnly, container: "mp4", codec: "avc1", resolution: "480p", height: 480, fps: 30},
	136: {id: 136, kind: itagVideoOnly, container: "mp4", codec: "avc1", resolution: "720p", height: 720, fps: 30},
	137: {id: 137, kind: itagVideoOnly, container: "mp4", codec: "avc1", resolution: "1080p", height: 1080, fps: 30},
	298: {id: 298, kind: itagVideoOnly, container: "mp4", codec: "avc1", resolution: "720p60", height: 720, fps: 60},
	299: {id: 299, kind: itagVideoOnly, container: "mp4", codec: "avc1", resolution: "1080p60", height: 1080, fps: 60},
	266: {id: 266, kind: itagVideoOnly, container: "mp4", codec: "avc1", resolution: "2160p", height: 2160, fps: 30},
	// 音频(DASH,无视频)
	140: {id: 140, kind: itagAudio, container: "m4a", audioCodec: "mp4a"},
	141: {id: 141, kind: itagAudio, container: "m4a", audioCodec: "mp4a"},
}

func getItag(id int) (itagInfo, bool) {
	info, ok := itags[id]
	return info, ok
}

// isPlayableItag 该 itag 是否在播放栈支持范围内(H.264 视频 / aac 音频 / mp4)。
func isPlayableItag(info itagInfo) bool {
	return info.kind == itagVideo ||
		info.kind == itagVideoOnly ||
		(info.kind == itagAudio && info.audioCodec == "mp4a")
}
